package optioneod

import "testing"

// The underlying struct has 9 toml fields at the merge base. The hash changed with the window keys (in_session,
// open_after_minutes) of the in-session change.
func TestUSPlanHashPinned(t *testing.T) {
	t.Parallel()
	_, hash, err := planFrom(manifestBytes, "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "df4e2c47a5997d411867b819f90c0373137e93bdfb623b7a580131227ace7901" {
		t.Fatalf("hash %s", hash)
	}
}
