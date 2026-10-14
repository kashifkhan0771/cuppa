package questions

import (
	"strings"
)

// Two questions count as near-duplicates when either score reaches its limit.
const (
	maxWordOverlap  = 0.8 // share of words in common (Jaccard)
	maxTextSameness = 0.9 // how alike the letters are (Levenshtein ratio)
)

// stopwords are common words dropped before comparing questions,
// so "Would you rather X or Y?" questions are compared on X and Y.
var stopwords = map[string]bool{
	"a": true, "an": true, "and": true, "any": true, "are": true, "as": true,
	"at": true, "be": true, "by": true, "can": true, "could": true, "did": true,
	"do": true, "does": true, "ever": true, "for": true, "from": true, "had": true,
	"has": true, "have": true, "how": true, "i": true, "if": true, "in": true,
	"is": true, "it": true, "its": true, "me": true, "my": true, "of": true,
	"on": true, "only": true, "or": true, "rather": true, "so": true, "than": true,
	"that": true, "the": true, "their": true, "them": true, "there": true, "this": true,
	"to": true, "up": true, "was": true, "we": true, "were": true, "what": true,
	"when": true, "where": true, "which": true, "who": true, "why": true, "will": true,
	"with": true, "would": true, "you": true, "your": true,
}

// normalizedQuestion is a question text cut down to its meaningful words.
type normalizedQuestion struct {
	where string
	id    string
	words map[string]bool
	text  string // the meaningful words joined by spaces
}

func normalize(text string) (map[string]bool, string) {
	words := map[string]bool{}
	var kept []string
	for _, word := range splitWords(text) {
		if stopwords[word] {
			continue
		}
		words[word] = true
		kept = append(kept, word)
	}
	return words, strings.Join(kept, " ")
}

// checkNearDuplicates compares every pair of questions.
// This is fine for a few thousand questions.
func (c *checker) checkNearDuplicates(files []File) {
	var list []normalizedQuestion
	for _, f := range files {
		for _, q := range f.Questions {
			words, text := normalize(q.Text)
			// skip questions with nothing left to compare
			if text == "" {
				continue
			}
			list = append(list, normalizedQuestion{where: questionWhere(f.Path, q.ID), id: q.ID, words: words, text: text})
		}
	}

	for i := range list {
		for j := i + 1; j < len(list); j++ {
			a, b := list[i], list[j]
			if wordOverlap(a.words, b.words) >= maxWordOverlap || textSameness(a.text, b.text) >= maxTextSameness {
				c.errorf(b.where, "too similar to %s", a.id)
			}
		}
	}
}

// wordOverlap is the number of shared words divided by
// the number of distinct words in both sets together.
func wordOverlap(a, b map[string]bool) float64 {
	shared := 0
	for word := range a {
		if b[word] {
			shared++
		}
	}
	total := len(a) + len(b) - shared
	if total == 0 {
		return 0
	}
	return float64(shared) / float64(total)
}

// textSameness is 1 minus the edit distance divided by the longer length.
// 1 means the texts are the same, 0 means nothing in common.
func textSameness(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	longest := max(len(ar), len(br))
	if longest == 0 {
		return 1
	}
	return 1 - float64(editDistance(ar, br))/float64(longest)
}

// editDistance is the Levenshtein distance: the fewest single-letter
// inserts, deletes or swaps to turn a into b. Keeps only two rows in memory.
func editDistance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}
