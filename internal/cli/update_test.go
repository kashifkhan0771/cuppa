package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kashifkhan0771/cuppa/internal/selfupdate"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// fakeReleases serves a GitHub releases list with the given tags, and
// pretends this cuppa is version current.
func fakeReleases(t *testing.T, current string, tags ...string) {
	t.Helper()
	var list []map[string]any
	for _, tag := range tags {
		list = append(list, map[string]any{"tag_name": tag, "html_url": "https://example/" + tag})
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(list)
	}))
	t.Cleanup(srv.Close)

	oldURL, oldVersion := selfupdate.ReleasesURL, Version
	selfupdate.ReleasesURL, Version = srv.URL, current
	t.Cleanup(func() { selfupdate.ReleasesURL, Version = oldURL, oldVersion })
}

// fakeExecutable makes self-update work on a throwaway file instead of
// the test binary.
func fakeExecutable(t *testing.T, path string) {
	t.Helper()
	old := executablePath
	executablePath = func() (string, error) { return path, nil }
	t.Cleanup(func() { executablePath = old })
}

func TestVersionCheck(t *testing.T) {
	isolate(t)

	fakeReleases(t, "0.2.0", "v0.3.0", "v1.0.0")
	r := run(t, "", "version", "--check")
	if r.code != exitOK || !strings.Contains(r.stdout, "newer release: v0.3.0") || !strings.Contains(r.stdout, "new major version v1.0.0") {
		t.Errorf("newer: %+v", r)
	}

	fakeReleases(t, "0.3.0", "v0.3.0")
	if r := run(t, "", "version", "--check"); !strings.Contains(r.stdout, "up to date") {
		t.Errorf("up to date: %+v", r)
	}

	fakeReleases(t, "dev")
	if r := run(t, "", "version", "--check"); r.code != exitConfig || !strings.Contains(r.stderr, "built from source") {
		t.Errorf("dev build: %+v", r)
	}

	// plain version never goes online
	selfupdate.ReleasesURL = "http://127.0.0.1:1/unreachable"
	if r := run(t, "", "version"); r.code != exitOK {
		t.Errorf("version: %+v", r)
	}
}

func TestSelfUpdate(t *testing.T) {
	dir := isolate(t)
	exe := filepath.Join(dir, "cuppa")
	writeFile(t, exe, "old cuppa")
	fakeExecutable(t, exe)

	fakeReleases(t, "0.3.0", "v0.3.0", "v1.0.0")
	if r := run(t, "", "self-update"); r.code != exitOK || !strings.Contains(r.stdout, "up to date") || !strings.Contains(r.stdout, "v1.0.0") {
		t.Errorf("up to date: %+v", r)
	}

	// saying no changes nothing
	fakeReleases(t, "0.2.0", "v0.3.0")
	if r := run(t, "n\n", "self-update"); r.code != exitOK || !strings.Contains(r.stdout, "nothing changed") {
		t.Errorf("declined: %+v", r)
	}
	// saying yes downloads; this fake release has no files, so it fails
	// before touching the binary (selfupdate's own tests cover the rest)
	if r := run(t, "", "self-update", "--yes"); r.code != exitFailed || !strings.Contains(r.stderr, "has no") {
		t.Errorf("--yes: %+v", r)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old cuppa" {
		t.Errorf("binary changed: %q", got)
	}

	fakeReleases(t, "dev")
	if r := run(t, "", "self-update"); r.code != exitConfig || !strings.Contains(r.stderr, "built from source") {
		t.Errorf("dev build: %+v", r)
	}

	fakeReleases(t, "0.2.0", "v0.3.0")
	fakeExecutable(t, "/opt/homebrew/Cellar/cuppa/0.2.0/bin/cuppa")
	if r := run(t, "", "self-update"); r.code != exitConfig || !strings.Contains(r.stderr, "brew upgrade") {
		t.Errorf("homebrew: %+v", r)
	}
}

// fakeCDN serves a questions.json over https and counts downloads.
// The file holds one question in category "fresh".
func fakeCDN(t *testing.T, datasetVersion string) (url string, downloads *atomic.Int32, broken *atomic.Bool) {
	t.Helper()
	downloads, broken = &atomic.Int32{}, &atomic.Bool{}
	file := cuppa.DatasetFile{
		SchemaVersion:  cuppa.SchemaVersion,
		DatasetVersion: datasetVersion,
		Categories:     []cuppa.Category{{ID: "fresh", Name: "Fresh", Count: 1}},
		Questions: []cuppa.Question{{ID: "fre-0001", Category: "fresh", Text: "What is the freshest question?",
			Type: "open", SFW: "safe", Locale: "en", Added: "2099-01-01"}},
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		if broken.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(file)
	}))
	t.Cleanup(srv.Close)

	old := refreshClient
	refreshClient = srv.Client()
	t.Cleanup(func() { refreshClient = old })
	return srv.URL + "/questions.json", downloads, broken
}

func refreshConfig(t *testing.T, url string) {
	t.Helper()
	writeFile(t, "cuppa.yaml", "data:\n  refresh: true\n  refresh_url: "+url+"\n  refresh_interval: 24h\n")
}

func TestDataRefresh(t *testing.T) {
	isolate(t)
	url, downloads, broken := fakeCDN(t, "2099.01.01")
	refreshConfig(t, url)

	if r := run(t, "", "list", "--category", "fresh"); r.code != exitOK || !strings.Contains(r.stdout, "freshest") {
		t.Errorf("first run: %+v", r)
	}

	// within the interval, the cached copy is used
	run(t, "", "list", "--category", "fresh")
	if downloads.Load() != 1 {
		t.Errorf("downloads: %d", downloads.Load())
	}

	// once the cache is old, cuppa downloads again; if that fails,
	// it keeps using the old copy
	cache, err := questionsCachePath()
	if err != nil {
		t.Fatal(err)
	}
	stale := fixedNow.Add(-25 * time.Hour)
	if err := os.Chtimes(cache, stale, stale); err != nil {
		t.Fatal(err)
	}
	broken.Store(true)
	r := run(t, "", "list", "--category", "fresh")
	if downloads.Load() != 2 || !strings.Contains(r.stdout, "freshest") || !strings.Contains(r.stderr, "could not refresh") {
		t.Errorf("offline: %d %+v", downloads.Load(), r)
	}
}

func TestDataRefreshFallsBackToBuiltIn(t *testing.T) {
	isolate(t)

	// an older file than the binary's own questions is ignored
	url, _, _ := fakeCDN(t, "2000.01.01")
	refreshConfig(t, url)
	if r := run(t, "", "list", "--category", "fresh"); r.code != exitOK || strings.Contains(r.stdout, "freshest") {
		t.Errorf("older file used: %+v", r)
	}

	// nothing cached and the server is down: built-in questions, a warning
	isolate(t)
	url, _, broken := fakeCDN(t, "2099.01.01")
	broken.Store(true)
	refreshConfig(t, url)
	r := run(t, "", "list", "--category", "icebreakers")
	if r.code != exitOK || !strings.Contains(r.stdout, "ice-0001") || !strings.Contains(r.stderr, "using the built-in questions") {
		t.Errorf("down with no cache: %+v", r)
	}

	// a file that isn't valid questions is never saved
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"schema_version": 99}`))
	}))
	defer srv.Close()
	refreshClient = srv.Client()
	refreshConfig(t, srv.URL)
	if r := run(t, "", "list"); r.code != exitOK || !strings.Contains(r.stderr, "schema_version 99") {
		t.Errorf("bad file: %+v", r)
	}
	if cache, _ := questionsCachePath(); fileExists(cache) {
		t.Error("bad file was cached")
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
