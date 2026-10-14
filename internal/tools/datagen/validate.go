package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kashifkhan0771/cuppa/pkg/cuppa"
)

// Limits for question content.
const (
	minTextLength   = 10
	maxTextLength   = 200
	minPollOptions  = 2
	maxPollOptions  = 6
	maxOptionLength = 40
)

var (
	kebabCase    = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	prefixFormat = regexp.MustCompile(`^[a-z]{2,5}$`)
	idNumber     = regexp.MustCompile(`^[0-9]{4,}$`)
	localeFormat = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)
	githubHandle = regexp.MustCompile(`^[A-Za-z0-9-]{1,39}$`)
)

// problem is one thing wrong with the question data.
// Warnings are printed but do not fail the check.
type problem struct {
	where   string
	message string
	warning bool
}

func (p problem) String() string {
	level := "error"
	if p.warning {
		level = "warning"
	}
	return fmt.Sprintf("%s: %s: %s", level, p.where, p.message)
}

// rules holds the word lists from schema/.
type rules struct {
	tags      map[string]bool
	blocklist map[string]bool
}

// checker collects problems while walking the data.
type checker struct {
	rules
	problems []problem
}

func (c *checker) errorf(where, format string, args ...any) {
	c.problems = append(c.problems, problem{where: where, message: fmt.Sprintf(format, args...)})
}

func (c *checker) warnf(where, format string, args ...any) {
	c.problems = append(c.problems, problem{where: where, message: fmt.Sprintf(format, args...), warning: true})
}

// validate checks every file and question, then looks for
// duplicate IDs and near-duplicate questions across all files.
func validate(files []sourceFile, retired []retiredQuestion, r rules) []problem {
	c := &checker{rules: r}

	// retired IDs stay reserved, so seed the "seen" list with them
	seenIDs := map[string]string{}
	for _, rq := range retired {
		if rq.ID == "" {
			c.errorf(retiredFileName, "retired entry has no id")
			continue
		}
		if _, ok := seenIDs[rq.ID]; ok {
			c.errorf(retiredFileName, "id %s is listed twice", rq.ID)
		}
		seenIDs[rq.ID] = retiredFileName
	}

	seenCategories := map[string]string{}
	seenPrefixes := map[string]string{}
	for _, f := range files {
		c.checkFile(f, seenCategories, seenPrefixes)

		for _, q := range f.Questions {
			where := questionWhere(f.Path, q.ID)
			if q.ID != "" {
				if firstFile, ok := seenIDs[q.ID]; ok {
					c.errorf(where, "id already used in %s", firstFile)
				}
				seenIDs[q.ID] = f.Path
			}
			c.checkQuestion(where, f.Prefix, q)
		}
	}

	c.checkNearDuplicates(files)
	return c.problems
}

func questionWhere(path, id string) string {
	if id == "" {
		id = "(no id)"
	}
	return path + ": " + id
}

// checkFile checks the header fields of one category file.
func (c *checker) checkFile(f sourceFile, seenCategories, seenPrefixes map[string]string) {
	// the file name must match the category, e.g. icebreakers.yaml
	wantName := f.Category + ".yaml"
	if !kebabCase.MatchString(f.Category) {
		c.errorf(f.Path, "category %q must be lowercase kebab-case", f.Category)
	} else if filepath.Base(f.Path) != wantName {
		c.errorf(f.Path, "file name must be %s to match category %q", wantName, f.Category)
	}
	if other, ok := seenCategories[f.Category]; ok {
		c.errorf(f.Path, "category %q already defined in %s", f.Category, other)
	}
	seenCategories[f.Category] = f.Path

	if strings.TrimSpace(f.Name) == "" {
		c.errorf(f.Path, "name is required")
	}

	if !prefixFormat.MatchString(f.Prefix) {
		c.errorf(f.Path, "prefix %q must be 2 to 5 lowercase letters", f.Prefix)
	}
	if other, ok := seenPrefixes[f.Prefix]; ok {
		c.errorf(f.Path, "prefix %q already used in %s", f.Prefix, other)
	}
	seenPrefixes[f.Prefix] = f.Path

	if len(f.Questions) == 0 {
		c.errorf(f.Path, "file has no questions")
	}
}

// checkQuestion checks the fields of one question.
func (c *checker) checkQuestion(where, prefix string, q cuppa.Question) {
	// ID looks like "<prefix>-0001"
	number, hasPrefix := strings.CutPrefix(q.ID, prefix+"-")
	if !hasPrefix || !idNumber.MatchString(number) {
		c.errorf(where, "id must be %s- followed by at least 4 digits", prefix)
	}

	c.checkText(where, q.Text)
	c.checkOptions(where, q)
	c.checkTags(where, q.Tags)

	if q.SFW != cuppa.SFWSafe && q.SFW != cuppa.SFWCasual {
		c.errorf(where, "sfw must be %q or %q", cuppa.SFWSafe, cuppa.SFWCasual)
	}
	if !localeFormat.MatchString(q.Locale) {
		c.errorf(where, "locale %q must look like en or pt-BR", q.Locale)
	}
	if _, err := time.Parse(time.DateOnly, q.Added); err != nil {
		c.errorf(where, "added %q must be a date like 2026-10-14", q.Added)
	}
	if q.Author != "" && !githubHandle.MatchString(q.Author) {
		c.errorf(where, "author %q must be a GitHub handle", q.Author)
	}

	// blocked words can hide in the text or in any option
	for _, s := range append([]string{q.Text}, q.Options...) {
		c.checkWords(where, s)
	}
}

func (c *checker) checkText(where, text string) {
	if strings.TrimSpace(text) != text {
		c.errorf(where, "text has spaces at the start or end")
	}
	n := utf8.RuneCountInString(text)
	if n < minTextLength || n > maxTextLength {
		c.errorf(where, "text is %d characters, must be %d to %d", n, minTextLength, maxTextLength)
	}
	if !strings.HasSuffix(text, "?") {
		c.warnf(where, "text should end with ?")
	}
}

// checkOptions makes sure the option count fits the question type,
// and that each option is short and not repeated.
func (c *checker) checkOptions(where string, q cuppa.Question) {
	switch q.Type {
	case cuppa.TypeOpen:
		if len(q.Options) > 0 {
			c.errorf(where, "open questions must not have options")
		}
	case cuppa.TypeThisOrThat:
		if len(q.Options) != 2 {
			c.errorf(where, "this-or-that questions need exactly 2 options, got %d", len(q.Options))
		}
	case cuppa.TypePoll:
		if len(q.Options) < minPollOptions || len(q.Options) > maxPollOptions {
			c.errorf(where, "poll questions need %d to %d options, got %d", minPollOptions, maxPollOptions, len(q.Options))
		}
	default:
		c.errorf(where, "type %q must be one of open, poll, this-or-that", q.Type)
	}

	seen := map[string]bool{}
	for _, option := range q.Options {
		n := utf8.RuneCountInString(option)
		if strings.TrimSpace(option) != option || n == 0 || n > maxOptionLength {
			c.errorf(where, "option %q must be 1 to %d characters with no outer spaces", option, maxOptionLength)
		}
		key := strings.ToLower(option)
		if seen[key] {
			c.errorf(where, "option %q is repeated", option)
		}
		seen[key] = true
	}
}

func (c *checker) checkTags(where string, tags []string) {
	seen := map[string]bool{}
	for _, tag := range tags {
		if !kebabCase.MatchString(tag) {
			c.errorf(where, "tag %q must be lowercase kebab-case", tag)
		} else if !c.tags[tag] {
			c.errorf(where, "tag %q is not in schema/tags.txt", tag)
		}
		if seen[tag] {
			c.errorf(where, "tag %q is repeated", tag)
		}
		seen[tag] = true
	}
}

// checkWords looks for blocked words and long dashes.
func (c *checker) checkWords(where, s string) {
	if strings.ContainsAny(s, "\u2013\u2014") {
		c.errorf(where, "use a plain hyphen, colon or comma instead of a long dash in %q", s)
	}
	for _, word := range splitWords(s) {
		if c.blocklist[word] {
			c.errorf(where, "blocked word %q", word)
		}
	}
}

// splitWords lowercases s and splits it into words,
// treating anything that is not a letter or digit as a gap.
func splitWords(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
