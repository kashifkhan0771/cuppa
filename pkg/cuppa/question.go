// Package cuppa loads watercooler questions, filters them, and picks
// one: at random, as the question of the day, or without repeats.
// It makes no network calls and has no third-party dependencies.
package cuppa

// SchemaVersion is the shape version of questions.json.
// It only changes on a breaking change to the file layout.
const SchemaVersion = 1

// Question types.
const (
	TypeOpen       = "open"
	TypePoll       = "poll"
	TypeThisOrThat = "this-or-that"
)

// SFW levels. "safe" fits any workplace, "casual" fits relaxed teams.
const (
	SFWSafe   = "safe"
	SFWCasual = "casual"
)

// Question is one watercooler question.
//
// The yaml tags describe the hand-written files in questions/.
// Category is not written per question in YAML: it comes from the file.
type Question struct {
	ID       string   `json:"id" yaml:"id"`
	Category string   `json:"category" yaml:"-"`
	Text     string   `json:"text" yaml:"text"`
	Type     string   `json:"type" yaml:"type"`
	Options  []string `json:"options,omitempty" yaml:"options"`
	Tags     []string `json:"tags,omitempty" yaml:"tags"`
	SFW      string   `json:"sfw" yaml:"sfw"`
	Locale   string   `json:"locale" yaml:"locale"`
	Added    string   `json:"added" yaml:"added"` // date as YYYY-MM-DD
	Author   string   `json:"author,omitempty" yaml:"author"`
}

// Category is a group of questions, as listed in questions.json.
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DatasetFile is the exact layout of a compiled questions.json file.
type DatasetFile struct {
	SchemaVersion  int        `json:"schema_version"`
	DatasetVersion string     `json:"dataset_version"`
	Categories     []Category `json:"categories"`
	Questions      []Question `json:"questions"`
}
