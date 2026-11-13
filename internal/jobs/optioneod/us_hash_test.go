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
	if hash != "312297c3632760f775bf0388ceeec9ff926f85cc52175ce8ea5518d7df62926a" {
		t.Fatalf("hash %s", hash)
	}
}
