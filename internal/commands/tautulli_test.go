package commands

import "testing"

func TestComputeStaleScoreMatchesShellFormula(t *testing.T) {
	got := computeStaleScore(5.5, 730, 1, 2)
	want := (5.5 * 0.6) + (730.0 / 365.0 * 0.3) + (float64((2+1)-1) * 0.1)
	if got != want {
		t.Fatalf("expected shell stale score %f, got %f", want, got)
	}
}

func TestDaysSinceLastPlayedTreatsNeverPlayedAsMaxAge(t *testing.T) {
	now := int64(1772222400)
	added30DaysAgo := now - (30 * 86400)
	if got := daysSinceLastPlayed(now, 0, added30DaysAgo); got != 30 {
		t.Fatalf("expected never-played age to fall back to added_at, got %d", got)
	}
}
