// Package scheduler works out when the daemon should post next, from a
// cron line, a timezone and skip rules.
package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// searchYears is how far ahead Next looks before giving up. A cron line
// like "0 10 30 2 *" (Feb 30) never fires, so the search must end.
const searchYears = 5

// Options describe a schedule.
type Options struct {
	Cron         string         // 5 fields: minute hour day-of-month month day-of-week
	Location     *time.Location // whose clock the cron line follows
	SkipWeekends bool           // never post on Saturday or Sunday
	SkipDates    []string       // never post on these days, as YYYY-MM-DD
}

// Schedule is a parsed set of Options.
type Schedule struct {
	minute, hour, dayOfMonth, month, dayOfWeek fieldSet
	// cron rule: when both day fields are restricted, a day matching
	// either one counts; when one is "*", only the other matters
	eitherDay bool

	location     *time.Location
	skipWeekends bool
	skipDates    map[string]bool
}

// fieldSet has bit n set when value n is allowed.
type fieldSet uint64

func (f fieldSet) has(n int) bool { return f&(1<<n) != 0 }

// New checks the options and parses the cron line.
//
// Each cron field takes *, a number (5), a range (1-5), a step (*/15
// or 1-5/2), or a comma list of those. Day of week runs 0-7, where
// both 0 and 7 mean Sunday. Names like MON or JAN are not supported.
func New(o Options) (*Schedule, error) {
	fields := strings.Fields(o.Cron)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron %q must have 5 fields: minute hour day-of-month month day-of-week", o.Cron)
	}

	s := &Schedule{location: o.Location, skipWeekends: o.SkipWeekends, skipDates: map[string]bool{}}
	if s.location == nil {
		s.location = time.Local
	}

	// parse each field into the set of values it allows
	limits := []struct {
		name     string
		min, max int
		set      *fieldSet
	}{
		{"minute", 0, 59, &s.minute},
		{"hour", 0, 23, &s.hour},
		{"day of month", 1, 31, &s.dayOfMonth},
		{"month", 1, 12, &s.month},
		{"day of week", 0, 7, &s.dayOfWeek},
	}
	for i, limit := range limits {
		set, err := parseField(fields[i], limit.min, limit.max)
		if err != nil {
			return nil, fmt.Errorf("cron %q: %s: %w", o.Cron, limit.name, err)
		}
		*limit.set = set
	}
	if s.dayOfWeek.has(7) {
		s.dayOfWeek |= 1 // 7 is Sunday too
	}
	s.eitherDay = !strings.HasPrefix(fields[2], "*") && !strings.HasPrefix(fields[4], "*")

	for _, date := range o.SkipDates {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return nil, fmt.Errorf("skip date %q must look like 2026-12-25", date)
		}
		s.skipDates[date] = true
	}
	return s, nil
}

// parseField turns one cron field into the set of values it allows.
func parseField(field string, min, max int) (fieldSet, error) {
	var set fieldSet
	for part := range strings.SplitSeq(field, ",") {
		// split off an optional step: "1-5/2" is range "1-5", step 2
		rangeText, stepText, hasStep := strings.Cut(part, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return 0, fmt.Errorf("bad step in %q", part)
			}
			step = n
		}

		// the range: "*" is everything, "5" alone is 5 (or 5 to the
		// end with a step, like "5/15"), "1-5" is 1 to 5
		low, high := min, max
		if rangeText != "*" {
			lowText, highText, isRange := strings.Cut(rangeText, "-")
			var err error
			if low, err = strconv.Atoi(lowText); err != nil {
				return 0, fmt.Errorf("%q is not a number or a range", part)
			}
			high = low
			if isRange {
				if high, err = strconv.Atoi(highText); err != nil {
					return 0, fmt.Errorf("%q is not a number or a range", part)
				}
			} else if hasStep {
				high = max
			}
		}
		if low < min || high > max || low > high {
			return 0, fmt.Errorf("%q is outside %d-%d", part, min, max)
		}

		for v := low; v <= high; v += step {
			set |= 1 << v
		}
	}
	return set, nil
}

// Next returns the first time after the given one when a post is due,
// leaving out weekends and skip dates if asked.
//
// It walks the clock in the schedule's timezone, so daylight saving
// changes behave like a wall clock: a time that is skipped when clocks
// go forward fires right after the jump, and a time that happens twice
// when clocks go back fires only once.
func (s *Schedule) Next(after time.Time) (time.Time, error) {
	// "wall" is the clock reading, kept in UTC only so adding a minute
	// never jumps; it is turned into a real time once it matches
	wall := s.clock(after).Truncate(time.Minute).Add(time.Minute)
	end := wall.AddDate(searchYears, 0, 0)

	for wall.Before(end) {
		year, month, day := wall.Date()
		hour := wall.Hour()

		// skip whole days and hours that can't match, so the search
		// is quick even for rare schedules
		if !s.dayMatches(wall) {
			wall = time.Date(year, month, day+1, 0, 0, 0, 0, time.UTC)
			continue
		}
		if !s.hour.has(hour) {
			wall = time.Date(year, month, day, hour+1, 0, 0, 0, time.UTC)
			continue
		}
		if !s.minute.has(wall.Minute()) {
			wall = wall.Add(time.Minute)
			continue
		}

		// a clock reading that never happens (clocks jumped past it)
		// comes back too early, so move it to the first minute after
		// the jump
		at := time.Date(year, month, day, hour, wall.Minute(), 0, 0, s.location)
		for s.clock(at).Before(wall) {
			at = at.Add(time.Minute)
		}
		// a clock reading that happens twice gives the first one; if
		// that is already past, this reading was used, so move on
		if at.After(after) {
			return at, nil
		}
		wall = wall.Add(time.Minute)
	}
	return time.Time{}, errors.New("the schedule has no post time in the next 5 years")
}

// clock is the reading on a wall clock in the schedule's timezone,
// stored as a UTC time.
func (s *Schedule) clock(t time.Time) time.Time {
	local := t.In(s.location)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), local.Minute(), local.Second(), local.Nanosecond(), time.UTC)
}

// dayMatches reports whether a post may happen on this day at all.
func (s *Schedule) dayMatches(wall time.Time) bool {
	weekday := wall.Weekday()
	if s.skipWeekends && (weekday == time.Saturday || weekday == time.Sunday) {
		return false
	}
	if s.skipDates[wall.Format(time.DateOnly)] || !s.month.has(int(wall.Month())) {
		return false
	}

	dayOfMonth := s.dayOfMonth.has(wall.Day())
	dayOfWeek := s.dayOfWeek.has(int(weekday))
	if s.eitherDay {
		return dayOfMonth || dayOfWeek
	}
	return dayOfMonth && dayOfWeek
}
