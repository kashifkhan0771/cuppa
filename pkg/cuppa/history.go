package cuppa

import (
	"maps"
	"sync"
	"time"
)

// History remembers which questions were already used, for Next.
type History interface {
	// Used returns the used question IDs and when each was used.
	Used() (map[string]time.Time, error)
	// MarkUsed records that a question was used.
	MarkUsed(id string, at time.Time) error
	// Reset forgets every used question, starting a new cycle.
	Reset() error
}

// MemoryHistory keeps history in memory, so it is lost when the
// program stops. The zero value is ready to use and it is safe to
// use from many goroutines.
type MemoryHistory struct {
	mu   sync.Mutex
	used map[string]time.Time
}

func (h *MemoryHistory) Used() (map[string]time.Time, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	used := maps.Clone(h.used)
	if used == nil {
		used = map[string]time.Time{}
	}
	return used, nil
}

func (h *MemoryHistory) MarkUsed(id string, at time.Time) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.used == nil {
		h.used = map[string]time.Time{}
	}
	h.used[id] = at
	return nil
}

func (h *MemoryHistory) Reset() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.used = nil
	return nil
}
