package optioneod

import "testing"

// The underlying struct has 9 toml fields at the merge base.
func TestUSPlanHashPinned(t *testing.T) {
	t.Parallel()
	_, hash, err := planFrom(manifestBytes, "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if hash != "dcfe32c1c4cfacd2bd3924c3f5c2b425b62aeaf068d5ff85710c3397e8ccc8c9" {
		t.Fatalf("hash %s", hash)
	}
}
