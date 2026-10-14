package cuppa_test

import (
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

func TestFilter(t *testing.T) {
	casual := question("ice-0003", "icebreakers", "fun")
	casual.SFW = cuppa.SFWCasual

	poll := question("tot-0001", "this-or-that", "food")
	poll.Type = cuppa.TypePoll

	spanish := question("ice-0004", "icebreakers")
	spanish.Locale = "es"

	d := load(t, "v",
		question("ice-0001", "icebreakers", "work"),
		question("ice-0002", "icebreakers", "food", "fun"),
		casual, poll, spanish,
	)

	tests := []struct {
		name   string
		filter cuppa.Filter
		want   string
	}{
		{"empty filter keeps safe only", cuppa.Filter{}, "ice-0001,ice-0002,ice-0004,tot-0001"},
		{"casual adds casual", cuppa.Filter{SFW: cuppa.SFWCasual}, "ice-0001,ice-0002,ice-0003,ice-0004,tot-0001"},
		{"unknown sfw means safe", cuppa.Filter{SFW: "spicy"}, "ice-0001,ice-0002,ice-0004,tot-0001"},
		{"category", cuppa.Filter{Categories: []string{"this-or-that"}}, "tot-0001"},
		{"any of the tags", cuppa.Filter{Tags: []string{"work", "food"}}, "ice-0001,ice-0002,tot-0001"},
		{"exclude tag", cuppa.Filter{ExcludeTags: []string{"fun"}}, "ice-0001,ice-0004,tot-0001"},
		{"exclude ids", cuppa.Filter{ExcludeIDs: []string{"ice-0001", "tot-0001"}}, "ice-0002,ice-0004"},
		{"locale", cuppa.Filter{Locale: "es"}, "ice-0004"},
		{"type", cuppa.Filter{Types: []string{cuppa.TypePoll}}, "tot-0001"},
		{"combined", cuppa.Filter{Categories: []string{"icebreakers"}, Tags: []string{"fun"}, SFW: cuppa.SFWCasual}, "ice-0002,ice-0003"},
		{"nothing matches", cuppa.Filter{Categories: []string{"nope"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ids(d.Filter(tt.filter).All()); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterKeepsVersionAndUpdatesCounts(t *testing.T) {
	d := load(t, "2026.10.14", question("ice-0001", "icebreakers", "work"), question("ice-0002", "icebreakers"))
	filtered := d.Filter(cuppa.Filter{Tags: []string{"work"}})

	if filtered.Version() != "2026.10.14" {
		t.Errorf("version = %q", filtered.Version())
	}
	categories := filtered.Categories()
	if len(categories) != 1 || categories[0].Count != 1 || categories[0].Name != "Icebreakers" {
		t.Errorf("categories = %v", categories)
	}
}
