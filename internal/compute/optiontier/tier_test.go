package optiontier

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

const (
	strikeBadInput = "7.77" // the fake evaluator answers ErrBadInput for it
	strikeBoom     = "6.66" // the fake evaluator answers another error for it
)

// fakeEvaluator answers the figures the real one defines, by decimal arithmetic, and counts its calls.
type fakeEvaluator struct{ calls int }

var errBoom = errors.New("boom")

func (f *fakeEvaluator) Evaluate(r optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
	f.calls++
	switch r.Strike {
	case strikeBadInput:
		return optionevidenceapi.Evidence{}, optionevidenceapi.ErrBadInput
	case strikeBoom:
		return optionevidenceapi.Evidence{}, errBoom
	}
	strike, _ := decimal.NewFromString(r.Strike)
	bid, _ := decimal.NewFromString(r.Bid)
	be := strike.Sub(bid)
	ev := optionevidenceapi.Evidence{BreakEven: be.String()}
	if buy, err := decimal.NewFromString(r.BuyPrice); err == nil && buy.IsPositive() {
		below := buy.Sub(be)
		ev.BelowBuy = below.String()
		ev.BelowBuyPct = below.Div(buy).Mul(decimal.NewFromInt(100)).StringFixed(4)
	}
	if r.IV == "" {
		ev.MoveWhy = "no_iv"
	} else {
		ev.BreakEvenInSigmas = "0.9000"
	}
	return ev, nil
}

type fixture struct {
	Params optiontierapi.Params
	Base   optiontierapi.Row
	ITM    optiontierapi.Row
}

func load(t *testing.T) fixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/tiers.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func loadAccounts(t *testing.T, names ...string) []optiontierapi.AccountCash {
	t.Helper()
	raw, err := os.ReadFile("testdata/cash.json")
	if err != nil {
		t.Fatal(err)
	}
	all := map[string]optiontierapi.AccountCash{}
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	out := []optiontierapi.AccountCash{}
	for _, n := range names {
		a, ok := all[n]
		if !ok {
			t.Fatalf("no account %q in testdata/cash.json", n)
		}
		out = append(out, a)
	}
	return out
}

// judge runs Judge over rows with the accounts named.
func judge(t *testing.T, f fixture, rows []optiontierapi.Row, accts ...string) optiontierapi.Result {
	t.Helper()
	in := optiontierapi.Input{Day: "2026-11-17", Params: f.Params, Rows: rows, Accounts: loadAccounts(t, accts...)}
	res, err := New(&fakeEvaluator{}).Judge(in)
	if err != nil {
		t.Fatalf("Judge: %v", err)
	}
	return res
}

func one(t *testing.T, f fixture, mod func(*optiontierapi.Row), accts ...string) optiontierapi.Verdict {
	t.Helper()
	r := f.Base
	if mod != nil {
		mod(&r)
	}
	return judge(t, f, []optiontierapi.Row{r}, accts...).Verdicts[0]
}

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func TestDefaultParamsAreTrialValues(t *testing.T) {
	t.Parallel()
	want := optiontierapi.Params{DeltaMax: "0.3", ThinDelta: "0.15", MaxCashAgeDays: 3, DaysInYear: 365,
		BandOTMMinPct: "5", BandOTMMaxPct: "15", BandDeltaMin: "0.15", PerCompany: 3}
	if got := DefaultParams(); got != want {
		t.Errorf("DefaultParams = %+v, want %+v", got, want)
	}
}

func TestFitsAllConditions(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, nil, "main")
	if v.Tier != "fits" || v.Place != 1 || len(v.Badges) != 0 || v.FitAlias != "main" {
		t.Errorf("verdict = %+v", v)
	}
}

func TestCheckNeedsHardThree(t *testing.T) {
	t.Parallel()
	f := load(t)
	// A look-at badge with the hard three holding is check.
	v := one(t, f, func(r *optiontierapi.Row) { r.QuoteReason = "wide_spread" }, "main")
	if v.Tier != "check" || !has(v.Badges, "wide_quote") {
		t.Errorf("hard three hold: %+v", v)
	}
	// The same badge with a hard condition failing stays grey.
	v = one(t, f, func(r *optiontierapi.Row) { r.QuoteReason = "wide_spread"; r.BuyPrice = "" }, "main")
	if v.Tier != "grey" || v.Grey != "no_buy_price" {
		t.Errorf("hard condition fails: %+v", v)
	}
}

func TestDeltaAboveBoundIsCheck(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { r.Delta = "-0.34" }, "main")
	if v.Tier != "check" || !has(v.Badges, "delta_above_bound") {
		t.Errorf("verdict = %+v", v)
	}
}

func TestDeltaBoundIsParam(t *testing.T) {
	t.Parallel()
	f := load(t)
	f.Params.DeltaMax = "0.4"
	v := one(t, f, func(r *optiontierapi.Row) { r.Delta = "-0.34" }, "main")
	if v.Tier != "fits" || has(v.Badges, "delta_above_bound") {
		t.Errorf("verdict = %+v", v)
	}
}

func TestThinPremiumStaysGreen(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { r.Delta = "-0.11" }, "main")
	if v.Tier != "fits" || len(v.Notes) != 1 || v.Notes[0] != "thin_premium" {
		t.Errorf("verdict = %+v", v)
	}
}

func TestDeltaUnknownIsCheck(t *testing.T) {
	t.Parallel()
	f := load(t)
	v := one(t, f, func(r *optiontierapi.Row) { r.Delta = "" }, "main")
	if v.Tier != "check" || !has(v.Badges, "delta_unknown") {
		t.Errorf("verdict = %+v", v)
	}
}

func TestAtTheMoneyIsNotItm(t *testing.T) {
	t.Parallel()
	f := load(t)
	atm := one(t, f, func(r *optiontierapi.Row) { r.Strike = "96.50"; r.BuyPrice = "100.00" }, "main")
	if atm.Tier == "grey" {
		t.Errorf("strike equal to spot: %+v", atm)
	}
	itm := one(t, f, func(r *optiontierapi.Row) { r.Strike = "96.51"; r.BuyPrice = "100.00" }, "main")
	if itm.Tier != "grey" || itm.Grey != "in_the_money" {
		t.Errorf("strike above spot: %+v", itm)
	}
}

// TestThresholdBoundariesAreTheIssueNumbers: the default delta bound 0.3, the thin line 0.15 and the cash
// age 3 days are inclusive/exclusive exactly where the issue puts them.
func TestThresholdBoundariesAreTheIssueNumbers(t *testing.T) {
	t.Parallel()
	f := load(t)
	f.Params = DefaultParams()
	delta := func(d string) optiontierapi.Verdict {
		return one(t, f, func(r *optiontierapi.Row) { r.Delta = d }, "main")
	}
	if v := delta("-0.30"); v.Tier != "fits" {
		t.Errorf("|delta| 0.30 = bound: %+v", v)
	}
	if v := delta("-0.31"); v.Tier != "check" {
		t.Errorf("|delta| 0.31: %+v", v)
	}
	if v := delta("-0.15"); len(v.Notes) != 0 {
		t.Errorf("|delta| 0.15 = thin line has note: %+v", v.Notes)
	}
	if v := delta("-0.1499"); len(v.Notes) != 1 {
		t.Errorf("|delta| 0.1499 lacks the note: %+v", v.Notes)
	}
	age := func(days int) string {
		in := optiontierapi.Input{Params: f.Params, Rows: []optiontierapi.Row{f.Base},
			Accounts: []optiontierapi.AccountCash{{Alias: "x", Cash: "50000", Currency: "USD", AgeDays: days}}}
		res, err := New(&fakeEvaluator{}).Judge(in)
		if err != nil {
			t.Fatal(err)
		}
		return res.Verdicts[0].Cash[0].Why
	}
	if got := age(3); got != "" {
		t.Errorf("age 3 days: why %q", got)
	}
	if got := age(4); got != "stale_cash" {
		t.Errorf("age 4 days: why %q", got)
	}
}
