package optionstruct

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func evalHeld(day string, held []compute.HeldOption, positions []compute.LedgerPosition) compute.CaptureDiff {
	return New().Evaluate(compute.StructureInput{
		Alias: "acct", Positions: positions, Held: held, HeldDay: contracts.Day(day),
		AsOf: "2026-11-06", SessionDays: sessionDays,
	}, baseParams()).Diff
}

func TestCaptureDiffBothWays(t *testing.T) {
	t.Parallel()
	held := []compute.HeldOption{
		{Underlying: "AAPL", Right: "P", Strike: "90.0", Expiry: "2026-12-18", Quantity: "-1"},
		{Underlying: "AAPL", Right: "P", Strike: "95", Expiry: "2026-12-18", Quantity: "-2"},
		{Underlying: "AAPL", Right: "P", Strike: "70", Expiry: "2026-12-18", Quantity: "0"},
	}
	d := evalHeld("2026-11-05", held, []compute.LedgerPosition{shortPut(), pos("c1", "C", "100", "-1", "2.00")})
	if d.Skipped != "" || !slices.Equal(d.CaptureOnly, []string{"AAPL 2026-12-18 95 P"}) ||
		!slices.Equal(d.LedgerOnly, []string{"AAPL 2026-12-18 100 C"}) {
		t.Errorf("diff = %+v", d)
	}
}

func TestOldCaptureSkipped(t *testing.T) {
	t.Parallel()
	held := []compute.HeldOption{{Underlying: "AAPL", Right: "P", Strike: "95", Expiry: "2026-12-18", Quantity: "-2"}}
	d := evalHeld("2026-11-02", held, []compute.LedgerPosition{shortPut()}) // 4 session days old
	if d.Skipped == "" || d.CaptureOnly != nil || d.LedgerOnly != nil {
		t.Errorf("4 session days old: %+v, want skipped and no diff", d)
	}
	if d := evalHeld("2026-11-03", held, []compute.LedgerPosition{shortPut()}); d.Skipped != "" { // 3 days
		t.Errorf("3 session days old: %+v, want a diff", d)
	}
	if d := evalHeld("", nil, nil); d.Skipped == "" {
		t.Errorf("no capture: %+v, want skipped", d)
	}
}
