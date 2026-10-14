package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

func TestDiffQuestions(t *testing.T) {
	kept := cuppa.Question{ID: "ice-0001", Text: "What is your favorite snack?"}

	edited := cuppa.Question{ID: "ice-0002", Text: "What is your favorite snack?"}
	editedNew := edited
	editedNew.Text = "What is your favorite late-night snack?"

	removed := cuppa.Question{ID: "ice-0003"}
	added := cuppa.Question{ID: "ice-0004"}

	changes := diffQuestions(
		[]cuppa.Question{kept, edited, removed},
		[]cuppa.Question{kept, editedNew, added},
	)

	if strings.Join(changes.added, ",") != "ice-0004" ||
		strings.Join(changes.changed, ",") != "ice-0002" ||
		strings.Join(changes.removed, ",") != "ice-0003" {
		t.Fatalf("got %+v", changes)
	}
}

func TestWriteSummary(t *testing.T) {
	var out bytes.Buffer
	err := writeSummary(&out, questionChanges{added: []string{"ice-0004", "ice-0005"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| Added | 2 | ice-0004, ice-0005 |", "| Changed | 0 |  |", "| Removed | 0 |  |"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("summary missing %q:\n%s", want, out.String())
		}
	}
}

// the whole command, run on the real repo data
func TestRunOnRepoData(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	questions := filepath.Join(root, "questions")
	schema := filepath.Join(root, "schema")

	var stdout, stderr bytes.Buffer
	if code := run([]string{"check", "-questions", questions, "-schema", schema}, &stdout, &stderr); code != 0 {
		t.Fatalf("check exit %d: %s", code, stderr.String())
	}

	// with no base folder, every question shows as added
	stdout.Reset()
	missingBase := filepath.Join(t.TempDir(), "none")
	if code := run([]string{"diff", "-base", missingBase, "-questions", questions}, &stdout, &stderr); code != 0 {
		t.Fatalf("diff exit %d: %s", code, stderr.String())
	}
	summary := stdout.String()
	if strings.Contains(summary, "| Added | 0 |") || !strings.Contains(summary, "| Changed | 0 |") || !strings.Contains(summary, "| Removed | 0 |") {
		t.Errorf("diff output:\n%s", summary)
	}
}

func TestRunUsageErrors(t *testing.T) {
	tests := [][]string{
		{},
		{"nope"},
		{"diff"}, // missing -base
		{"check", "-bad-flag"},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%v) = %d, want 2", args, code)
		}
	}
}
