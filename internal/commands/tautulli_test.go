package commands

import (
	"math"
	"testing"
)

func TestComputeStaleScoreCapsAgeContribution(t *testing.T) {
	now := int64(1772222400)
	sHuge := computeStaleScore(5.0, 999999, 0, 2, 1, 0, now)
	sCapped := computeStaleScore(5.0, 3650, 0, 2, 1, 0, now)
	if math.Abs(sHuge-sCapped) > 1e-9 {
		t.Fatalf("expected capped age contribution, got huge=%f capped=%f", sHuge, sCapped)
	}
}

func TestComputeStaleScoreNeverPlayedGetsModerateBonus(t *testing.T) {
	now := int64(1772222400)
	added30DaysAgo := now - (30 * 86400)

	sPlayed := computeStaleScore(5.0, 30, 0, 2, now-(30*86400), 0, now)
	sNever := computeStaleScore(5.0, 999999, 0, 2, 0, added30DaysAgo, now)
	if sNever <= sPlayed {
		t.Fatalf("expected never-played to rank above similarly-aged played item: never=%f played=%f", sNever, sPlayed)
	}
	if sNever-sPlayed > 0.2 {
		t.Fatalf("expected moderate never-played boost, got delta=%f", sNever-sPlayed)
	}
}
