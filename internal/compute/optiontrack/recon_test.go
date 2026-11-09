package optiontrack

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func held(under string, exp contracts.Day, strike, right, qty string) trackapi.TrackHeld {
	return trackapi.TrackHeld{Underlying: under, Expiry: exp, Strike: strike, Right: right, Quantity: qty}
}

func TestReconCodes(t *testing.T) {
	t.Parallel()
	in := input([]compute.LedgerPosition{
		pos("match", nil),
		pos("gone", func(p *compute.LedgerPosition) { p.Strike = "90" }),
		pos("long", func(p *compute.LedgerPosition) { p.Quantity = "1"; p.Strike = "80" }),
	})
	in.HeldKnown = true
	in.Held = []trackapi.TrackHeld{
		held("AAPL", "2026-12-18", "100.00", "P", "-1"), // matches by terms, strike as number
		held("MSFT", "2026-12-18", "300", "C", "-2"),    // not in Flex
		held("AAPL", "2026-12-18", "70", "P", "3"),      // long: not a short
	}
	got := New().Track(in, defParams).Recon
	want := []trackapi.TrackRecon{
		{Underlying: "AAPL", Right: "P", Strike: "90", Quantity: "-1", Expiry: "2026-12-18", Code: "not_in_account"},
		{Underlying: "MSFT", Right: "C", Strike: "300", Quantity: "-2", Expiry: "2026-12-18", Code: "not_in_flex"},
	}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d: %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestReconSkippedWithoutCapture(t *testing.T) {
	t.Parallel()
	in := input([]compute.LedgerPosition{pos("a", nil)})
	in.Held = []trackapi.TrackHeld{held("MSFT", "2026-12-18", "300", "C", "-2")}
	if r := New().Track(in, defParams); r.Recon != nil {
		t.Fatalf("recon without capture: %+v", r.Recon)
	}
	// with a capture that holds nothing, the open position is not in the account
	in.HeldKnown, in.Held = true, nil
	if r := New().Track(in, defParams); len(r.Recon) != 1 || r.Recon[0].Code != "not_in_account" {
		t.Fatalf("%+v", r.Recon)
	}
}
