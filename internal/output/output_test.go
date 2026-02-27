package output

import "testing"

func TestToStrings(t *testing.T) {
	got := ToStrings(1, "a", true)
	if len(got) != 3 || got[0] != "1" || got[2] != "true" {
		t.Fatalf("unexpected: %#v", got)
	}
}
