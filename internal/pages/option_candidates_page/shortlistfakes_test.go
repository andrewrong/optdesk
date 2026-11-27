package option_candidates_page

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeShortlister records its inputs and answers with res, or with err.
type fakeShortlister struct {
	mu  sync.Mutex
	in  []optionshortlistapi.Input
	res optionshortlistapi.Result
	err error
}

var _ optionshortlistapi.Shortlister = (*fakeShortlister)(nil)

func (f *fakeShortlister) Select(in optionshortlistapi.Input) (optionshortlistapi.Result, error) {
	f.mu.Lock()
	f.in = append(f.in, in)
	f.mu.Unlock()
	return f.res, f.err
}

func (f *fakeShortlister) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.in)
}

var testBounds = optionshortlistapi.Bounds{OTMMinPct: "5", OTMMaxPct: "15", DeltaMin: "0.15", DeltaMax: "0.30", PerUnderlying: 3}

// shortRows are three stored puts: two of AAA (underlying 7, buy price 90.00) and one of BBB (underlying 8, no
// buy price). Their instrument ids equal their positions, so each key is tier-short_put-N-N.
func shortRows() []store.Candidate {
	a0, a1 := tierPut(0, "ok", "85.00", "0.42"), tierPut(1, "ok", "90.00", "0.94")
	a0.AnnualizedYield, a1.AnnualizedYield = new("0.123456"), new("0.2")
	a1.Marks = `["buy_from_valuation"]` // the run priced this row from the valuation
	b := tierPut(2, "blocked", "40.00", "0.50")
	b.UnderlyingID, b.BuyPrice, b.Spot, b.AnnualizedYield = 8, nil, new("41.20"), new("0.15")
	return []store.Candidate{a0, a1, b}
}

// shortResult is a result the page cannot produce itself: its order and its numbers are the fake's.
func shortResult(rows []store.Candidate) optionshortlistapi.Result {
	return optionshortlistapi.Result{
		Groups: []optionshortlistapi.Group{
			{Company: "8", Best: "0.15", Rows: []optionshortlistapi.Placed{{Key: keyOf(rows, 2), OTM: "2.9126", AbsDelta: "0.19", Liquidity: "wide_spread"}}},
			{Company: "7", Best: "0.2", Rows: []optionshortlistapi.Placed{
				{Key: keyOf(rows, 1), OTM: "6.7358", AbsDelta: "0.19"}, {Key: keyOf(rows, 0), OTM: "11.9171", AbsDelta: "0.19"}}},
		},
		Dropped: []optionshortlistapi.Drop{{Code: "no_bid", N: 2}, {Code: "no_spot", N: 0}, {Code: "otm_low", N: 3}, {Code: "over_cap", N: 1}},
		Scanned: 9, Shown: 3, StaleChain: 2, DTEMin: "21", DTEMax: "45", Bounds: testBounds,
	}
}

// shortConfig is a page of rows with the fake shortlister s and, when judge is true, a tier judge that
// answers fitsVerdict for every row.
func shortConfig(rows []store.Candidate, s optionshortlistapi.Shortlister, judge bool) Config {
	var t *fakeTierer
	c := evConfig(rows, &fakeEval{})
	c.Names = &namesFake{names: map[int]string{7: "AAA", 8: "BBB"}}
	c.Shortlist = s
	if judge {
		vs := make([]optiontierapi.Verdict, len(rows))
		for i := range rows {
			vs[i] = fitsVerdict(keyOf(rows, i))
		}
		t = &fakeTierer{res: resultOf(vs...)}
		c.Tiers = t
		c.Figures = &tierFigs{}
	}
	return c
}

func buildShort(t *testing.T, c Config) []byte {
	t.Helper()
	b, err := Build(context.Background(), c, nil, buildDay, testNow)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func listOf(t *testing.T, c Config) *shortListView {
	t.Helper()
	var p struct {
		List *shortListView `json:"short_put_list"`
	}
	if err := json.Unmarshal(buildShort(t, c), &p); err != nil {
		t.Fatal(err)
	}
	return p.List
}

// shortScenario is the page of the golden files: "shortlist_ok" has the judge and the result of shortResult;
// "shortlist_gaps" has no judge, a row with no delta and a row with no buy price.
func shortScenario(name string) Config {
	rows := shortRows()
	if name == "shortlist_ok" {
		return shortConfig(rows, &fakeShortlister{res: shortResult(rows)}, true)
	}
	noDelta := rows[0]
	noDelta.Delta = nil
	b := rows[2]
	b.InstrumentID = 1
	rows = []store.Candidate{noDelta, b}
	res := optionshortlistapi.Result{
		Groups: []optionshortlistapi.Group{
			{Company: "7", Best: "0.123456", Rows: []optionshortlistapi.Placed{{Key: keyOf(rows, 0), OTM: "11.9171"}}},
			{Company: "8", Best: "0.15", Rows: []optionshortlistapi.Placed{{Key: keyOf(rows, 1), OTM: "2.9126", AbsDelta: "0.19"}}},
		},
		Scanned: 2, Shown: 2, DTEMin: "30", DTEMax: "30", Bounds: testBounds,
	}
	return shortConfig(rows, &fakeShortlister{res: res}, false)
}
