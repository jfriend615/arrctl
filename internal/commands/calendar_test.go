package commands

import (
	"testing"
	"time"
)

func TestCalendarRangeDays(t *testing.T) {
	now := time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC)
	start, end := calendarRange(now, 7, false)
	if start != "2026-02-26" || end != "2026-03-05" {
		t.Fatalf("got %s..%s", start, end)
	}
}

func TestCalendarRangeWeek(t *testing.T) {
	now := time.Date(2026, 2, 26, 12, 0, 0, 0, time.UTC) // Thu
	start, end := calendarRange(now, 7, true)
	if start != "2026-02-23" || end != "2026-03-01" {
		t.Fatalf("got %s..%s", start, end)
	}
}

func TestFirstDate(t *testing.T) {
	if v := firstDate("", "2026-01-02T00:00:00Z", "2026-01-03"); v != "2026-01-02T00:00:00Z" {
		t.Fatalf("unexpected %q", v)
	}
}
