package cuppa_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// question makes a safe English open question in the given category.
func question(id, category string, tags ...string) cuppa.Question {
	return cuppa.Question{
		ID: id, Category: category, Text: "Question " + id + "?",
		Type: cuppa.TypeOpen, Tags: tags, SFW: cuppa.SFWSafe, Locale: "en", Added: "2026-10-14",
	}
}

// load builds a Dataset the same way real code does: from JSON.
func load(t *testing.T, version string, questions ...cuppa.Question) *cuppa.Dataset {
	t.Helper()
	raw, err := json.Marshal(cuppa.DatasetFile{
		SchemaVersion:  cuppa.SchemaVersion,
		DatasetVersion: version,
		Categories:     []cuppa.Category{{ID: "icebreakers", Name: "Icebreakers"}},
		Questions:      questions,
	})
	if err != nil {
		t.Fatal(err)
	}
	d, err := cuppa.Load(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func ids(questions []cuppa.Question) string {
	var out []string
	for _, q := range questions {
		out = append(out, q.ID)
	}
	return strings.Join(out, ",")
}

func TestLoad(t *testing.T) {
	d := load(t, "2026.10.14", question("ice-0002", "icebreakers"), question("ice-0001", "icebreakers"), question("tot-0001", "this-or-that"))

	if d.Len() != 3 || d.Version() != "2026.10.14" {
		t.Fatalf("len=%d version=%q", d.Len(), d.Version())
	}
	if got := ids(d.All()); got != "ice-0001,ice-0002,tot-0001" {
		t.Errorf("All() not sorted by id: %s", got)
	}
	if q, ok := d.Get("ice-0002"); !ok || q.ID != "ice-0002" {
		t.Errorf("Get(ice-0002) = %v, %v", q, ok)
	}
	if _, ok := d.Get("nope"); ok {
		t.Error("Get(nope) found something")
	}

	// a category missing from the categories list falls back to its ID as name
	want := []cuppa.Category{{ID: "icebreakers", Name: "Icebreakers", Count: 2}, {ID: "this-or-that", Name: "this-or-that", Count: 1}}
	if got := d.Categories(); !slices.Equal(got, want) {
		t.Errorf("Categories() = %v, want %v", got, want)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name, json, want string
	}{
		{"not json", `nope`, "read questions"},
		{"newer schema", `{"schema_version": 2, "questions": []}`, "try upgrading cuppa"},
		{"missing schema", `{"questions": []}`, "schema_version 0"},
		{"duplicate id", `{"schema_version": 1, "questions": [{"id": "a"}, {"id": "a"}]}`, `"a" appears more than once`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cuppa.Load(strings.NewReader(tt.json))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
}

func TestLoadIgnoresUnknownFields(t *testing.T) {
	raw := `{"schema_version": 1, "future_field": true, "questions": [{"id": "a", "future": 1}]}`
	d, err := cuppa.Load(strings.NewReader(raw))
	if err != nil || d.Len() != 1 {
		t.Fatalf("got %v, %v", d, err)
	}
}

func TestLoadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "questions.json")
	if err := os.WriteFile(path, []byte(`{"schema_version": 1, "questions": [{"id": "a"}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if d, err := cuppa.LoadFile(path); err != nil || d.Len() != 1 {
		t.Fatalf("got %v, %v", d, err)
	}
	if _, err := cuppa.LoadFile(filepath.Join(t.TempDir(), "missing.json")); err == nil {
		t.Fatal("want error for missing file")
	}
}

func TestEmbedded(t *testing.T) {
	d := cuppa.Embedded()
	if d.Len() == 0 || d.Version() == "" || len(d.Categories()) == 0 {
		t.Fatalf("embedded dataset looks empty: len=%d version=%q", d.Len(), d.Version())
	}
	// same Dataset every call, parsed once
	if cuppa.Embedded() != d {
		t.Error("Embedded() returned a different Dataset")
	}
}

func TestReturnedQuestionsAreCopies(t *testing.T) {
	d := load(t, "v", question("ice-0001", "icebreakers", "work"))

	q, _ := d.Get("ice-0001")
	q.Tags[0] = "changed"
	all := d.All()
	all[0].Tags[0] = "changed"

	if q, _ := d.Get("ice-0001"); q.Tags[0] != "work" {
		t.Errorf("Dataset was changed through a returned question: %v", q.Tags)
	}
}

func TestMerge(t *testing.T) {
	base := load(t, "2026.10.14", question("ice-0001", "icebreakers"), question("ice-0002", "icebreakers"))

	replacement := question("ice-0002", "icebreakers")
	replacement.Text = "Our team's own version?"
	pack := load(t, "pack-1", replacement, question("team-0001", "team"))

	merged, overridden := cuppa.Merge(base, pack)

	if got := ids(merged.All()); got != "ice-0001,ice-0002,team-0001" {
		t.Errorf("merged ids = %s", got)
	}
	if q, _ := merged.Get("ice-0002"); q.Text != "Our team's own version?" {
		t.Errorf("later set should win, got %q", q.Text)
	}
	if !slices.Equal(overridden, []string{"ice-0002"}) {
		t.Errorf("overridden = %v", overridden)
	}
	if merged.Version() != "2026.10.14" {
		t.Errorf("version = %q, want first set's", merged.Version())
	}

	empty, overridden := cuppa.Merge()
	if empty.Len() != 0 || overridden != nil {
		t.Errorf("Merge() = %d questions, %v", empty.Len(), overridden)
	}
}
