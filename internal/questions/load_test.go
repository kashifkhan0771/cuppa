package questions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadQuestions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "icebreakers.yaml"), `category: icebreakers
name: Icebreakers
prefix: ice
questions:
  - id: ice-0001
    text: "What is your favorite snack?"
    type: open
    sfw: safe
    locale: en
    added: 2026-10-14
`)
	writeFile(t, filepath.Join(dir, retiredFileName), `retired:
  - id: ice-0002
    reason: too similar
`)

	files, retired, err := LoadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || len(files[0].Questions) != 1 {
		t.Fatalf("want 1 file with 1 question, got %+v", files)
	}

	q := files[0].Questions[0]
	// an unquoted YAML date must come through as plain text
	if q.Added != "2026-10-14" {
		t.Errorf("added = %q, want 2026-10-14", q.Added)
	}
	// the category comes from the file header
	if q.Category != "icebreakers" {
		t.Errorf("category = %q, want icebreakers", q.Category)
	}
	if len(retired) != 1 || retired[0].ID != "ice-0002" {
		t.Errorf("retired = %+v, want ice-0002", retired)
	}
}

func TestLoadQuestionsRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "icebreakers.yaml"), `category: icebreakers
questions:
  - id: ice-0001
    tagz: [food]
`)

	_, _, err := LoadDir(dir)
	if err == nil || !strings.Contains(err.Error(), "tagz") {
		t.Fatalf("want error about tagz, got %v", err)
	}
}

func TestLoadQuestionsMissingDir(t *testing.T) {
	files, retired, err := LoadDir(filepath.Join(t.TempDir(), "nope"))
	if err != nil || files != nil || retired != nil {
		t.Fatalf("want nothing and no error, got %v %v %v", files, retired, err)
	}
}

func TestReadWordList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list.txt")
	writeFile(t, path, "# comment\n\nFood\n  work  \n")

	words, err := ReadWordList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(words) != 2 || !words["food"] || !words["work"] {
		t.Fatalf("got %v, want food and work", words)
	}
}
