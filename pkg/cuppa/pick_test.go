package cuppa_test

import (
	"encoding/json"
	"errors"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

func threeQuestions(t *testing.T) *cuppa.Dataset {
	return load(t, "v", question("ice-0001", "icebreakers"), question("ice-0002", "icebreakers"), question("ice-0003", "icebreakers"))
}

func TestPickersOnEmptyDataset(t *testing.T) {
	empty := load(t, "v")
	if _, err := cuppa.Random(empty, nil); !errors.Is(err, cuppa.ErrNoMatch) {
		t.Errorf("Random: %v", err)
	}
	if _, err := cuppa.Daily(empty, "seed", time.Now()); !errors.Is(err, cuppa.ErrNoMatch) {
		t.Errorf("Daily: %v", err)
	}
	if _, err := cuppa.Next(empty, &cuppa.MemoryHistory{}, nil); !errors.Is(err, cuppa.ErrNoMatch) {
		t.Errorf("Next: %v", err)
	}
}

func TestRandom(t *testing.T) {
	d := threeQuestions(t)

	// a seeded source always gives the same pick
	first, _ := cuppa.Random(d, rand.New(rand.NewPCG(1, 2)))
	second, _ := cuppa.Random(d, rand.New(rand.NewPCG(1, 2)))
	if first.ID != second.ID {
		t.Errorf("same seed gave %s and %s", first.ID, second.ID)
	}

	// over many picks, every question shows up
	seen := map[string]bool{}
	for range 100 {
		q, err := cuppa.Random(d, nil)
		if err != nil {
			t.Fatal(err)
		}
		seen[q.ID] = true
	}
	if len(seen) != 3 {
		t.Errorf("saw only %v", seen)
	}
}

// pins the daily rule: the Apps Script example ports it and must agree
func TestDailyVectors(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "daily_vectors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		IDs   []string `json:"ids"`
		Cases []struct {
			Seed     string `json:"seed"`
			Date     string `json:"date"`
			Expected string `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatal(err)
	}

	var questions []cuppa.Question
	for _, id := range vectors.IDs {
		questions = append(questions, question(id, "icebreakers"))
	}
	d := load(t, "v", questions...)

	for _, c := range vectors.Cases {
		day, err := time.Parse(time.DateOnly, c.Date)
		if err != nil {
			t.Fatal(err)
		}
		q, err := cuppa.Daily(d, c.Seed, day)
		if err != nil {
			t.Fatal(err)
		}
		if q.ID != c.Expected {
			t.Errorf("seed %q date %s: got %s, want %s", c.Seed, c.Date, q.ID, c.Expected)
		}
	}
}

func TestDailyUsesDateInGivenTimeZone(t *testing.T) {
	d := cuppa.Embedded()
	karachi := time.FixedZone("PKT", 5*60*60)

	// 21:00 UTC on Oct 14 is already 02:00 on Oct 15 in Karachi
	instant := time.Date(2026, 10, 14, 21, 0, 0, 0, time.UTC)
	oct14 := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	oct15 := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)

	pick := func(day time.Time) string {
		q, err := cuppa.Daily(d, "team-alpha", day)
		if err != nil {
			t.Fatal(err)
		}
		return q.ID
	}

	if pick(instant) != pick(oct14) {
		t.Error("in UTC the pick should be Oct 14's")
	}
	if pick(instant.In(karachi)) != pick(oct15) {
		t.Error("in Karachi the pick should be Oct 15's")
	}
	if pick(oct14) == pick(oct15) {
		t.Skip("both days happen to pick the same question; test can't tell them apart")
	}
}

func TestNextUsesEveryQuestionOncePerCycle(t *testing.T) {
	d := threeQuestions(t)
	history := &cuppa.MemoryHistory{}
	rng := rand.New(rand.NewPCG(7, 7))

	// first cycle: three different questions
	seen := map[string]bool{}
	for range 3 {
		q, err := cuppa.Next(d, history, rng)
		if err != nil {
			t.Fatal(err)
		}
		if seen[q.ID] {
			t.Fatalf("%s repeated within one cycle", q.ID)
		}
		seen[q.ID] = true
	}

	// fourth pick: all used, so history resets and a new cycle starts
	if _, err := cuppa.Next(d, history, rng); err != nil {
		t.Fatal(err)
	}
	used, _ := history.Used()
	if len(used) != 1 {
		t.Errorf("after reset, want 1 used question, got %d", len(used))
	}
}

func TestNextIgnoresUsedQuestionsOutsideTheSet(t *testing.T) {
	d := threeQuestions(t)
	history := &cuppa.MemoryHistory{}
	// a used ID from some other filter, plus two of ours
	for _, id := range []string{"tot-0001", "ice-0001", "ice-0002"} {
		if err := history.MarkUsed(id, time.Now()); err != nil {
			t.Fatal(err)
		}
	}

	q, err := cuppa.Next(d, history, nil)
	if err != nil || q.ID != "ice-0003" {
		t.Fatalf("want ice-0003, got %v %v", q.ID, err)
	}
}

// failingHistory returns an error from every method.
type failingHistory struct{}

var errBroken = errors.New("broken")

func (failingHistory) Used() (map[string]time.Time, error) { return nil, errBroken }
func (failingHistory) MarkUsed(string, time.Time) error    { return errBroken }
func (failingHistory) Reset() error                        { return errBroken }

func TestNextReportsHistoryErrors(t *testing.T) {
	if _, err := cuppa.Next(threeQuestions(t), failingHistory{}, nil); !errors.Is(err, errBroken) {
		t.Fatalf("want history error, got %v", err)
	}
}
