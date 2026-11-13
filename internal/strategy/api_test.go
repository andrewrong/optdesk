package strategy_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
)

// fake echoes the snapshot into evaluations so a caller can read every field back.
type fake struct{ p strategy.Params }

func (f fake) ID() string              { return "fake" }
func (f fake) Version() string         { return "fake_v1" }
func (f fake) Params() strategy.Params { return f.p }

func (f fake) Evaluate(s strategy.Snapshot) []strategy.Evaluation {
	facts := map[string]string{
		"run_day": string(s.RunDay), "chain_day": string(s.ChainDay), "sessions": string(s.SessionDays[0]),
		"issuer": s.Issuer, "underlying": s.Underlying, "spot": s.Spot, "quotes": s.Quotes[0].InstrumentID,
		"buy": s.BuyPrice, "sale": s.SalePrice, "earnings": s.Earnings.State, "dividend": s.Dividend.State,
		"rate": s.Rate, "rate_day": string(s.RateDay), "account": s.Accounts[0].Alias,
	}
	for k, v := range s.Context {
		facts[k] = v
	}
	leg := strategy.Leg{InstrumentID: "7", Side: "short", Right: "P", Strike: "50", Expiry: "2026-12-18", Multiplier: "100", Quantity: 1}
	return []strategy.Evaluation{{
		Key: "7", Legs: []strategy.Leg{leg},
		Checks: []strategy.Check{{Name: "buy_price", Status: "block", Reason: "above_buy"}},
		Status: "blocked", Facts: facts, Rank: 0, RankInUnderlying: int(s.UnderlyingID),
	}}
}

func (f fake) Rank(in []strategy.Evaluation) []strategy.Evaluation {
	out := append([]strategy.Evaluation(nil), in...)
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func (f fake) Manage(p strategy.Position, s strategy.Snapshot) strategy.Action {
	if p.Expiry <= s.RunDay {
		return strategy.Action{Kind: "close", Reason: "expiry"}
	}
	return strategy.Action{Kind: "hold"}
}

func TestStrategyFromBothSides(t *testing.T) {
	t.Parallel()
	var s strategy.Strategy = fake{p: strategy.Params{Values: map[string]string{"k": "1"}, Hash: "abc"}}
	snap := strategy.Snapshot{
		RunDay: "2026-11-13", ChainDay: "2026-11-12", SessionDays: []contracts.Day{"2026-11-12"},
		Issuer: "iss", Underlying: "AAPL", UnderlyingID: 3, Spot: "190.5",
		Quotes:   []compute.OptionQuote{{InstrumentID: "q1"}},
		BuyPrice: "45", SalePrice: "60",
		Earnings: &compute.EarningsInfo{State: "listed"}, Dividend: &compute.DividendInfo{State: "present"},
		Rate: "0.04", RateDay: "2026-11-11", Accounts: []compute.AliasFunds{{Alias: "A"}},
		Context: map[string]string{"earnings_state": "listed"},
	}
	if s.ID() != "fake" || s.Version() != "fake_v1" || s.Params().Hash != "abc" || s.Params().Values["k"] != "1" {
		t.Fatalf("identity: %q %q %+v", s.ID(), s.Version(), s.Params())
	}
	got := s.Evaluate(snap)
	wantFacts := map[string]string{
		"run_day": "2026-11-13", "chain_day": "2026-11-12", "sessions": "2026-11-12", "issuer": "iss",
		"underlying": "AAPL", "spot": "190.5", "quotes": "q1", "buy": "45", "sale": "60", "earnings": "listed",
		"dividend": "present", "rate": "0.04", "rate_day": "2026-11-11", "account": "A", "earnings_state": "listed",
	}
	wantLeg := strategy.Leg{InstrumentID: "7", Side: "short", Right: "P", Strike: "50", Expiry: "2026-12-18", Multiplier: "100", Quantity: 1}
	if len(got) != 1 {
		t.Fatalf("got %d evaluations", len(got))
	}
	e := got[0]
	if e.Key != "7" || e.Status != "blocked" || e.Rank != 0 || e.RankInUnderlying != 3 ||
		!reflect.DeepEqual(e.Legs, []strategy.Leg{wantLeg}) ||
		!reflect.DeepEqual(e.Checks, []strategy.Check{{Name: "buy_price", Status: "block", Reason: "above_buy"}}) ||
		!reflect.DeepEqual(e.Facts, wantFacts) {
		t.Fatalf("evaluation = %+v", e)
	}
	if r := s.Rank(got); r[0].Rank != 1 || got[0].Rank != 0 {
		t.Fatalf("rank = %+v, input = %+v", r, got)
	}
	pos := strategy.Position{
		Contract: "c", Right: "P", Strike: "50", Expiry: "2026-11-13", OpenedOn: "2026-10-01",
		OpenPrice: "1.2", Quantity: "-1", Multiplier: "100",
	}
	if a := s.Manage(pos, snap); a != (strategy.Action{Kind: "close", Reason: "expiry"}) {
		t.Fatalf("manage = %+v", a)
	}
	pos.Expiry = "2026-12-18"
	if a := s.Manage(pos, snap); a != (strategy.Action{Kind: "hold"}) {
		t.Fatalf("manage = %+v", a)
	}
}

func TestStrategyPackageImportsNoStoreJobsPages(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		n++
		if _, err := os.Stat(f); err != nil {
			t.Fatal(err)
		}
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p := strings.Trim(im.Path.Value, `"`)
			for _, bad := range []string{"internal/store", "internal/jobs", "internal/pages"} {
				if strings.Contains(p, bad) {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no non-test file found")
	}
}
