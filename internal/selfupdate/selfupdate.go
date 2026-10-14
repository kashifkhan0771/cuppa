// Package selfupdate finds newer cuppa releases on GitHub and swaps
// the running binary for a newer one, after checking its checksum.
package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// ReleasesURL lists cuppa's releases. Tests point it at a fake server.
var ReleasesURL = "https://api.github.com/repos/kashifkhan0771/cuppa/releases?per_page=100"

// limits keep a broken or hostile download from filling memory or disk
const (
	maxListSize    = 5 << 20   // the releases list
	maxArchiveSize = 100 << 20 // one release archive
	maxBinarySize  = 200 << 20 // the cuppa binary inside it
)

var httpClient = &http.Client{Timeout: 2 * time.Minute}

// Release is one published cuppa release.
type Release struct {
	Tag     string // like v0.3.0
	URL     string // release page, for the notes
	version version
	assets  map[string]string // file name to download URL
}

// Check is what Latest found.
type Check struct {
	Newer      *Release // newest release with the same major version, if newer than ours
	NewerMajor *Release // newest release with a higher major version, if any
}

// Latest finds the newest releases compared to current (like "0.2.1"
// or "v0.2.1"). Drafts and pre-releases are ignored.
//
// Newer stays within current's major version, because a new major may
// need config changes; NewerMajor only tells the user it exists.
func Latest(ctx context.Context, current string) (Check, error) {
	have, ok := parseVersion(current)
	if !ok {
		return Check{}, fmt.Errorf("version %q is not a release version", current)
	}

	body, err := get(ctx, ReleasesURL, maxListSize)
	if err != nil {
		return Check{}, err
	}
	var list []struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return Check{}, fmt.Errorf("read the releases list: %w", err)
	}

	var check Check
	for _, item := range list {
		v, ok := parseVersion(item.Tag)
		if item.Draft || item.Prerelease || !ok || !have.less(v) {
			continue
		}
		r := &Release{Tag: item.Tag, URL: item.URL, version: v, assets: map[string]string{}}
		for _, a := range item.Assets {
			r.assets[a.Name] = a.URL
		}

		// keep the newest of each kind; the list order is not trusted
		best := &check.NewerMajor
		if v.major == have.major {
			best = &check.Newer
		}
		if *best == nil || (*best).version.less(v) {
			*best = r
		}
	}
	return check, nil
}

// Apply downloads the release for this computer, checks it against
// checksums.txt, and puts it in place of the binary at exePath.
func Apply(ctx context.Context, r *Release, exePath string) error {
	archiveName := ArchiveName(r.version.String(), runtime.GOOS, runtime.GOARCH)
	archiveURL, ok := r.assets[archiveName]
	if !ok {
		return fmt.Errorf("release %s has no %s", r.Tag, archiveName)
	}
	sumsURL, ok := r.assets["checksums.txt"]
	if !ok {
		return fmt.Errorf("release %s has no checksums.txt", r.Tag)
	}

	// download both, and refuse an archive that doesn't match
	sums, err := get(ctx, sumsURL, maxListSize)
	if err != nil {
		return err
	}
	archive, err := get(ctx, archiveURL, maxArchiveSize)
	if err != nil {
		return err
	}
	want, ok := findChecksum(sums, archiveName)
	if !ok {
		return fmt.Errorf("checksums.txt has no line for %s", archiveName)
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != want {
		return fmt.Errorf("%s does not match its checksum; nothing was changed", archiveName)
	}

	binary, err := extractBinary(archive, strings.HasSuffix(archiveName, ".zip"))
	if err != nil {
		return fmt.Errorf("%s: %w", archiveName, err)
	}
	return replace(exePath, binary)
}

// IsRelease reports whether a version string is a release version,
// like v1.2.3, and not "dev" or a pre-release.
func IsRelease(version string) bool {
	_, ok := parseVersion(version)
	return ok
}

// ArchiveName is the release file GoReleaser makes for one platform.
func ArchiveName(version, goos, goarch string) string {
	name := fmt.Sprintf("cuppa_%s_%s_%s", strings.TrimPrefix(version, "v"), goos, goarch)
	if goos == "windows" {
		return name + ".zip"
	}
	return name + ".tar.gz"
}

// findChecksum reads a "<sha256>  <file name>" line from checksums.txt.
func findChecksum(sums []byte, name string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(sums))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && fields[1] == name {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}

// extractBinary returns the cuppa (or cuppa.exe) file from an archive.
func extractBinary(archive []byte, isZip bool) ([]byte, error) {
	if isZip {
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, err
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) == "cuppa.exe" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				defer rc.Close()
				return readLimited(rc, maxBinarySize)
			}
		}
		return nil, errors.New("no cuppa.exe inside")
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("no cuppa binary inside")
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg && filepath.Base(h.Name) == "cuppa" {
			return readLimited(tr, maxBinarySize)
		}
	}
}

// replace writes the new binary next to the old one, then swaps it in
// with a rename, so the old binary is never half overwritten.
func replace(exePath string, binary []byte) error {
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".cuppa-update-*")
	if err != nil {
		return fmt.Errorf("can't write to %s (try again with permission, or reinstall): %w", dir, err)
	}
	defer os.Remove(tmp.Name()) // only left over if something failed

	if _, err := tmp.Write(binary); err != nil {
		_ = tmp.Close() // the write error matters more
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil { // #nosec G302 -- a program must be executable
		return err
	}

	// Windows can't replace a running .exe, but it can rename it;
	// RemoveOld deletes it on the next run
	if runtime.GOOS == "windows" {
		old := exePath + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			return err
		}
		if err := os.Rename(tmp.Name(), exePath); err != nil {
			_ = os.Rename(old, exePath) // put the old one back
			return err
		}
		return nil
	}
	return os.Rename(tmp.Name(), exePath)
}

// RemoveOld deletes the binary a Windows update left behind.
func RemoveOld(exePath string) {
	_ = os.Remove(exePath + ".old")
}

// get downloads a URL, refusing answers bigger than limit.
func get(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cuppa")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return readLimited(resp.Body, limit)
}

func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("download is bigger than %d MB", limit>>20)
	}
	return data, nil
}

// version is a release version like v1.2.3.
type version struct{ major, minor, patch int }

// parseVersion reads "1.2.3" or "v1.2.3". Anything else, including
// pre-releases like 1.2.3-rc.1 and "dev", is not a release version.
func parseVersion(text string) (version, bool) {
	parts := strings.Split(strings.TrimPrefix(text, "v"), ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var numbers [3]int
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version{}, false
		}
		numbers[i] = n
	}
	return version{numbers[0], numbers[1], numbers[2]}, true
}

func (v version) less(other version) bool {
	if v.major != other.major {
		return v.major < other.major
	}
	if v.minor != other.minor {
		return v.minor < other.minor
	}
	return v.patch < other.patch
}

func (v version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch)
}
