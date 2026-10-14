package cuppa

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// ErrNoMatch means the dataset has no questions to pick from,
// usually because the filter left nothing.
var ErrNoMatch = errors.New("cuppa: no questions match the filter")

// Random picks any question, each equally likely.
// A nil rng uses Go's default random source.
func Random(d *Dataset, rng *rand.Rand) (Question, error) {
	if d.Len() == 0 {
		return Question{}, ErrNoMatch
	}
	return cloneQuestion(d.questions[randomIndex(rng, d.Len())]), nil
}

// Daily picks the question of the day. It needs no saved state:
// the same seed and date give the same question on every machine.
//
// For each question it hashes "seed|YYYY-MM-DD|id" with SHA-256 and
// picks the smallest hash. Adding questions only changes the pick if
// a new question wins. The date is taken from day in its own time zone,
// so pass day.In(teamTimeZone) to change days at the team's midnight.
//
// Questions can repeat over time; use Next to avoid repeats.
func Daily(d *Dataset, seed string, day time.Time) (Question, error) {
	if d.Len() == 0 {
		return Question{}, ErrNoMatch
	}

	date := day.Format(time.DateOnly)
	best := 0
	bestHash := dailyHash(seed, date, d.questions[0].ID)
	for i := 1; i < len(d.questions); i++ {
		hash := dailyHash(seed, date, d.questions[i].ID)
		if bytes.Compare(hash[:], bestHash[:]) < 0 {
			best, bestHash = i, hash
		}
	}
	return cloneQuestion(d.questions[best]), nil
}

func dailyHash(seed, date, id string) [sha256.Size]byte {
	return sha256.Sum256([]byte(seed + "|" + date + "|" + id))
}

// Next picks a question not used yet and marks it as used in h.
// When every question has been used, it resets h and starts over.
// New questions from upgrades join the pool on their own.
// A nil rng uses Go's default random source.
func Next(d *Dataset, h History, rng *rand.Rand) (Question, error) {
	if d.Len() == 0 {
		return Question{}, ErrNoMatch
	}

	used, err := h.Used()
	if err != nil {
		return Question{}, fmt.Errorf("cuppa: read history: %w", err)
	}
	var unused []Question
	for _, q := range d.questions {
		if _, ok := used[q.ID]; !ok {
			unused = append(unused, q)
		}
	}

	// everything was used: start a new cycle with the full set
	if len(unused) == 0 {
		if err := h.Reset(); err != nil {
			return Question{}, fmt.Errorf("cuppa: reset history: %w", err)
		}
		unused = d.questions
	}

	q := unused[randomIndex(rng, len(unused))]
	if err := h.MarkUsed(q.ID, time.Now()); err != nil {
		return Question{}, fmt.Errorf("cuppa: save history: %w", err)
	}
	return cloneQuestion(q), nil
}

func randomIndex(rng *rand.Rand, n int) int {
	if rng == nil {
		return rand.IntN(n) // #nosec G404 -- picking a chat question, not a secret
	}
	return rng.IntN(n)
}
