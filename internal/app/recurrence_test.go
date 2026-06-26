package app

import (
	"testing"
	"time"
)

func TestProximoFireAtAdvancesPastNow(t *testing.T) {
	loc := time.UTC
	// Daily alert that last fired 3 days ago; must land exactly one day after the
	// most recent past occurrence, i.e. strictly in the future, same wall time.
	last := time.Date(2026, 6, 23, 9, 0, 0, 0, loc)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, loc)
	next := proximoFireAt(recurrenceRule{Freq: "daily", Interval: 1}, last, now)
	want := time.Date(2026, 6, 27, 9, 0, 0, 0, loc)
	if !next.Equal(want) {
		t.Fatalf("daily catch-up: got %v want %v", next, want)
	}
}

func TestProximoFireAtWeeklyPreservesWeekday(t *testing.T) {
	loc := time.UTC
	last := time.Date(2026, 6, 22, 8, 0, 0, 0, loc) // a Monday
	now := time.Date(2026, 6, 24, 0, 0, 0, 0, loc)  // Wednesday
	next := proximoFireAt(recurrenceRule{Freq: "weekly", Interval: 1, Weekday: "mon"}, last, now)
	want := time.Date(2026, 6, 29, 8, 0, 0, 0, loc) // next Monday
	if !next.Equal(want) || next.Weekday() != time.Monday {
		t.Fatalf("weekly: got %v (%v) want %v", next, next.Weekday(), want)
	}
}

func TestProximoFireAtMonthlyAndInterval(t *testing.T) {
	loc := time.UTC
	last := time.Date(2026, 1, 15, 7, 30, 0, 0, loc)
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, loc)
	next := proximoFireAt(recurrenceRule{Freq: "monthly", Interval: 2}, last, now)
	want := time.Date(2026, 7, 15, 7, 30, 0, 0, loc) // +2 months stepped past now
	if !next.Equal(want) {
		t.Fatalf("monthly interval 2: got %v want %v", next, want)
	}
}

func TestParseAndMarshalRecurrence(t *testing.T) {
	if _, ok := parseRecurrence(""); ok {
		t.Fatal("empty string should not parse as a rule")
	}
	if _, ok := parseRecurrence("not json"); ok {
		t.Fatal("garbage should not parse")
	}
	r, ok := parseRecurrence(`{"freq":"weekly","interval":1,"weekday":"mon","time":"09:00"}`)
	if !ok || r.Freq != "weekly" || r.Interval != 1 || r.Weekday != "mon" {
		t.Fatalf("parseRecurrence wrong: %+v ok=%v", r, ok)
	}
	if marshalRecurrence(nil) != "" {
		t.Fatal("nil rule should marshal to empty string")
	}
	out := marshalRecurrence(&recurrenceRule{Freq: "daily", Interval: 1})
	if out == "" || out[0] != '{' {
		t.Fatalf("unexpected marshal: %q", out)
	}
}
