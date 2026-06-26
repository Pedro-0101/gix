package app

import (
	"encoding/json"
	"time"
)

// recurrenceRule is the closed, minimal repeat spec the AI emits and the
// scheduler reads. weekday/time are informational (display + AI contract);
// proximoFireAt only uses freq+interval, because the first fire_at is an
// absolute timestamp that already fixes the wall-clock time and weekday.
type recurrenceRule struct {
	Freq     string `json:"freq"`              // daily|weekly|monthly|yearly
	Interval int    `json:"interval"`          // every N periods
	Weekday  string `json:"weekday,omitempty"` // mon..sun, weekly only
	Time     string `json:"time,omitempty"`    // "09:00"
}

// maxRecurrenceSteps caps the catch-up loop so a wildly stale alert can never
// spin forever (e.g. a daily alert untouched for years).
const maxRecurrenceSteps = 4000

// parseRecurrence decodes a stored recurrence string. Returns ok=false for the
// empty (one-shot) string or any invalid/blank-freq JSON.
func parseRecurrence(s string) (recurrenceRule, bool) {
	if s == "" {
		return recurrenceRule{}, false
	}
	var r recurrenceRule
	if err := json.Unmarshal([]byte(s), &r); err != nil || r.Freq == "" {
		return recurrenceRule{}, false
	}
	return r, true
}

// marshalRecurrence serializes a rule, or "" when there's none (one-shot).
func marshalRecurrence(r *recurrenceRule) string {
	if r == nil || r.Freq == "" {
		return ""
	}
	b, err := json.Marshal(r)
	if err != nil {
		return ""
	}
	return string(b)
}

// proximoFireAt returns the next occurrence strictly after now, advancing from
// the previous fire time `last` by whole freq×interval periods. Arithmetic
// happens in last's location so wall-clock time and weekday stay stable across
// DST. A stale recurring alert advances once past now (no backlog spam).
func proximoFireAt(rule recurrenceRule, last, now time.Time) time.Time {
	next := last
	for i := 0; i < maxRecurrenceSteps; i++ {
		next = advance(rule, next)
		if next.After(now) {
			return next
		}
	}
	return next
}

// advance steps a time forward by one freq×interval period (>=1 period).
func advance(rule recurrenceRule, t time.Time) time.Time {
	n := rule.Interval
	if n < 1 {
		n = 1
	}
	switch rule.Freq {
	case "weekly":
		return t.AddDate(0, 0, 7*n)
	case "monthly":
		return t.AddDate(0, n, 0)
	case "yearly":
		return t.AddDate(n, 0, 0)
	default: // daily
		return t.AddDate(0, 0, n)
	}
}
