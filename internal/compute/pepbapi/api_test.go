package pepbapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/pepbapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// TestBuildFuncContract: a caller reads State, Ratio and History of a fake BuildFunc,
// and the fake sees the input the caller gave.
func TestBuildFuncContract(t *testing.T) {
	t.Parallel()
	var seen pepbapi.Input
	var build pepbapi.BuildFunc = func(in pepbapi.Input) pepbapi.Result {
		seen = in
		return pepbapi.Result{
			PE: pepbapi.Reading{
				Kind: "pe", State: pepbapi.StateOK, Day: in.Today, Ratio: "18.2",
				History: []pepbapi.Point{{Day: "2026-11-12", Text: "18.0", Value: 18.0}, {Day: "2026-11-13", Text: "18.2", Value: 18.2}},
			},
			PB: pepbapi.Reading{Kind: "pb", State: pepbapi.StateLoss, Reason: "negative_equity"},
		}
	}
	in := pepbapi.Input{Today: "2026-11-13", Bars: []contracts.PriceBar{{}}}
	res := build(in)
	if seen.Today != "2026-11-13" || len(seen.Bars) != 1 {
		t.Fatalf("input not passed: %+v", seen)
	}
	if res.PE.State != pepbapi.StateOK || res.PE.Ratio != "18.2" {
		t.Fatalf("PE = %+v", res.PE)
	}
	if n := len(res.PE.History); n != 2 || res.PE.History[0].Day > res.PE.History[1].Day {
		t.Fatalf("history = %+v", res.PE.History)
	}
	if res.PB.State != pepbapi.StateLoss || res.PB.Ratio != "" {
		t.Fatalf("PB = %+v", res.PB)
	}
	if pepbapi.MinDays != 250 || pepbapi.StalePriceDays != 5 || pepbapi.StateMissing != "missing_input" {
		t.Fatal("constants changed")
	}
}
