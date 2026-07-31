package commands

import (
	"context"
	"errors"
	"strings"
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

func TestCalendarRejectsConflictingServiceFlags(t *testing.T) {
	for _, args := range [][]string{
		{"calendar", "--sonarr", "--radarr"},
		{"calendar", "--radarr", "--sonarr"},
	} {
		cmd := rootCmd()
		cmd.SetArgs(args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "cannot be used together") {
			t.Fatalf("args %v: expected conflicting service error, got %v", args, err)
		}
	}
}

func TestUnifiedCalendarContinuesWhenOneServiceFails(t *testing.T) {
	oldFetch := fetchCalendarForUnified
	oldPrinter := printJSON
	oldFormat := format
	t.Cleanup(func() {
		fetchCalendarForUnified = oldFetch
		printJSON = oldPrinter
		format = oldFormat
	})
	fetchCalendarForUnified = func(_ context.Context, service string, _ bool, _, _ string) ([]calendarRow, error) {
		if service == "sonarr" {
			return nil, calendarFetchError{service: service, err: errors.New("unavailable")}
		}
		return []calendarRow{{Date: "2026-08-01", Title: "Example", Service: "Radarr"}}, nil
	}
	var captured any
	printJSON = func(v any) error {
		captured = v
		return nil
	}

	cmd := rootCmd()
	cmd.SetArgs([]string{"--format", "json", "calendar"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	rows, ok := captured.([]calendarRow)
	if !ok || len(rows) != 1 || rows[0].Service != "Radarr" {
		t.Fatalf("unexpected fallback result: %#v", captured)
	}
}

func TestUnifiedCalendarDoesNotHideConfigurationErrors(t *testing.T) {
	oldFetch := fetchCalendarForUnified
	t.Cleanup(func() { fetchCalendarForUnified = oldFetch })
	fetchCalendarForUnified = func(_ context.Context, _ string, _ bool, _, _ string) ([]calendarRow, error) {
		return nil, errors.New("invalid JSON in config")
	}

	cmd := rootCmd()
	cmd.SetArgs([]string{"calendar"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "invalid JSON in config") {
		t.Fatalf("expected configuration failure, got %v", err)
	}
}

func TestSingleServiceCalendarStillReportsFailure(t *testing.T) {
	oldFetch := fetchCalendarForUnified
	t.Cleanup(func() { fetchCalendarForUnified = oldFetch })
	fetchCalendarForUnified = func(_ context.Context, _ string, _ bool, _, _ string) ([]calendarRow, error) {
		return nil, errors.New("sonarr unavailable")
	}

	cmd := rootCmd()
	cmd.SetArgs([]string{"calendar", "--sonarr"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "sonarr unavailable") {
		t.Fatalf("expected explicit service failure, got %v", err)
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
