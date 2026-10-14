package cuppa

import "slices"

// Filter picks which questions to keep. Empty fields mean "no limit",
// except SFW, which keeps only "safe" questions unless set to "casual".
type Filter struct {
	Categories  []string // keep these categories
	Tags        []string // keep questions with at least one of these tags
	ExcludeTags []string // drop questions with any of these tags
	ExcludeIDs  []string // drop these question IDs
	SFW         string   // "safe" or empty: safe only; "casual": safe and casual
	Locale      string   // keep this locale, like "en"
	Types       []string // keep these types, like "open" or "poll"
}

// Filter returns a new Dataset with only the questions that match f.
func (d *Dataset) Filter(f Filter) *Dataset {
	var kept []Question
	for _, q := range d.questions {
		if f.matches(q) {
			kept = append(kept, q)
		}
	}
	return newDataset(d.version, kept, d.categoryNames)
}

func (f Filter) matches(q Question) bool {
	if len(f.Categories) > 0 && !slices.Contains(f.Categories, q.Category) {
		return false
	}
	if len(f.Types) > 0 && !slices.Contains(f.Types, q.Type) {
		return false
	}
	if f.Locale != "" && q.Locale != f.Locale {
		return false
	}
	if slices.Contains(f.ExcludeIDs, q.ID) {
		return false
	}

	// anything other than "casual" means safe only, the careful default
	if q.SFW != SFWSafe && f.SFW != SFWCasual {
		return false
	}

	if len(f.Tags) > 0 && !hasAnyTag(q, f.Tags) {
		return false
	}
	if hasAnyTag(q, f.ExcludeTags) {
		return false
	}
	return true
}

func hasAnyTag(q Question, tags []string) bool {
	for _, tag := range tags {
		if slices.Contains(q.Tags, tag) {
			return true
		}
	}
	return false
}
