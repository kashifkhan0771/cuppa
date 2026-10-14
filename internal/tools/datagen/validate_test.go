package main

import (
	"strings"
	"testing"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

var testRules = rules{
	tags:      map[string]bool{"work": true, "food": true},
	blocklist: map[string]bool{"beer": true},
}

func goodQuestion() cuppa.Question {
	return cuppa.Question{
		ID:     "ice-0001",
		Text:   "What is your favorite snack?",
		Type:   cuppa.TypeOpen,
		Tags:   []string{"food"},
		SFW:    cuppa.SFWSafe,
		Locale: "en",
		Added:  "2026-10-14",
		Author: "someone",
	}
}

func goodFile(questions ...cuppa.Question) sourceFile {
	return sourceFile{
		Path:      "questions/icebreakers.yaml",
		Category:  "icebreakers",
		Name:      "Icebreakers",
		Prefix:    "ice",
		Questions: questions,
	}
}

// errorsOnly drops warnings so tests can check real failures.
func errorsOnly(problems []problem) []problem {
	var out []problem
	for _, p := range problems {
		if !p.warning {
			out = append(out, p)
		}
	}
	return out
}

func TestValidateGoodData(t *testing.T) {
	problems := validate([]sourceFile{goodFile(goodQuestion())}, nil, testRules)
	if len(problems) != 0 {
		t.Fatalf("want no problems, got %v", problems)
	}
}

// each case breaks one field of a good question
// and expects an error containing the given text
func TestValidateQuestionErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(q *cuppa.Question)
		want   string
	}{
		{"wrong prefix", func(q *cuppa.Question) { q.ID = "wyr-0001" }, "id must be ice-"},
		{"short id number", func(q *cuppa.Question) { q.ID = "ice-01" }, "id must be ice-"},
		{"text too short", func(q *cuppa.Question) { q.Text = "Why?" }, "text is 4 characters"},
		{"text too long", func(q *cuppa.Question) { q.Text = strings.Repeat("a", 200) + "?" }, "text is 201 characters"},
		{"text outer spaces", func(q *cuppa.Question) { q.Text = " What is your favorite snack?" }, "spaces at the start or end"},
		{"unknown type", func(q *cuppa.Question) { q.Type = "quiz" }, `type "quiz"`},
		{"open with options", func(q *cuppa.Question) { q.Options = []string{"A", "B"} }, "must not have options"},
		{"this-or-that with 3", func(q *cuppa.Question) {
			q.Type = cuppa.TypeThisOrThat
			q.Options = []string{"A", "B", "C"}
		}, "exactly 2 options"},
		{"poll with 1", func(q *cuppa.Question) {
			q.Type = cuppa.TypePoll
			q.Options = []string{"A"}
		}, "2 to 6 options"},
		{"poll with 7", func(q *cuppa.Question) {
			q.Type = cuppa.TypePoll
			q.Options = []string{"A", "B", "C", "D", "E", "F", "G"}
		}, "2 to 6 options"},
		{"repeated option", func(q *cuppa.Question) {
			q.Type = cuppa.TypeThisOrThat
			q.Options = []string{"Tea", "tea"}
		}, "is repeated"},
		{"long option", func(q *cuppa.Question) {
			q.Type = cuppa.TypeThisOrThat
			q.Options = []string{"A", strings.Repeat("b", 41)}
		}, "1 to 40 characters"},
		{"empty option", func(q *cuppa.Question) {
			q.Type = cuppa.TypeThisOrThat
			q.Options = []string{"A", ""}
		}, "1 to 40 characters"},
		{"tag not allowed", func(q *cuppa.Question) { q.Tags = []string{"music"} }, "not in schema/tags.txt"},
		{"tag bad format", func(q *cuppa.Question) { q.Tags = []string{"Food"} }, "kebab-case"},
		{"repeated tag", func(q *cuppa.Question) { q.Tags = []string{"food", "food"} }, `tag "food" is repeated`},
		{"bad sfw", func(q *cuppa.Question) { q.SFW = "spicy" }, "sfw must be"},
		{"missing locale", func(q *cuppa.Question) { q.Locale = "" }, "locale"},
		{"bad date", func(q *cuppa.Question) { q.Added = "14/10/2026" }, "must be a date"},
		{"bad author", func(q *cuppa.Question) { q.Author = "not a handle" }, "GitHub handle"},
		{"blocked word in text", func(q *cuppa.Question) { q.Text = "What is your favorite beer?" }, `blocked word "beer"`},
		{"blocked word in option", func(q *cuppa.Question) {
			q.Type = cuppa.TypeThisOrThat
			q.Options = []string{"Tea", "BEER"}
		}, `blocked word "beer"`},
		{"long dash", func(q *cuppa.Question) { q.Text = "What is your favorite snack \u2014 and why?" }, "long dash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := goodQuestion()
			tt.change(&q)
			problems := errorsOnly(validate([]sourceFile{goodFile(q)}, nil, testRules))
			if !hasMessage(problems, tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, problems)
			}
		})
	}
}

func TestValidateBlockedWordIsWholeWordOnly(t *testing.T) {
	q := goodQuestion()
	q.Text = "What is your favorite beers?" // "beers" is not "beer"
	if problems := errorsOnly(validate([]sourceFile{goodFile(q)}, nil, testRules)); len(problems) != 0 {
		t.Fatalf("want no errors, got %v", problems)
	}
}

func TestValidateMissingQuestionMarkIsWarning(t *testing.T) {
	q := goodQuestion()
	q.Text = "Tell us your favorite snack."
	problems := validate([]sourceFile{goodFile(q)}, nil, testRules)
	if len(problems) != 1 || !problems[0].warning {
		t.Fatalf("want exactly one warning, got %v", problems)
	}
}

func TestValidateFileErrors(t *testing.T) {
	tests := []struct {
		name   string
		change func(f *sourceFile)
		want   string
	}{
		{"file name mismatch", func(f *sourceFile) { f.Path = "questions/other.yaml" }, "file name must be icebreakers.yaml"},
		{"bad category", func(f *sourceFile) { f.Category = "Ice Breakers" }, "kebab-case"},
		{"missing name", func(f *sourceFile) { f.Name = "" }, "name is required"},
		{"bad prefix", func(f *sourceFile) { f.Prefix = "ICE" }, "2 to 5 lowercase letters"},
		{"no questions", func(f *sourceFile) { f.Questions = nil }, "no questions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := goodFile(goodQuestion())
			tt.change(&f)
			if problems := validate([]sourceFile{f}, nil, testRules); !hasMessage(problems, tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, problems)
			}
		})
	}
}

func TestValidateDuplicates(t *testing.T) {
	q1 := goodQuestion()
	q2 := goodQuestion()
	q2.Text = "What is the best board game ever made?"

	t.Run("same id twice", func(t *testing.T) {
		problems := validate([]sourceFile{goodFile(q1, q2)}, nil, testRules)
		if !hasMessage(problems, "id already used") {
			t.Fatalf("want duplicate id error, got %v", problems)
		}
	})

	t.Run("id is retired", func(t *testing.T) {
		retired := []retiredQuestion{{ID: "ice-0001"}}
		problems := validate([]sourceFile{goodFile(q1)}, retired, testRules)
		if !hasMessage(problems, "id already used in "+retiredFileName) {
			t.Fatalf("want retired id error, got %v", problems)
		}
	})

	t.Run("same category and prefix twice", func(t *testing.T) {
		q2.ID = "ice-0002"
		problems := validate([]sourceFile{goodFile(q1), goodFile(q2)}, nil, testRules)
		if !hasMessage(problems, "category \"icebreakers\" already defined") || !hasMessage(problems, "prefix \"ice\" already used") {
			t.Fatalf("want category and prefix errors, got %v", problems)
		}
	})
}

func hasMessage(problems []problem, want string) bool {
	for _, p := range problems {
		if strings.Contains(p.message, want) {
			return true
		}
	}
	return false
}
