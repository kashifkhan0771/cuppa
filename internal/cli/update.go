package cli

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kashifkhan0771/cuppa/internal/selfupdate"
	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// runVersion prints the build and dataset versions. With --check it
// also asks GitHub whether a newer release exists. cuppa never checks
// on its own.
func runVersion(a *app, args []string) error {
	fs := a.newFlagSet("version", "version [--check]")
	check := fs.Bool("check", false, "check GitHub for a newer release")
	if err := a.parse(fs, args); err != nil {
		return err
	}

	fmt.Fprintf(a.stdout, "cuppa %s", Version)
	if Commit != "" {
		fmt.Fprintf(a.stdout, " (commit %s, built %s)", Commit, Date)
	}
	d := cuppa.Embedded()
	fmt.Fprintf(a.stdout, "\nquestions %s (%d built in)\n", d.Version(), d.Len())
	if !*check {
		return nil
	}

	if !selfupdate.IsRelease(Version) {
		return configError(errDevBuild)
	}
	found, err := selfupdate.Latest(a.ctx, Version)
	if err != nil {
		return err
	}
	if found.Newer == nil {
		fmt.Fprintln(a.stdout, "up to date")
	} else {
		fmt.Fprintf(a.stdout, "newer release: %s (%s)\nupgrade with \"cuppa self-update\", your package manager, or from the releases page\n", found.Newer.Tag, found.Newer.URL)
	}
	printNewerMajor(a, found)
	return nil
}

// errDevBuild is for builds without a release version to compare.
var errDevBuild = errors.New(`this cuppa was built from source, so it has no release version to compare; update it the way you installed it, like "go install github.com/kashifkhan0771/cuppa/cmd/cuppa@latest"`)

// printNewerMajor mentions a new major version, which self-update
// won't install because it may need config changes.
func printNewerMajor(a *app, found selfupdate.Check) {
	if found.NewerMajor != nil {
		fmt.Fprintf(a.stdout, "a new major version %s is out; it may need config changes, so read its notes first: %s\n", found.NewerMajor.Tag, found.NewerMajor.URL)
	}
}

// executablePath is the running binary. Tests point it at a copy, so
// they never replace the test binary itself.
var executablePath = os.Executable

// runSelfUpdate replaces a downloaded cuppa binary with the newest
// release of the same major version, after checking its checksum.
func runSelfUpdate(a *app, args []string) error {
	fs := a.newFlagSet("self-update", "self-update [--yes]")
	yes := fs.Bool("yes", false, "update without asking")
	if err := a.parse(fs, args); err != nil {
		return err
	}

	exe, err := executablePath()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	selfupdate.RemoveOld(exe) // left by an earlier update on Windows

	// other installs update their own way
	if !selfupdate.IsRelease(Version) {
		return configError(errDevBuild)
	}
	if strings.Contains(filepath.ToSlash(exe), "/Cellar/") {
		return configError(errors.New(`cuppa was installed with Homebrew; update it with "brew upgrade cuppa"`))
	}

	found, err := selfupdate.Latest(a.ctx, Version)
	if err != nil {
		return err
	}
	if found.Newer == nil {
		fmt.Fprintf(a.stdout, "cuppa %s is up to date\n", Version)
		printNewerMajor(a, found)
		return nil
	}

	if !*yes {
		fmt.Fprintf(a.stdout, "update %s from %s to %s? [y/N]: ", exe, Version, found.Newer.Tag)
		in := bufio.NewScanner(a.stdin)
		in.Scan()
		if answer := strings.ToLower(strings.TrimSpace(in.Text())); answer != "y" && answer != "yes" {
			fmt.Fprintln(a.stdout, "nothing changed")
			return nil
		}
	}
	if err := selfupdate.Apply(a.ctx, found.Newer, exe); err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "updated to %s (%s)\n", found.Newer.Tag, found.Newer.URL)
	printNewerMajor(a, found)
	return nil
}

// data.refresh

// refreshClient downloads questions for data.refresh. Tests swap it
// for one that trusts their test server.
var refreshClient = &http.Client{Timeout: 15 * time.Second}

const maxQuestionsSize = 20 << 20

// builtInQuestions is the newest question set cuppa has: the one inside
// the binary or, with data.refresh on, a fresher copy from the CDN.
// Refresh problems are only warnings; the built-in set always works.
func (a *app) builtInQuestions() *cuppa.Dataset {
	embedded := cuppa.Embedded()
	if !a.cfg.Data.Refresh {
		return embedded
	}
	refreshed, err := a.refreshedQuestions()
	if err != nil {
		a.log.Warn("using the built-in questions", "reason", err)
		return embedded
	}
	// dataset versions are dates like 2026.10.14, so text order works;
	// never go back to questions older than the binary's own
	if refreshed.Version() < embedded.Version() {
		a.log.Debug("refreshed questions are older than the built-in ones", "refreshed", refreshed.Version())
		return embedded
	}
	return refreshed
}

// refreshedQuestions returns the cached CDN copy, downloading a new
// one first when the cache is older than data.refresh_interval. If the
// download fails, the older cached copy is still used.
func (a *app) refreshedQuestions() (*cuppa.Dataset, error) {
	path, err := questionsCachePath()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil || a.now().Sub(info.ModTime()) >= a.cfg.Data.RefreshInterval {
		if err := a.downloadQuestions(path); err != nil {
			a.log.Warn("could not refresh questions", "error", err)
		}
	}
	return cuppa.LoadFile(path)
}

// downloadQuestions fetches data.refresh_url and saves it to path,
// only if it is a valid questions file.
func (a *app) downloadQuestions(path string) error {
	req, err := http.NewRequestWithContext(a.ctx, http.MethodGet, a.cfg.Data.RefreshURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "cuppa")
	resp, err := refreshClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", a.cfg.Data.RefreshURL, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxQuestionsSize+1))
	if err != nil {
		return err
	}
	if len(body) > maxQuestionsSize {
		return fmt.Errorf("%s is bigger than %d MB", a.cfg.Data.RefreshURL, maxQuestionsSize>>20)
	}
	if _, err := cuppa.Load(bytes.NewReader(body)); err != nil {
		return err
	}

	// write a temp file and rename it, so a reader never sees half a file
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".questions-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // only left over if something failed
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close() // the write error matters more
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	a.log.Info("refreshed questions", "url", a.cfg.Data.RefreshURL)
	return nil
}

// questionsCachePath is where data.refresh keeps its copy, in the
// user cache folder (~/.cache, ~/Library/Caches or %LocalAppData%).
func questionsCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "cuppa", "questions.json"), nil
}
