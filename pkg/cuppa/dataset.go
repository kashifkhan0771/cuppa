package cuppa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/kashifkhan0771/cuppa/dist"
)

// Dataset is a set of questions. It never changes after it is built,
// so one Dataset can be shared between goroutines.
type Dataset struct {
	version       string
	questions     []Question        // sorted by ID
	byID          map[string]int    // question ID -> index in questions
	categoryNames map[string]string // category ID -> display name
}

// newDataset sorts the questions by ID and builds the lookup index.
// Callers make sure IDs are unique.
func newDataset(version string, questions []Question, categoryNames map[string]string) *Dataset {
	questions = slices.Clone(questions)
	slices.SortFunc(questions, func(a, b Question) int { return strings.Compare(a.ID, b.ID) })

	byID := make(map[string]int, len(questions))
	for i, q := range questions {
		byID[q.ID] = i
	}
	return &Dataset{version: version, questions: questions, byID: byID, categoryNames: categoryNames}
}

// Load reads a questions.json file.
// Unknown fields are ignored, so newer files with extra fields still load.
func Load(r io.Reader) (*Dataset, error) {
	var file DatasetFile
	if err := json.NewDecoder(r).Decode(&file); err != nil {
		return nil, fmt.Errorf("cuppa: read questions: %w", err)
	}
	return New(file)
}

// New builds a Dataset from questions already in the questions.json layout.
func New(file DatasetFile) (*Dataset, error) {
	if file.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("cuppa: questions file has schema_version %d, this version of cuppa reads %d; try upgrading cuppa",
			file.SchemaVersion, SchemaVersion)
	}

	seen := make(map[string]bool, len(file.Questions))
	for _, q := range file.Questions {
		if seen[q.ID] {
			return nil, fmt.Errorf("cuppa: question id %q appears more than once", q.ID)
		}
		seen[q.ID] = true
	}

	names := make(map[string]string, len(file.Categories))
	for _, c := range file.Categories {
		names[c.ID] = c.Name
	}
	return newDataset(file.DatasetVersion, file.Questions, names), nil
}

// LoadFile reads a questions.json file from disk.
func LoadFile(path string) (*Dataset, error) {
	f, err := os.Open(path) // #nosec G304 -- reading a file the caller chose is the point
	if err != nil {
		return nil, fmt.Errorf("cuppa: %w", err)
	}
	defer f.Close()
	return Load(f)
}

// embedded parses the built-in questions once, on first use.
var embedded = sync.OnceValue(func() *Dataset {
	d, err := Load(bytes.NewReader(dist.QuestionsJSON))
	if err != nil {
		// tests load this data, so this only happens with a broken build
		panic("cuppa: built-in questions are broken: " + err.Error())
	}
	return d
})

// Embedded returns the questions built into this version of cuppa.
func Embedded() *Dataset {
	return embedded()
}

// Merge combines datasets into one, for example the built-in questions
// plus a team's private pack. When two sets have the same question ID,
// the later set wins, and that ID is listed in overridden so the caller
// can warn about it. The result keeps the version of the first set.
func Merge(sets ...*Dataset) (merged *Dataset, overridden []string) {
	version := ""
	if len(sets) > 0 {
		version = sets[0].version
	}

	byID := map[string]Question{}
	names := map[string]string{}
	for _, set := range sets {
		for _, q := range set.questions {
			if _, exists := byID[q.ID]; exists {
				overridden = append(overridden, q.ID)
			}
			byID[q.ID] = q
		}
		for id, name := range set.categoryNames {
			names[id] = name
		}
	}

	questions := make([]Question, 0, len(byID))
	for _, q := range byID {
		questions = append(questions, q)
	}
	return newDataset(version, questions, names), overridden
}

// Get returns the question with the given ID.
func (d *Dataset) Get(id string) (Question, bool) {
	i, ok := d.byID[id]
	if !ok {
		return Question{}, false
	}
	return cloneQuestion(d.questions[i]), true
}

// All returns every question, sorted by ID.
func (d *Dataset) All() []Question {
	all := make([]Question, len(d.questions))
	for i, q := range d.questions {
		all[i] = cloneQuestion(q)
	}
	return all
}

// Len returns the number of questions.
func (d *Dataset) Len() int {
	return len(d.questions)
}

// Version returns the dataset version, like "2026.10.14".
func (d *Dataset) Version() string {
	return d.version
}

// Categories lists the categories that have at least one question,
// with counts for this dataset (so they reflect any filter), sorted by ID.
func (d *Dataset) Categories() []Category {
	counts := map[string]int{}
	for _, q := range d.questions {
		counts[q.Category]++
	}

	categories := make([]Category, 0, len(counts))
	for id, count := range counts {
		name := d.categoryNames[id]
		if name == "" {
			name = id
		}
		categories = append(categories, Category{ID: id, Name: name, Count: count})
	}
	slices.SortFunc(categories, func(a, b Category) int { return strings.Compare(a.ID, b.ID) })
	return categories
}

// cloneQuestion copies the slices inside a question, so a caller
// changing the result can't change the Dataset.
func cloneQuestion(q Question) Question {
	q.Options = slices.Clone(q.Options)
	q.Tags = slices.Clone(q.Tags)
	return q
}
