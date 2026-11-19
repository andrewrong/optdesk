package optioncandidates

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func TestGateCountsByGateAndReason(t *testing.T) {
	t.Parallel()
	blocked := func(reason string) compute.GateResult {
		return compute.GateResult{Gate: "quote", Status: compute.GateBlock, Reason: reason}
	}
	cs := []compute.Candidate{
		{Gates: []compute.GateResult{blocked("no_bid")}},
		{Gates: []compute.GateResult{blocked("no_bid")}},
		{Gates: []compute.GateResult{blocked("wide_spread"), {Gate: "delta", Status: compute.GatePass}}},
	}
	if got, want := gateCounts(cs), "quote:no_bid=2,quote:wide_spread=1"; got != want {
		t.Fatalf("gateCounts = %q, want %q", got, want)
	}
}
