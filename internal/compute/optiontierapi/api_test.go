package optiontierapi_test

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// fakeTierer answers made-up AAA/BBB/CCC verdicts by key: AAA green, BBB amber, CCC grey, others plain.
type fakeTierer struct{}

func (fakeTierer) Judge(in optiontierapi.Input) (optiontierapi.Result, error) {
	res := optiontierapi.Result{
		Order:     []string{},
		Common:    []string{"earnings_unconfirmed"},
		Kept:      []optiontierapi.Kept{{Code: "no_open_interest", N: 1, Every: false}},
		CashBuilt: len(in.Accounts) > 0,
		Params:    in.Params,
	}
	for _, r := range in.Rows {
		v := optiontierapi.Verdict{Key: r.Key, Badges: []string{}, Infos: []string{}, Notes: []string{}}
		switch r.Company {
		case "AAA":
			v.Tier, v.Place = "fits", 1
			v.Cash = []optiontierapi.CashCell{{Alias: "main", Verdict: "fits", Free: "30000.00", Committed: "90000.00"}}
			v.FitAlias = "main"
		case "BBB":
			v.Tier, v.Place = "check", 1
			v.Badges = []string{"cash_short"}
			v.Cash = []optiontierapi.CashCell{{Alias: "main", Verdict: "short", Free: "100.00", Committed: "0"}}
		default:
			v.Tier, v.Grey = "grey", "no_bid"
			v.Place = 1
		}
		res.Verdicts = append(res.Verdicts, v)
	}
	// fits, check, then grey.
	for _, tier := range []string{"fits", "check", "grey"} {
		for _, v := range res.Verdicts {
			if v.Tier == tier {
				res.Order = append(res.Order, v.Key)
			}
		}
	}
	return res, nil
}

func input() optiontierapi.Input {
	return optiontierapi.Input{
		Day:    "2026-11-17",
		Params: optiontierapi.Params{DeltaMax: "0.3", ThinDelta: "0.15", MaxCashAgeDays: 3, DaysInYear: 365},
		Rows: []optiontierapi.Row{
			{Key: "c", Company: "CCC", Strike: "50.00"},
			{Key: "a", Company: "AAA", Strike: "90.00", Bid: "0.94", BuyPrice: "90.00"},
			{Key: "b", Company: "BBB", Strike: "290.00", Bid: "2.00", BuyPrice: "290.00"},
		},
		Accounts: []optiontierapi.AccountCash{{Alias: "main", Cash: "120000.00", Currency: "USD", CaptureDay: "2026-11-16", AgeDays: 1}},
	}
}

// TestTiererContractFake: a caller reads a green, an amber and a grey verdict from a fake Tierer.
func TestTiererContractFake(t *testing.T) {
	t.Parallel()
	var tr optiontierapi.Tierer = fakeTierer{}
	res, err := tr.Judge(input())
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	by := map[string]optiontierapi.Verdict{}
	for _, v := range res.Verdicts {
		by[v.Key] = v
	}
	checkRows(t, by)
	checkResult(t, res)
}

func checkRows(t *testing.T, by map[string]optiontierapi.Verdict) {
	t.Helper()
	if g := by["a"]; g.Tier != "fits" || g.Place != 1 || g.FitAlias != "main" || len(g.Cash) != 1 || g.Cash[0].Verdict != "fits" {
		t.Errorf("green row = %+v", g)
	}
	if a := by["b"]; a.Tier != "check" || !reflect.DeepEqual(a.Badges, []string{"cash_short"}) || a.FitAlias != "" || a.Cash[0].Verdict != "short" {
		t.Errorf("amber row = %+v", a)
	}
	if g := by["c"]; g.Tier != "grey" || g.Grey != "no_bid" {
		t.Errorf("grey row = %+v", g)
	}
}

func checkResult(t *testing.T, res optiontierapi.Result) {
	t.Helper()
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(res.Order, want) {
		t.Errorf("Order = %v, want %v", res.Order, want)
	}
	if !reflect.DeepEqual(res.Common, []string{"earnings_unconfirmed"}) {
		t.Errorf("Common = %v", res.Common)
	}
	if len(res.Kept) != 1 || res.Kept[0].Code != "no_open_interest" || res.Kept[0].N != 1 || res.Kept[0].Every {
		t.Errorf("Kept = %+v", res.Kept)
	}
	if !res.CashBuilt {
		t.Error("CashBuilt = false, want true")
	}
	if res.Params.DeltaMax != "0.3" {
		t.Errorf("Params echo = %+v", res.Params)
	}
}

// TestVerdictListsNonNil: the fake's answer for a row with no badge has empty, non-nil lists.
func TestVerdictListsNonNil(t *testing.T) {
	t.Parallel()
	in := input()
	in.Rows = in.Rows[:1]
	res, err := fakeTierer{}.Judge(in)
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	v := res.Verdicts[0]
	if v.Badges == nil || v.Infos == nil || v.Notes == nil {
		t.Fatalf("nil list in %+v", v)
	}
	if len(v.Badges)+len(v.Infos)+len(v.Notes) != 0 {
		t.Errorf("lists not empty: %+v", v)
	}
}
