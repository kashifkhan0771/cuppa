package main

import (
	"bytes"
	"os"
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

// writeQuestions writes a category file with the given IDs, and a retired list.
func writeQuestions(t *testing.T, dir string, ids []string, retired string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	b.WriteString("category: icebreakers\nname: Icebreakers\nprefix: ice\nquestions:\n")
	for _, id := range ids {
		b.WriteString("  - id: " + id + "\n    text: What is your favorite snack?\n    type: open\n    sfw: safe\n    locale: en\n    added: 2026-01-01\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "icebreakers.yaml"), []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "_retired.yaml"), []byte(retired), 0o600); err != nil {
		t.Fatal(err)
	}
}

// a removed question must be listed in _retired.yaml, so its ID is never reused
func TestDiffFailsWhenRemovedQuestionIsNotRetired(t *testing.T) {
	root := t.TempDir()
	base, now := filepath.Join(root, "base"), filepath.Join(root, "now")
	writeQuestions(t, base, []string{"ice-0001", "ice-0002"}, "retired: []\n")

	writeQuestions(t, now, []string{"ice-0001"}, "retired: []\n")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"diff", "-base", base, "-questions", now}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "ice-0002") {
		t.Errorf("not retired: exit %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "| Removed | 1 | ice-0002 |") {
		t.Errorf("summary should still be printed:\n%s", stdout.String())
	}

	writeQuestions(t, now, []string{"ice-0001"}, "retired:\n  - id: ice-0002\n    reason: too similar to ice-0001\n")
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"diff", "-base", base, "-questions", now}, &stdout, &stderr); code != 0 {
		t.Errorf("retired: exit %d, stderr %q", code, stderr.String())
	}
}
