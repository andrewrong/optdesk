package option_candidates_page

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionfigapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeTierer records its inputs and answers with res, or with fn of the input.
type fakeTierer struct {
	mu  sync.Mutex
	in  []optiontierapi.Input
	res optiontierapi.Result
	fn  func(optiontierapi.Input) optiontierapi.Result
	err error
}

func (f *fakeTierer) Judge(in optiontierapi.Input) (optiontierapi.Result, error) {
	f.mu.Lock()
	f.in = append(f.in, in)
	f.mu.Unlock()
	if f.err != nil {
		return optiontierapi.Result{}, f.err
	}
	if f.fn != nil {
		return f.fn(in), nil
	}
	return f.res, nil
}

func (f *fakeTierer) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.in)
}

// tierFigs records the inputs it is asked about.
type tierFigs struct {
	mu  sync.Mutex
	got []optionfigapi.PutFigureInput
}

func (f *tierFigs) Put(in optionfigapi.PutFigureInput) optionfigapi.PutFigures {
	f.mu.Lock()
	f.got = append(f.got, in)
	f.mu.Unlock()
	return optionfigapi.PutFigures{ItmPercent: "21.8%", ItmKey: "2180", ExcessPercent: "8.0%", ExcessKey: "800"}
}

// fakeAccount holds values and positions by "alias/day" and records the reads.
type fakeAccount struct {
	mu     sync.Mutex
	values map[string][]store.AccountValue
	pos    map[string][]store.Position
	reads  []string
	err    error
}

func (f *fakeAccount) Values(_ context.Context, alias, day string) ([]store.AccountValue, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, "values "+alias+"/"+day)
	return f.values[alias+"/"+day], f.err
}

func (f *fakeAccount) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads = append(f.reads, "positions "+alias+"/"+day)
	return f.pos[alias+"/"+day], f.err
}

func (f *fakeAccount) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

func cashAt(alias, day, value string) map[string][]store.AccountValue {
	return map[string][]store.AccountValue{alias + "/" + day: {{Alias: alias, SessionDay: day, Tag: "TotalCashValue", Value: value, Currency: "USD"}}}
}

// tierPut is a stored short put of AAA (underlying 7) with the given strike and bid.
func tierPut(id int, status, strike, bid string) store.Candidate {
	c := okPut(id)
	c.Status, c.Strike, c.BidEod = status, strike, bid
	c.Delta, c.Iv = strp("-0.19"), strp("0.305")
	return c
}

func keyOf(rows []store.Candidate, i int) string { return tierKey(rows[i], i) }

// fitsVerdict is a verdict the page cannot produce itself: its numbers are not those of the stored row.
func fitsVerdict(key string) optiontierapi.Verdict {
	return optiontierapi.Verdict{
		Key: key, Tier: "fits", Place: 1, Badges: []string{}, Infos: []string{}, Notes: []string{},
		BreakEven: "84.58", BelowBuy: "5.42", MarginPct: "6.0111", BelowSpotPct: "12.3834", Moves: "1.2900",
		PerContract: "42.00", Intrinsic: "0", TimeValue: "0.42", YieldPct: "5.8000",
		Cash:     []optiontierapi.CashCell{{Alias: "A", Verdict: "fits", Free: "120000.00", Committed: "0.00"}},
		FitAlias: "A",
	}
}

func checkVerdict(key string, badges ...string) optiontierapi.Verdict {
	v := fitsVerdict(key)
	v.Tier, v.Badges = "check", badges
	return v
}

func greyVerdict(key, grey string) optiontierapi.Verdict {
	return optiontierapi.Verdict{Key: key, Tier: "grey", Grey: grey, Badges: []string{}, Infos: []string{}, Notes: []string{},
		BreakEven: "96.50", Intrinsic: "3.50", TimeValue: "1.90", PerContract: "540.00", YieldPct: "22.4000"}
}

// resultOf answers with the verdicts in the given order as the display order.
func resultOf(vs ...optiontierapi.Verdict) optiontierapi.Result {
	r := optiontierapi.Result{
		Verdicts: vs, CashBuilt: true,
		Params:    optiontierapi.Params{DeltaMax: "0.3", ThinDelta: "0.15", MaxCashAgeDays: 3, DaysInYear: 365},
		Companies: []optiontierapi.CompanyCount{{Company: "7"}},
	}
	for _, v := range vs {
		r.Order = append(r.Order, v.Key)
		switch v.Tier {
		case "fits":
			r.Companies[0].Fits++
		case "check":
			r.Companies[0].Check++
		default:
			r.Companies[0].Grey++
		}
	}
	return r
}

func tierConfig(rows []store.Candidate, t optiontierapi.Tierer) Config {
	c := evConfig(rows, &fakeEval{})
	c.Tiers = t
	c.Names = &namesFake{names: map[int]string{7: "AAA", 8: "BBB"}}
	return c
}

func buildTiersOf(t *testing.T, c Config) *tiersView {
	t.Helper()
	b, err := Build(context.Background(), c, nil, buildDay, testNow)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Tiers *tiersView `json:"short_put_tiers"`
	}
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	return p.Tiers
}

func tierByID(v *tiersView, id string) *tierView {
	for i := range v.Tiers {
		if v.Tiers[i].ID == id {
			return &v.Tiers[i]
		}
	}
	return nil
}

func tierIDs(v *tiersView) []string {
	var out []string
	for _, t := range v.Tiers {
		out = append(out, t.ID)
	}
	return out
}

func fieldByID(r tierRow, id string) *fieldView {
	for i := range r.Fields {
		if r.Fields[i].ID == id {
			return &r.Fields[i]
		}
	}
	return nil
}

// tierScenario is the page of the invented numbers of the tier golden files: AAA spot 96.50, buy price
// 90.00, BBB spot 41.20 with no buy price, cash of account A $120,000.00.
func tierScenario(name string) (Config, *fakeTierer) {
	a0, a1, a2 := tierPut(1, "ok", "85.00", "0.42"), tierPut(2, "ok", "90.00", "0.94"), tierPut(3, "blocked", "87.50", "0.60")
	bbb := tierPut(5, "ok", "40.00", "0.50")
	bbb.UnderlyingID, bbb.BuyPrice, bbb.Spot = 8, nil, strp("41.20")
	k := func(rows []store.Candidate, i int) string { return keyOf(rows, i) }
	var rows []store.Candidate
	var res optiontierapi.Result
	switch name {
	case "tiers_ok":
		itm := tierPut(4, "ok", "100.00", "5.40")
		rows = []store.Candidate{a0, a1, a2, itm, bbb}
		thin := fitsVerdict(k(rows, 0))
		thin.Notes = []string{"thin_premium"}
		amber := checkVerdict(k(rows, 2), "no_open_interest")
		amber.MarginPct, amber.BreakEven, amber.Moves, amber.PerContract, amber.YieldPct = "3.4000", "86.90", "1.1000", "60.00", "8.1000"
		noBuy := greyVerdict(k(rows, 4), "no_buy_price")
		noBuy.MarginPct, noBuy.Why = "", map[string]string{"margin": "no_buy_price"}
		res = resultOf(thin, fitsVerdict(k(rows, 1)), amber, greyVerdict(k(rows, 3), "in_the_money"), noBuy)
		res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 2, Check: 1, Grey: 1}, {Company: "8", Grey: 1}}
		res.Kept = []optiontierapi.Kept{{Code: "no_buy_price", N: 1}, {Code: "in_the_money", N: 1}, {Code: "no_open_interest", N: 1}}
		res.Common = []string{"earnings_unconfirmed"}
	default:
		noIV := a0
		noIV.Iv = nil
		rows = []store.Candidate{noIV, bbb}
		fit := fitsVerdict(k(rows, 0))
		fit.Moves, fit.Why, fit.Cash, fit.FitAlias = "", map[string]string{"moves": "no_iv"}, nil, ""
		noBuy := greyVerdict(k(rows, 1), "no_buy_price")
		noBuy.MarginPct, noBuy.Why = "", map[string]string{"margin": "no_buy_price"}
		res = resultOf(fit, noBuy)
		res.Companies = []optiontierapi.CompanyCount{{Company: "7", Fits: 1}, {Company: "8", Grey: 1}}
		res.CashBuilt = false
	}
	f := &fakeTierer{res: res}
	c := tierConfig(rows, f)
	c.Figures = &tierFigs{}
	c.ATMFor = (&atmCalls{iv: "0.294", strike: "95.00"}).fn
	c.Readings = allReadings("2026-11-09")
	if name == "tiers_ok" {
		c.Account, c.Aliases = &fakeAccount{values: cashAt("A", "2026-11-09", "120000.00")}, []string{"A"}
	}
	return c, f
}
