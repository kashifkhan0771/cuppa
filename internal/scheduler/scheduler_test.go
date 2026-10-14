package scheduler

import (
	"strings"
	"testing"
	"time"
)

func mustNew(t *testing.T, o Options) *Schedule {
	t.Helper()
	s, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func at(t *testing.T, loc *time.Location, text string) time.Time {
	t.Helper()
	when, err := time.ParseInLocation("2006-01-02 15:04", text, loc)
	if err != nil {
		t.Fatal(err)
	}
	return when
}

func TestNext(t *testing.T) {
	// 2026-10-14 is a Wednesday
	tests := []struct {
		cron, after, want string
	}{
		{"0 10 * * *", "2026-10-14 09:00", "2026-10-14 10:00"},
		{"0 10 * * *", "2026-10-14 10:00", "2026-10-15 10:00"}, // strictly after
		{"0 10 * * 1-5", "2026-10-16 11:00", "2026-10-19 10:00"},
		{"*/15 9 * * *", "2026-10-14 09:16", "2026-10-14 09:30"},
		{"5/20 * * * *", "2026-10-14 09:30", "2026-10-14 09:45"},
		{"0 9,17 * * *", "2026-10-14 10:00", "2026-10-14 17:00"},
		{"0 0 1 1 *", "2026-10-14 10:00", "2027-01-01 00:00"},
		{"0 10 * * 7", "2026-10-14 10:00", "2026-10-18 10:00"}, // 7 is Sunday
		{"0 10 31 * *", "2026-10-31 11:00", "2026-12-31 10:00"},
		// both day fields set: either one matches (the 20th, or a Friday)
		{"0 10 20 * 5", "2026-10-14 10:00", "2026-10-16 10:00"},
		{"0 10 20 * 5", "2026-10-16 10:00", "2026-10-20 10:00"},
		// a day field starting with "*" counts as "*": only the other matters
		{"0 10 */1 * 5", "2026-10-14 10:00", "2026-10-16 10:00"},
	}
	for _, tt := range tests {
		s := mustNew(t, Options{Cron: tt.cron, Location: time.UTC})
		got, err := s.Next(at(t, time.UTC, tt.after))
		if err != nil || got.Format("2006-01-02 15:04") != tt.want {
			t.Errorf("%q after %s: got %s %v, want %s", tt.cron, tt.after, got, err, tt.want)
		}
	}
}

func TestNextUsesTimezone(t *testing.T) {
	karachi, err := time.LoadLocation("Asia/Karachi") // UTC+5
	if err != nil {
		t.Fatal(err)
	}
	s := mustNew(t, Options{Cron: "0 10 * * *", Location: karachi})
	got, _ := s.Next(at(t, time.UTC, "2026-10-14 06:00")) // 11:00 in Karachi
	if want := at(t, karachi, "2026-10-15 10:00"); !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSkipRules(t *testing.T) {
	s := mustNew(t, Options{Cron: "0 10 * * *", Location: time.UTC, SkipWeekends: true, SkipDates: []string{"2026-10-19"}})

	// Friday 10:00 is done; Sat and Sun are weekends, Monday is skipped
	got, _ := s.Next(at(t, time.UTC, "2026-10-16 10:00"))
	if want := "2026-10-20 10:00"; got.Format("2006-01-02 15:04") != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestDaylightSaving(t *testing.T) {
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}

	// 2026-03-08: clocks jump from 02:00 to 03:00, so 02:30 never
	// happens; the post goes out right after the jump instead of
	// waiting a whole day
	s := mustNew(t, Options{Cron: "30 2 * * *", Location: newYork})
	got, _ := s.Next(at(t, newYork, "2026-03-08 01:00"))
	if want := at(t, newYork, "2026-03-08 03:00"); !got.Equal(want) {
		t.Errorf("spring forward: got %s", got)
	}

	// 2026-11-01: clocks go back from 02:00 to 01:00, so 01:30 happens
	// twice; after the first one, the next post is the next day
	s = mustNew(t, Options{Cron: "30 1 * * *", Location: newYork})
	first, _ := s.Next(at(t, newYork, "2026-11-01 00:00"))
	second, _ := s.Next(first)
	if first.Day() != 1 || second.Day() != 2 || second.Hour() != 1 {
		t.Errorf("fall back: first %s, second %s", first, second)
	}
	// starting between the two 01:30s also gives the next day
	between := first.Add(45 * time.Minute) // the second 01:15
	if got, _ := s.Next(between); got.Day() != 2 {
		t.Errorf("fall back from %s: got %s", between, got)
	}
}

func TestNeverFires(t *testing.T) {
	s := mustNew(t, Options{Cron: "0 10 30 2 *", Location: time.UTC})
	if _, err := s.Next(at(t, time.UTC, "2026-10-14 10:00")); err == nil {
		t.Error("want an error for Feb 30")
	}
}

func TestBadOptions(t *testing.T) {
	tests := map[string]Options{
		"4 fields":       {Cron: "0 10 * *"},
		"minute too big": {Cron: "60 10 * * *"},
		"day zero":       {Cron: "0 10 0 * *"},
		"backwards":      {Cron: "0 10-9 * * *"},
		"zero step":      {Cron: "*/0 10 * * *"},
		"bad step":       {Cron: "*/x 10 * * *"},
		"name":           {Cron: "0 10 * * MON"},
		"bad range end":  {Cron: "0 1-x * * *"},
		"empty part":     {Cron: "0 1,,2 * * *"},
		"bad skip date":  {Cron: "0 10 * * *", SkipDates: []string{"25/12/2026"}},
	}
	for name, o := range tests {
		if _, err := New(o); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}

	// errors say which field is wrong
	_, err := New(Options{Cron: "0 25 * * *"})
	if err == nil || !strings.Contains(err.Error(), "hour") {
		t.Errorf("got %v", err)
	}
}
