package optiontier

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// TestTiererContractReal runs the cases of the interface's TestTiererContractFake (a green, an amber and a
// grey row, one account) against the real Tierer with a fake evaluator.
func TestTiererContractReal(t *testing.T) {
	t.Parallel()
	in := optiontierapi.Input{
		Day:    "2026-11-17",
		Params: DefaultParams(),
		Rows: []optiontierapi.Row{
			{Key: "c", Company: "CCC", Strike: "50.00"},
			{Key: "a", Company: "AAA", Spot: "96.50", Strike: "90.00", Bid: "0.94", BuyPrice: "90.00", Delta: "-0.22", DTE: 31},
			{Key: "b", Company: "BBB", Spot: "300.00", Strike: "290.00", Bid: "2.00", BuyPrice: "290.00", Delta: "-0.22", DTE: 31},
		},
		Accounts: []optiontierapi.AccountCash{{Alias: "main", Cash: "29000.00", Currency: "USD", CaptureDay: "2026-11-16", AgeDays: 1}},
	}
	in.Rows[2].Strike = "291.00"
	in.Rows[2].BuyPrice = "291.00"
	// Cash covers 29000.00: row b needs 29100.00 and is short; row a needs 9000.00 and fits.
	res, err := New(&fakeEvaluator{}).Judge(in)
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	checkContract(t, res)
}

func checkContract(t *testing.T, res optiontierapi.Result) {
	t.Helper()
	by := map[string]optiontierapi.Verdict{}
	for _, v := range res.Verdicts {
		by[v.Key] = v
	}
	if g := by["a"]; g.Tier != "fits" || g.Place != 1 || g.FitAlias != "main" || len(g.Cash) != 1 || g.Cash[0].Verdict != "fits" {
		t.Errorf("green row = %+v", g)
	}
	if a := by["b"]; a.Tier != "check" || !reflect.DeepEqual(a.Badges, []string{"cash_short"}) || a.FitAlias != "" || a.Cash[0].Verdict != "short" {
		t.Errorf("amber row = %+v", a)
	}
	if g := by["c"]; g.Tier != "grey" || g.Grey != "no_buy_price" {
		t.Errorf("grey row = %+v", g)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(res.Order, want) {
		t.Errorf("Order = %v, want %v", res.Order, want)
	}
	if !res.CashBuilt || res.Params.DeltaMax != "0.3" {
		t.Errorf("CashBuilt %v, Params %+v", res.CashBuilt, res.Params)
	}
	for _, v := range res.Verdicts {
		if v.Badges == nil || v.Infos == nil || v.Notes == nil {
			t.Errorf("nil list in %+v", v)
		}
	}
}
