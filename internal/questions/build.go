package questions

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// Compile turns checked source files into the questions.json layout.
// Categories and questions are sorted by ID so the same input
// always gives the same output.
func Compile(files []File) cuppa.DatasetFile {
	var categories []cuppa.Category
	for _, f := range files {
		categories = append(categories, cuppa.Category{ID: f.Category, Name: f.Name, Count: len(f.Questions)})
	}
	slices.SortFunc(categories, func(a, b cuppa.Category) int { return strings.Compare(a.ID, b.ID) })

	questions := AllQuestions(files)
	slices.SortFunc(questions, func(a, b cuppa.Question) int { return strings.Compare(a.ID, b.ID) })

	return cuppa.DatasetFile{
		SchemaVersion:  cuppa.SchemaVersion,
		DatasetVersion: datasetVersion(questions),
		Categories:     categories,
		Questions:      questions,
	}
}

// datasetVersion is the newest "added" date, written as YYYY.MM.DD.
// Taking it from the data (not the clock) keeps builds repeatable.
func datasetVersion(questions []cuppa.Question) string {
	newest := ""
	for _, q := range questions {
		newest = max(newest, q.Added)
	}
	return strings.ReplaceAll(newest, "-", ".")
}

// WriteDist writes questions.json and one file per category into outDir.
// Old category files are removed first so deleted categories don't linger.
func WriteDist(outDir string, dataset cuppa.DatasetFile) error {
	categoriesDir := filepath.Join(outDir, "categories")
	if err := os.RemoveAll(categoriesDir); err != nil {
		return err
	}
	if err := os.MkdirAll(categoriesDir, 0o750); err != nil {
		return err
	}

	if err := writeJSON(filepath.Join(outDir, "questions.json"), dataset); err != nil {
		return err
	}

	// each category file has the same layout, holding just that category
	for _, category := range dataset.Categories {
		part := cuppa.DatasetFile{
			SchemaVersion:  dataset.SchemaVersion,
			DatasetVersion: dataset.DatasetVersion,
			Categories:     []cuppa.Category{category},
		}
		for _, q := range dataset.Questions {
			if q.Category == category.ID {
				part.Questions = append(part.Questions, q)
			}
		}
		if err := writeJSON(filepath.Join(categoriesDir, category.ID+".json"), part); err != nil {
			return err
		}
	}
	return nil
}

// writeJSON writes v as indented JSON with a trailing newline.
// HTML escaping is off so characters like & stay readable.
func writeJSON(path string, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644) // #nosec G306 -- public data file
}
