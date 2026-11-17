package optionevidence_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optionevidence"
	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
)

type fixedValuer struct {
	fair string
	err  error
}

func (f fixedValuer) Value(right, spot, strike, years, rate, vol string) (string, error) {
	return f.fair, f.err
}

func eval(t *testing.T, fair string, r optionevidenceapi.Row) optionevidenceapi.Evidence {
	t.Helper()
	ev, err := optionevidence.New(fixedValuer{fair: fair}, optionevidence.DefaultParams()).Evaluate(r)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return ev
}

// aaa is a made-up put candidate of company AAA.
func aaa() optionevidenceapi.Row {
	return optionevidenceapi.Row{
		ChainDay: "2026-11-17", Expiry: "2026-12-18", DTE: 31,
		Spot: "96.50", Strike: "90.00", Bid: "0.94", BuyPrice: "90.00", IV: "0.312",
		Delta: "-0.19", Multiplier: "100", RatePercent: "4", Forecast: "0.25", ForecastErr: "0.05",
		ATMIV: "0.300", ATMStrike: "96.00",
	}
}

func same(t *testing.T, name, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	g, e1 := decimal.NewFromString(got)
	w, e2 := decimal.NewFromString(want)
	if e1 != nil || e2 != nil || !g.Equal(w) {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

// TestOneSigmaAndInSigmas: 1-sigma of 100/0.20/365 days is 20; of the AAA row 8.7744; the break-even is 0.8479 sigmas.
func TestOneSigmaAndInSigmas(t *testing.T) {
	t.Parallel()
	ev := eval(t, "0.74", optionevidenceapi.Row{Spot: "100", Strike: "90", Bid: "1", IV: "0.20", DTE: 365})
	same(t, "OneSigma", ev.OneSigma, "20")
	ev = eval(t, "0.74", aaa())
	same(t, "OneSigma", ev.OneSigma, "8.7744")
	same(t, "BreakEvenInSigmas", ev.BreakEvenInSigmas, "0.8479")
}

// TestBelowBuy: the golden cases of below buy (the figure the page sorts companies by).
func TestBelowBuy(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string
		Row  optionevidenceapi.Row
		Want map[string]string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 3 {
		t.Fatalf("%d cases", len(cases))
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			ev := eval(t, "0.74", c.Row)
			got := map[string]string{
				"BreakEven": ev.BreakEven, "BelowBuy": ev.BelowBuy, "BelowBuyPct": ev.BelowBuyPct,
				"BelowBuyWhy": ev.BelowBuyWhy, "OneSigma": ev.OneSigma,
				"BreakEvenInSigmas": ev.BreakEvenInSigmas, "MoveWhy": ev.MoveWhy,
			}
			for k, w := range c.Want {
				same(t, k, got[k], w)
			}
		})
	}
}

// TestEdgeAtBid: the edge is the bid less the fixed fair price; zero or negative sets no_edge_at_bid.
func TestEdgeAtBid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		fair, share, contract string
		label                 bool
	}{
		{"0.74", "0.20", "20.00", false},
		{"1.10", "-0.16", "-16.00", true},
		{"0.94", "0", "0.00", true},
	}
	for _, c := range cases {
		ev := eval(t, c.fair, aaa())
		same(t, "EdgePerShare", ev.EdgePerShare, c.share)
		same(t, "EdgePerContract", ev.EdgePerContract, c.contract)
		has := false
		for _, l := range ev.Labels {
			has = has || l == "no_edge_at_bid"
		}
		if has != c.label {
			t.Errorf("fair %s: label = %v, want %v", c.fair, has, c.label)
		}
	}
}

// TestEmptyFiguresHaveReasons: each empty figure carries its Why, and a cause leaves the other figures given.
func TestEmptyFiguresHaveReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		mut    func(*optionevidenceapi.Row)
		failer bool
		edge   string
		move   string
	}{
		{"earnings", func(r *optionevidenceapi.Row) { r.EarningsBeforeExpiry = true }, false, "earnings_before_expiry", ""},
		{"no forecast", func(r *optionevidenceapi.Row) { r.Forecast = "" }, false, "no_forecast", ""},
		{"no rate", func(r *optionevidenceapi.Row) { r.RatePercent = "" }, false, "no_rate", ""},
		{"valuer error", func(*optionevidenceapi.Row) {}, true, "no_fair_price", ""},
		{"no iv", func(r *optionevidenceapi.Row) { r.IV = "" }, false, "", "no_iv"},
	}
	for _, c := range cases {
		r := aaa()
		c.mut(&r)
		v := fixedValuer{fair: "0.74"}
		if c.failer {
			v.err = errors.New("boom")
		}
		ev, err := optionevidence.New(v, optionevidence.DefaultParams()).Evaluate(r)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if ev.EdgeWhy != c.edge || ev.MoveWhy != c.move {
			t.Errorf("%s: EdgeWhy %q MoveWhy %q", c.name, ev.EdgeWhy, ev.MoveWhy)
		}
		if (ev.EdgePerShare == "") != (ev.EdgeWhy != "") || (ev.OneSigma == "") != (ev.MoveWhy != "") {
			t.Errorf("%s: an empty figure lacks its Why, or a Why sits beside a figure: %+v", c.name, ev)
		}
		if (ev.BelowBuy == "") != (ev.BelowBuyWhy != "") || (ev.Skew == "") != (ev.SkewWhy != "") {
			t.Errorf("%s: below buy or skew: %+v", c.name, ev)
		}
		if c.name == "earnings" && ev.OneSigma == "" {
			t.Error("the 1-sigma is still given before earnings")
		}
	}
}

// TestSkew: contract IV less ATM IV, no_atm without an ATM row, forecast error unchanged.
func TestSkew(t *testing.T) {
	t.Parallel()
	ev := eval(t, "0.74", aaa())
	same(t, "Skew", ev.Skew, "0.012")
	if ev.ForecastErr != "0.05" {
		t.Errorf("ForecastErr = %q", ev.ForecastErr)
	}
	r := aaa()
	r.ATMIV, r.ATMStrike = "", ""
	ev = eval(t, "0.74", r)
	if ev.Skew != "" || ev.SkewWhy != "no_atm" {
		t.Errorf("Skew %q Why %q", ev.Skew, ev.SkewWhy)
	}
}

// TestBadInput: an unusable spot, strike, bid or DTE is ErrBadInput.
func TestBadInput(t *testing.T) {
	t.Parallel()
	for _, mut := range []func(*optionevidenceapi.Row){
		func(r *optionevidenceapi.Row) { r.Spot = "x" },
		func(r *optionevidenceapi.Row) { r.Strike = "0" },
		func(r *optionevidenceapi.Row) { r.Bid = "" },
		func(r *optionevidenceapi.Row) { r.DTE = -1 },
	} {
		r := aaa()
		mut(&r)
		if _, err := optionevidence.New(nil, optionevidence.DefaultParams()).Evaluate(r); !errors.Is(err, optionevidenceapi.ErrBadInput) {
			t.Errorf("err = %v", err)
		}
	}
}

// A row expiring on its chain day has an IV; the move must not be reported as "no_iv" beside a value.
func TestZeroDTEMoveHasNoFalseReason(t *testing.T) {
	t.Parallel()
	r := aaa()
	r.DTE, r.Expiry = 0, r.ChainDay
	ev := eval(t, "0.74", r)
	if ev.MoveWhy == "no_iv" {
		t.Errorf("MoveWhy = no_iv although IV is %s", r.IV)
	}
	if ev.OneSigma != "" && ev.MoveWhy != "" {
		t.Errorf("OneSigma %q has a Why %q beside it", ev.OneSigma, ev.MoveWhy)
	}
	if ev.OneSigma == "" && ev.MoveWhy == "" {
		t.Error("an empty 1-sigma has no Why")
	}
}
