// Package history saves which questions were used to a JSON file,
// so "no-repeat" mode works across runs.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// fileVersion is the layout version of the history file.
// Bump it on a breaking change and teach read() to upgrade old files.
const fileVersion = 1

// fileData is the exact layout of the history file.
type fileData struct {
	Version int                  `json:"version"`
	Cycles  int                  `json:"cycles"` // how many times every question was used
	Used    map[string]time.Time `json:"used"`
}

// File is a cuppa.History stored in a JSON file.
// It is safe to use from many goroutines and many processes at once.
type File struct {
	path string
	mu   sync.Mutex   // stops goroutines in this process from overlapping
	lock *flock.Flock // stops other processes from overlapping
}

// NewFile returns a history stored at path. The file and its folder
// are created on first write. A "<path>.lock" file sits next to it.
func NewFile(path string) *File {
	return &File{path: path, lock: flock.New(path + ".lock")}
}

func (f *File) Used() (map[string]time.Time, error) {
	var used map[string]time.Time
	err := f.withLock(func() error {
		data, err := f.read()
		used = data.Used
		return err
	})
	return used, err
}

func (f *File) MarkUsed(id string, at time.Time) error {
	return f.withLock(func() error {
		data, err := f.read()
		if err != nil {
			return err
		}
		data.Used[id] = at
		return f.write(data)
	})
}

func (f *File) Reset() error {
	return f.withLock(func() error {
		data, err := f.read()
		if err != nil {
			return err
		}
		data.Used = map[string]time.Time{}
		data.Cycles++
		return f.write(data)
	})
}

// withLock runs fn while holding both the in-process and the file lock.
func (f *File) withLock(fn func() error) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(f.path), 0o750); err != nil {
		return fmt.Errorf("history: %w", err)
	}
	if err := f.lock.Lock(); err != nil {
		return fmt.Errorf("history: lock %s: %w", f.path, err)
	}
	// unlock also closes the lock file; if that fails the OS
	// drops the lock anyway when the process exits
	defer func() { _ = f.lock.Unlock() }()
	return fn()
}

// read loads the file. A missing file means empty history.
func (f *File) read() (fileData, error) {
	data := fileData{Version: fileVersion}
	raw, err := os.ReadFile(f.path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return data, fmt.Errorf("history: %w", err)
	}
	if err == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			return data, fmt.Errorf("history: %s is not valid JSON: %w", f.path, err)
		}
		if data.Version > fileVersion {
			return data, fmt.Errorf("history: %s has version %d, this version of cuppa reads up to %d; try upgrading cuppa",
				f.path, data.Version, fileVersion)
		}
	}

	if data.Used == nil {
		data.Used = map[string]time.Time{}
	}
	data.Version = fileVersion
	return data, nil
}

// write saves data to a temp file, then renames it over the real one,
// so a crash mid-write never leaves a half-written history.
func (f *File) write(data fileData) error {
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(f.path), filepath.Base(f.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("history: %w", err)
	}
	// clean up the temp file if anything below fails
	// (after a successful rename this just finds nothing to remove)
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		_ = tmp.Close() // the write error matters more
		return fmt.Errorf("history: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("history: %w", err)
	}
	if err := os.Rename(tmp.Name(), f.path); err != nil {
		return fmt.Errorf("history: %w", err)
	}
	return nil
}
