package optioneod

import "testing"

// The underlying struct has 9 toml fields at the merge base.
func TestUSPlanHashPinned(t *testing.T) {
	t.Parallel()
	_, hash, err := planFrom(manifestBytes, "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "bdb05a9aa0258000b837effe3eeb2d49a0a44a6530e5c5fa1991c56796eb335a" {
		t.Fatalf("hash %s", hash)
	}
}
