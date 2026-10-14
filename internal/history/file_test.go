package history

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// File must satisfy the History interface
var _ cuppa.History = (*File)(nil)

func TestMissingFileIsEmpty(t *testing.T) {
	h := NewFile(filepath.Join(t.TempDir(), "history.json"))
	used, err := h.Used()
	if err != nil || len(used) != 0 {
		t.Fatalf("got %v, %v", used, err)
	}
}

func TestSaveAndReload(t *testing.T) {
	// folder doesn't exist yet: it must be created
	path := filepath.Join(t.TempDir(), "state", "cuppa", "history.json")
	at := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)

	if err := NewFile(path).MarkUsed("ice-0001", at); err != nil {
		t.Fatal(err)
	}

	// a fresh File reads what the first one saved
	used, err := NewFile(path).Used()
	if err != nil {
		t.Fatal(err)
	}
	if !used["ice-0001"].Equal(at) || len(used) != 1 {
		t.Fatalf("used = %v", used)
	}
}

func TestResetClearsAndCountsCycles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	h := NewFile(path)
	for range 2 {
		if err := h.MarkUsed("ice-0001", time.Now()); err != nil {
			t.Fatal(err)
		}
		if err := h.Reset(); err != nil {
			t.Fatal(err)
		}
	}

	if used, _ := h.Used(); len(used) != 0 {
		t.Errorf("used after reset = %v", used)
	}
	data := readRaw(t, path)
	if data.Cycles != 2 || data.Version != fileVersion {
		t.Errorf("file = %+v, want 2 cycles", data)
	}
}

func TestBadFiles(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"not json", "nope", "not valid JSON"},
		{"newer version", `{"version": 99, "used": {}}`, "try upgrading cuppa"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "history.json")
			if err := os.WriteFile(path, []byte(tt.content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := NewFile(path).Used()
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestFileWithoutVersionStillLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte(`{"used": {"ice-0001": "2026-10-14T10:00:00Z"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if used, err := NewFile(path).Used(); err != nil || len(used) != 1 {
		t.Fatalf("got %v, %v", used, err)
	}
}

// many writers at once, each with its own File like separate processes
// would have: no write may be lost and no temp files may be left behind
func TestConcurrentWriters(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	shared := NewFile(path)

	var wg sync.WaitGroup
	for i := range 40 {
		wg.Go(func() {
			h := shared
			if i%2 == 0 {
				h = NewFile(path)
			}
			if err := h.MarkUsed(fmt.Sprintf("q-%04d", i), time.Now()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()

	if used, _ := shared.Used(); len(used) != 40 {
		t.Errorf("want 40 used, got %d", len(used))
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func readRaw(t *testing.T, path string) fileData {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var data fileData
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	return data
}
