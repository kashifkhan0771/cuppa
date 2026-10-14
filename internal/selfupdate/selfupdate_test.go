package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeGitHub serves a releases list and the files it points to.
// files maps a file name to its contents.
type fakeGitHub struct {
	srv      *httptest.Server
	releases []map[string]any
	files    map[string][]byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{files: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/releases" {
			_ = json.NewEncoder(w).Encode(f.releases)
			return
		}
		data, ok := f.files[strings.TrimPrefix(r.URL.Path, "/download/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(f.srv.Close)

	old := ReleasesURL
	ReleasesURL = f.srv.URL + "/releases"
	t.Cleanup(func() { ReleasesURL = old })
	return f
}

// release adds a release whose assets are the given file names.
func (f *fakeGitHub) release(tag string, draft, prerelease bool, assets ...string) {
	var list []map[string]string
	for _, name := range assets {
		list = append(list, map[string]string{"name": name, "browser_download_url": f.srv.URL + "/download/" + name})
	}
	f.releases = append(f.releases, map[string]any{
		"tag_name": tag, "html_url": "https://example/" + tag,
		"draft": draft, "prerelease": prerelease, "assets": list,
	})
}

// archive builds a release archive holding a fake cuppa binary.
func archive(t *testing.T, windows bool, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if windows {
		zw := zip.NewWriter(&buf)
		for _, name := range []string{"README.md", "cuppa.exe"} {
			w, err := zw.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = w.Write(binary)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, name := range []string{"README.md", "cuppa"} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(binary)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		_, _ = tw.Write(binary)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// publish adds a release v<version> with a real archive for this
// computer and a checksums.txt listing it.
func (f *fakeGitHub) publish(t *testing.T, version string, binary []byte) {
	t.Helper()
	name := ArchiveName(version, runtime.GOOS, runtime.GOARCH)
	data := archive(t, runtime.GOOS == "windows", binary)
	sum := sha256.Sum256(data)
	f.files[name] = data
	f.files["checksums.txt"] = fmt.Appendf(nil, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	f.release("v"+version, false, false, name, "checksums.txt")
}

func TestLatest(t *testing.T) {
	f := newFakeGitHub(t)
	f.release("v0.2.0", false, false)
	f.release("v0.4.0", true, false)      // draft
	f.release("v0.5.0-rc.1", false, true) // pre-release
	f.release("v0.3.1", false, false)     // newest 0.x
	f.release("v0.3.0", false, false)
	f.release("v1.0.0", false, false)
	f.release("v1.2.0", false, false) // newest major, listed out of order
	f.release("not-a-version", false, false)

	check, err := Latest(context.Background(), "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if check.Newer == nil || check.Newer.Tag != "v0.3.1" || check.NewerMajor == nil || check.NewerMajor.Tag != "v1.2.0" {
		t.Errorf("got %+v %+v", check.Newer, check.NewerMajor)
	}

	// already the newest of its major
	check, _ = Latest(context.Background(), "v1.2.0")
	if check.Newer != nil || check.NewerMajor != nil {
		t.Errorf("up to date: got %+v", check)
	}

	if _, err := Latest(context.Background(), "dev"); err == nil {
		t.Error("want an error for a dev build")
	}
}

func TestLatestErrors(t *testing.T) {
	f := newFakeGitHub(t)
	f.files["x"] = nil
	ReleasesURL = f.srv.URL + "/missing"
	if _, err := Latest(context.Background(), "0.1.0"); err == nil {
		t.Error("want an error for a 404")
	}
	ReleasesURL = f.srv.URL + "/download/x"
	if _, err := Latest(context.Background(), "0.1.0"); err == nil {
		t.Error("want an error for a body that isn't JSON")
	}
}

func TestApply(t *testing.T) {
	f := newFakeGitHub(t)
	f.publish(t, "0.3.0", []byte("new cuppa"))
	check, err := Latest(context.Background(), "0.2.0")
	if err != nil || check.Newer == nil {
		t.Fatal(check, err)
	}

	exe := filepath.Join(t.TempDir(), "cuppa")
	if err := os.WriteFile(exe, []byte("old cuppa"), 0o755); err != nil { // #nosec G306
		t.Fatal(err)
	}
	if err := Apply(context.Background(), check.Newer, exe); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "new cuppa" {
		t.Errorf("binary is %q", got)
	}
	if info, _ := os.Stat(exe); runtime.GOOS != "windows" && info.Mode().Perm()&0o100 == 0 {
		t.Errorf("not executable: %v", info.Mode())
	}
	// no temp files left next to it
	if entries, _ := os.ReadDir(filepath.Dir(exe)); len(entries) > 2 {
		t.Errorf("left over files: %v", entries)
	}
	RemoveOld(exe)
	if _, err := os.Stat(exe + ".old"); err == nil {
		t.Error(".old still there")
	}
}

func TestApplyRefusesBadDownloads(t *testing.T) {
	f := newFakeGitHub(t)
	f.publish(t, "0.3.0", []byte("new cuppa"))
	name := ArchiveName("0.3.0", runtime.GOOS, runtime.GOARCH)
	check, _ := Latest(context.Background(), "0.2.0")
	exe := filepath.Join(t.TempDir(), "cuppa")
	_ = os.WriteFile(exe, []byte("old cuppa"), 0o600)

	tests := map[string]func(){
		"wrong checksum":    func() { f.files[name] = append(f.files[name], 'x') },
		"no checksum line":  func() { f.files["checksums.txt"] = []byte("abc  other.tar.gz\n") },
		"checksums missing": func() { delete(f.files, "checksums.txt") },
		"archive missing":   func() { delete(f.files, name) },
	}
	for what, breakIt := range tests {
		f.publish(t, "0.3.0", []byte("new cuppa")) // fresh good copy
		breakIt()
		if err := Apply(context.Background(), check.Newer, exe); err == nil {
			t.Errorf("%s: want an error", what)
		}
		if got, _ := os.ReadFile(exe); string(got) != "old cuppa" {
			t.Errorf("%s: binary changed to %q", what, got)
		}
	}

	// a release with nothing for this computer
	f.release("v0.4.0", false, false, "checksums.txt")
	check, _ = Latest(context.Background(), "0.2.0")
	if err := Apply(context.Background(), check.Newer, exe); err == nil || !strings.Contains(err.Error(), "has no") {
		t.Errorf("no archive: %v", err)
	}
}

func TestExtractBinary(t *testing.T) {
	for _, windows := range []bool{false, true} {
		got, err := extractBinary(archive(t, windows, []byte("bin")), windows)
		if err != nil || string(got) != "bin" {
			t.Errorf("windows=%v: %q %v", windows, got, err)
		}
	}
	if _, err := extractBinary([]byte("junk"), false); err == nil {
		t.Error("want an error for junk tar.gz")
	}
	if _, err := extractBinary([]byte("junk"), true); err == nil {
		t.Error("want an error for junk zip")
	}

	// archives without the binary
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_ = tar.NewWriter(gz).Close()
	_ = gz.Close()
	if _, err := extractBinary(buf.Bytes(), false); err == nil {
		t.Error("want an error for an empty tar.gz")
	}
	buf.Reset()
	_ = zip.NewWriter(&buf).Close()
	if _, err := extractBinary(buf.Bytes(), true); err == nil {
		t.Error("want an error for an empty zip")
	}
}

func TestReplaceNeedsWritableFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("folder permissions work differently on Windows")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "cuppa")
	_ = os.WriteFile(exe, []byte("old"), 0o600)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // #nosec G302
	if err := replace(exe, []byte("new")); err == nil || !strings.Contains(err.Error(), "permission") {
		t.Errorf("got %v", err)
	}
}

func TestVersions(t *testing.T) {
	for _, bad := range []string{"dev", "1.2", "1.2.x", "1.2.3-rc.1", "v-1.0.0", ""} {
		if _, ok := parseVersion(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
	a, _ := parseVersion("v1.2.3")
	for _, newer := range []string{"1.2.4", "1.3.0", "2.0.0"} {
		b, _ := parseVersion(newer)
		if !a.less(b) || b.less(a) {
			t.Errorf("%s should be newer than %s", newer, a)
		}
	}
	if got := ArchiveName("v1.2.3", "windows", "arm64"); got != "cuppa_1.2.3_windows_arm64.zip" {
		t.Errorf("archive name %s", got)
	}
}

func TestDownloadLimit(t *testing.T) {
	if _, err := readLimited(strings.NewReader("12345"), 4); err == nil {
		t.Error("want an error past the limit")
	}
}
