package questions

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// testFiles gives two categories, listed out of order on purpose.
func testFiles() []File {
	oldest := goodQuestion()
	oldest.Category = "icebreakers"

	newest := goodQuestion()
	newest.ID = "ice-0002"
	newest.Category = "icebreakers"
	newest.Text = "Tea & biscuits or coffee & cake?"
	newest.Added = "2026-11-02"

	other := goodQuestion()
	other.ID = "tot-0001"
	other.Category = "this-or-that"

	return []File{
		{Category: "this-or-that", Name: "This or That", Prefix: "tot", Questions: []cuppa.Question{other}},
		goodFile(newest, oldest),
	}
}

func TestCompile(t *testing.T) {
	dataset := Compile(testFiles())

	if dataset.SchemaVersion != cuppa.SchemaVersion {
		t.Errorf("schema_version = %d", dataset.SchemaVersion)
	}
	if dataset.DatasetVersion != "2026.11.02" {
		t.Errorf("dataset_version = %q, want newest added date", dataset.DatasetVersion)
	}

	// categories and questions come out sorted by ID
	if dataset.Categories[0].ID != "icebreakers" || dataset.Categories[0].Count != 2 {
		t.Errorf("categories = %+v", dataset.Categories)
	}
	var ids []string
	for _, q := range dataset.Questions {
		ids = append(ids, q.ID)
	}
	if strings.Join(ids, ",") != "ice-0001,ice-0002,tot-0001" {
		t.Errorf("question order = %v", ids)
	}
}

func TestWriteDist(t *testing.T) {
	files := testFiles()
	out := t.TempDir()

	// a file from a deleted category must be cleaned up
	stale := filepath.Join(out, "categories", "old.json")
	if err := os.MkdirAll(filepath.Dir(stale), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile(t, stale, "{}")

	if err := WriteDist(out, Compile(files)); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(filepath.Join(out, "questions.json"))
	if err != nil {
		t.Fatal(err)
	}

	// same input twice must give the exact same bytes
	if err := WriteDist(out, Compile(files)); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(out, "questions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("build output changed between runs")
	}
	if !bytes.Contains(first, []byte("Tea & biscuits")) {
		t.Error("& should not be escaped")
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Error("stale category file was not removed")
	}

	// each category file holds only its own questions
	data, err := os.ReadFile(filepath.Join(out, "categories", "this-or-that.json"))
	if err != nil {
		t.Fatal(err)
	}
	var part cuppa.DatasetFile
	if err := json.Unmarshal(data, &part); err != nil {
		t.Fatal(err)
	}
	if len(part.Questions) != 1 || part.Questions[0].ID != "tot-0001" || len(part.Categories) != 1 {
		t.Errorf("this-or-that.json = %+v", part)
	}
}
