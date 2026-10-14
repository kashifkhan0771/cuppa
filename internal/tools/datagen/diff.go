package main

import (
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// questionChanges lists question IDs that differ between two versions.
type questionChanges struct {
	added, changed, removed []string
}

// diffQuestions compares old and new questions by ID.
func diffQuestions(oldQuestions, newQuestions []cuppa.Question) questionChanges {
	oldByID := map[string]cuppa.Question{}
	for _, q := range oldQuestions {
		oldByID[q.ID] = q
	}

	var changes questionChanges
	newIDs := map[string]bool{}
	for _, q := range newQuestions {
		newIDs[q.ID] = true
		old, existed := oldByID[q.ID]
		if !existed {
			changes.added = append(changes.added, q.ID)
		} else if !reflect.DeepEqual(old, q) {
			changes.changed = append(changes.changed, q.ID)
		}
	}
	for _, q := range oldQuestions {
		if !newIDs[q.ID] {
			changes.removed = append(changes.removed, q.ID)
		}
	}

	slices.Sort(changes.added)
	slices.Sort(changes.changed)
	slices.Sort(changes.removed)
	return changes
}

// writeSummary prints the changes as Markdown for the CI run page.
func writeSummary(w io.Writer, changes questionChanges) error {
	var b strings.Builder
	b.WriteString("### Question changes\n\n")
	b.WriteString("| Change | Count | IDs |\n|---|---|---|\n")
	rows := []struct {
		label string
		ids   []string
	}{
		{"Added", changes.added},
		{"Changed", changes.changed},
		{"Removed", changes.removed},
	}
	for _, row := range rows {
		fmt.Fprintf(&b, "| %s | %d | %s |\n", row.label, len(row.ids), strings.Join(row.ids, ", "))
	}
	_, err := io.WriteString(w, b.String())
	return err
}
