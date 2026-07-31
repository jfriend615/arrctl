package commands

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestComputeStaleScoreMatchesShellFormula(t *testing.T) {
	got := computeStaleScore(5.5, 730, 1, 2)
	want := (5.5 * 0.6) + (730.0 / 365.0 * 0.3) + (float64((2+1)-1) * 0.1)
	if got != want {
		t.Fatalf("expected shell stale score %f, got %f", want, got)
	}
}

func TestComputeStaleScoreMatchesJQPrecision(t *testing.T) {
	if got, want := computeStaleScore(12.51, 999999, 0, 2), 829.7229863013697; got != want {
		t.Fatalf("expected jq-compatible score %.16g, got %.16g", want, got)
	}
}

func TestTautulliStaleRejectsInvalidThresholdsWithExitTwo(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"tautulli", "stale", "--min-days", "-1"}, "--min-days"},
		{[]string{"tautulli", "stale", "--max-plays", "-1"}, "--max-plays"},
		{[]string{"tautulli", "stale", "--min-size-gb", "-0.1"}, "--min-size-gb"},
		{[]string{"tautulli", "stale", "--min-size-gb", "NaN"}, "--min-size-gb"},
		{[]string{"tautulli", "stale", "--limit", "0"}, "--limit"},
		{[]string{"tautulli", "stale", "--limit", "nope"}, "invalid argument"},
	}
	for _, tt := range tests {
		cmd := rootCmd()
		cmd.SetArgs(tt.args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("args %v: expected %q error, got %v", tt.args, tt.want, err)
		}
		var ee *exitError
		if !errors.As(err, &ee) || ee.code != 2 {
			t.Fatalf("args %v: expected exit code 2, got %#v", tt.args, err)
		}
	}
}

func TestDaysSinceLastPlayedTreatsNeverPlayedAsMaxAge(t *testing.T) {
	now := int64(1772222400)
	added30DaysAgo := now - (30 * 86400)
	if got := daysSinceLastPlayed(now, 0, added30DaysAgo); got != 999999 {
		t.Fatalf("expected Bash-compatible never-played sentinel, got %d", got)
	}
}

func TestFormatUnixDateMatchesJQUTCFormatting(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC-4", -4*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	if got, want := formatUnixDateUTC(1785456000), "2026-07-31"; got != want {
		t.Fatalf("expected UTC date %q, got %q", want, got)
	}
}

func TestTautulliStaleEmptyLibrariesMatchesNoCandidatesBehavior(t *testing.T) {
	oldTransport := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Query().Get("cmd") != "get_libraries" {
			return nil, errors.New("unexpected request: " + r.URL.String())
		}
		body := io.NopCloser(strings.NewReader(`{"response":{"result":"success","data":[]}}`))
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header)}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = oldTransport })

	cmd := rootCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--config", writeArrTestConfig(t, "tautulli", "http://tautulli.test"), "tautulli", "stale"})
	err := cmd.Execute()
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 {
		t.Fatalf("expected no-candidates exit code 1, got %#v", err)
	}
	if !strings.Contains(stderr.String(), "No stale candidates found") {
		t.Fatalf("expected no-candidates message, got %q", stderr.String())
	}
}
