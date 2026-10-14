package main

import (
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"kitten", "sitting", 3},
		{"same", "same", 0},
		{"café", "cafe", 1}, // counts letters, not bytes
	}
	for _, tt := range tests {
		if got := editDistance([]rune(tt.a), []rune(tt.b)); got != tt.want {
			t.Errorf("editDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestWordOverlap(t *testing.T) {
	a := map[string]bool{"tea": true, "coffee": true}
	b := map[string]bool{"coffee": true, "juice": true}
	if got := wordOverlap(a, b); got != 1.0/3.0 {
		t.Errorf("got %v, want 1/3", got)
	}
	if got := wordOverlap(map[string]bool{}, map[string]bool{}); got != 0 {
		t.Errorf("empty sets: got %v, want 0", got)
	}
}

func TestNearDuplicates(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		similar bool
	}{
		{"same words, new order", "Coffee or tea?", "Tea or coffee?", true},
		{"only stopwords differ", "Would you rather fly or be invisible?", "Rather fly or be invisible?", true},
		{"one letter typo", "What is your favorite comfort food?", "What is your favourite comfort food?", true},
		{"different questions", "Pizza or tacos?", "Cats or dogs?", false},
		{"one shared word", "Pizza or pasta?", "Pizza or burgers?", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first := goodQuestion()
			first.Text = tt.a
			second := goodQuestion()
			second.ID = "ice-0002"
			second.Text = tt.b

			c := &checker{rules: testRules}
			c.checkNearDuplicates([]sourceFile{goodFile(first, second)})
			if got := len(c.problems) > 0; got != tt.similar {
				t.Fatalf("similar = %v, want %v (problems: %v)", got, tt.similar, c.problems)
			}
		})
	}
}

func TestNearDuplicatesSkipsEmptyText(t *testing.T) {
	// both texts are only stopwords, so nothing is left to compare
	first := cuppa.Question{ID: "ice-0001", Text: "What would you do?"}
	second := cuppa.Question{ID: "ice-0002", Text: "What do you do?"}

	c := &checker{rules: testRules}
	c.checkNearDuplicates([]sourceFile{goodFile(first, second)})
	if len(c.problems) != 0 {
		t.Fatalf("want no problems, got %v", c.problems)
	}
}
