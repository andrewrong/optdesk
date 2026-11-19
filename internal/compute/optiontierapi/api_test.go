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

type blocksTierer struct{}

func (blocksTierer) Judge(in optiontierapi.Input) (optiontierapi.Result, error) {
	return optiontierapi.Result{
		Params: in.Params,
		Blocks: []optiontierapi.CompanyBlock{
			{
				Company: "AAA", State: optiontierapi.StateSellable, Best: "21.5000",
				NFit: 1, NCheck: 1, NOpen: 1,
				Shown:  []string{"a1", "a2"},
				Folded: []string{"a3", "a4"},
				Why:    []optiontierapi.GreyCount{{Code: optiontierapi.FoldOTMLow, N: 1}, {Code: optiontierapi.FoldOverCap, N: 1}},
			},
			{
				Company: "CCC", State: optiontierapi.StateAboveBuy,
				Shown:  []string{},
				Folded: []string{"c1"},
				Why:    []optiontierapi.GreyCount{{Code: "above_buy_price", N: 1}},
			},
		},
	}, nil
}

// TestResultBlocksContract: a caller reads company blocks from a fake Tierer; the state and fold codes are distinct.
func TestResultBlocksContract(t *testing.T) {
	t.Parallel()
	var tr optiontierapi.Tierer = blocksTierer{}
	in := input()
	in.Params.BandOTMMinPct, in.Params.BandOTMMaxPct, in.Params.BandDeltaMin, in.Params.PerCompany = "5", "15", "0.15", 3
	res, err := tr.Judge(in)
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	if res.Params.PerCompany != 3 || res.Params.BandOTMMinPct != "5" || res.Params.BandOTMMaxPct != "15" || res.Params.BandDeltaMin != "0.15" {
		t.Errorf("Params echo = %+v", res.Params)
	}
	if len(res.Blocks) != 2 {
		t.Fatalf("Blocks = %d, want 2", len(res.Blocks))
	}
	a, c := res.Blocks[0], res.Blocks[1]
	if a.State != optiontierapi.StateSellable || a.NFit != 1 || a.NCheck != 1 || a.NOpen != 1 || a.Best != "21.5000" {
		t.Errorf("block AAA = %+v", a)
	}
	if !reflect.DeepEqual(a.Shown, []string{"a1", "a2"}) || !reflect.DeepEqual(a.Folded, []string{"a3", "a4"}) {
		t.Errorf("AAA rows: shown %v folded %v", a.Shown, a.Folded)
	}
	n := 0
	for _, w := range a.Why {
		n += w.N
	}
	if n != len(a.Folded) {
		t.Errorf("sum of Why = %d, want %d", n, len(a.Folded))
	}
	if c.State != optiontierapi.StateAboveBuy || len(c.Shown) != 0 || len(c.Folded) != 1 || c.Why[0].Code != "above_buy_price" {
		t.Errorf("block CCC = %+v", c)
	}
	distinct(t, "state", optiontierapi.StateSellable, optiontierapi.StateOutBand, optiontierapi.StateAboveBuy,
		optiontierapi.StateNoBuyPrice, optiontierapi.StateNoQuote, optiontierapi.StateBlocked)
	distinct(t, "fold", optiontierapi.FoldNoOpenInterest, optiontierapi.FoldOTMLow, optiontierapi.FoldOTMHigh,
		optiontierapi.FoldNoDelta, optiontierapi.FoldDeltaOut, optiontierapi.FoldOverCap)
}

func distinct(t *testing.T, kind string, codes ...string) {
	t.Helper()
	seen := map[string]bool{}
	for _, c := range codes {
		if c == "" || seen[c] {
			t.Errorf("%s code %q is empty or repeated", kind, c)
		}
		seen[c] = true
	}
}
