package optioncandidates

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

// threeChains is the fakes with three underlyings (ids 7, 8, 9: AAA, BBB, CCC), each with one chain row.
type threeChains struct{ *fakes }

var threeSymbols = map[int64]string{7: "AAA", 8: "BBB", 9: "CCC"}

func (t threeChains) OptionEod(ctx context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	if from != to {
		return t.fakes.OptionEod(ctx, id, from, to)
	}
	return []contracts.OptionEod{{InstrumentID: int(id) * 10, UnderlyingID: int(id), Bid: f64(1.5), Ask: f64(1.6),
		BidSize: f64(5), OpenInterest: f64(40), UnderlyingPrice: f64(100 + float64(id)), Status: 1, MarketDataType: 1, CaptureID: "c"}}, nil
}

func (t threeChains) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return []store.OptionEodLatest{{UnderlyingID: 7, SessionDay: "2026-11-09"}, {UnderlyingID: 8, SessionDay: "2026-11-09"},
		{UnderlyingID: 9, SessionDay: "2026-11-09"}}, nil
}

func (t threeChains) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: threeSymbols[id]})
	}
	return out, nil
}

// mixedRule answers ok, blocked and unknown put rows for each underlying, and a covered call and a guard row when
// asked for those kinds. Every figure depends on the underlying, so a mix-up of rows shows.
type mixedRule struct{}

func (mixedRule) Evaluate(in compute.CandidateInput, _ compute.CandidateParams) []compute.Candidate {
	base := map[string]int{"AAA": 100, "BBB": 200, "CCC": 300}[in.Underlying]
	put := func(n int, status string, yield string, rankIn int) compute.Candidate {
		c := compute.Candidate{
			Kind: compute.KindShortPut, Underlying: in.Underlying, InstrumentID: strconv.Itoa(base + n),
			Expiry: "2026-12-11", DTE: 25 + n, Right: "P", Strike: strconv.Itoa(80 + n), Multiplier: "100", Bid: "1.50",
			BreakEven: "78.50", Status: status, Marks: []string{"m" + strconv.Itoa(n)},
			Fits:  map[string]string{"B": "yes", "A": "no"},
			Gates: []compute.GateResult{{Gate: "buy_price", Status: compute.GatePass}},
		}
		switch status {
		case "ok":
			c.Ask, c.Spot, c.BuyPrice, c.AnnualizedYield, c.Delta, c.IV, c.Extrinsic = "1.60", in.Spot, in.BuyPrice, yield, "-0.20", "0.31", "1.10"
			c.RankInUnderlying = rankIn
		case "blocked":
			c.Ask, c.Spot = "1.60", in.Spot
			c.Gates = append(c.Gates, compute.GateResult{Gate: "spread", Status: compute.GateBlock, Reason: "spread_wide"})
		default:
			c.Gates = append(c.Gates, compute.GateResult{Gate: "earnings", Status: compute.GateUnknown, Reason: "no_event"})
		}
		return c
	}
	var out []compute.Candidate
	switch in.Underlying {
	case "AAA":
		out = append(out, put(1, "ok", "0.12", 2), put(2, "ok", "0.20", 1), put(3, "blocked", "", 0))
	case "BBB":
		out = append(out, put(1, "unknown", "", 0), put(2, "ok", "0.12", 1))
	default:
		out = append(out, put(1, "blocked", "", 0), put(2, "unknown", "", 0))
	}
	for _, k := range in.Kinds {
		if k == compute.KindCoveredCall {
			out = append(out, compute.Candidate{Kind: k, Underlying: in.Underlying, InstrumentID: strconv.Itoa(base + 50),
				Expiry: "2026-12-11", DTE: 25, Right: "C", Strike: "120", Multiplier: "100", Bid: "1.00", BreakEven: "99", Status: "ok",
				AnnualizedYield: "0.5", Gates: []compute.GateResult{}})
		}
		if k == compute.KindHeldCallGuard {
			out = append(out, compute.Candidate{Kind: k, Underlying: in.Underlying, InstrumentID: strconv.Itoa(base + 60),
				Expiry: "2026-12-11", DTE: 25, Right: "C", Strike: "120", Multiplier: "100", Bid: "1.00", BreakEven: "99", Status: "unknown",
				Gates: []compute.GateResult{}})
		}
	}
	return out
}

type fakeResults struct {
	mu    sync.Mutex
	rows  [][]strategyresult.Row
	err   error
	calls int
}

func (r *fakeResults) InsertResults(_ context.Context, rows []strategyresult.Row) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if r.err != nil {
		return 0, r.err
	}
	r.rows = append(r.rows, rows)
	return len(rows), nil
}

func (r *fakeResults) all() []strategyresult.Row {
	var out []strategyresult.Row
	for _, b := range r.rows {
		out = append(out, b...)
	}
	return out
}

func threeEntries() []jobs.WatchEntry {
	var out []jobs.WatchEntry
	for i, s := range []string{"AAA", "BBB", "CCC"} {
		out = append(out, jobs.WatchEntry{Issuer: fmt.Sprintf("cik:000000000%d", i+1), Market: "US", BuyPrice: "91.5",
			Contract: broker.Contract{Symbol: s, SecType: "STK"}})
	}
	return out
}

func putStrategy(t *testing.T) strategy.Strategy {
	t.Helper()
	s, err := shortput.New(mixedRule{}, compute.CandidateParams{MinDTE: 21, MaxDTE: 45}, shortput.Values{CloseMode: "expiry", EarlyKeep: "0.50", EarlyDTE: "21"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// shadowFixture builds the three-underlying fakes, the stores with the shadow, and the earnings of AAA.
func shadowFixture(t *testing.T, results *fakeResults) (*fakes, CandidateStores) {
	t.Helper()
	f := newFakes()
	f.earnings = map[string][]contracts.EarningsDate{"cik:0000000001": {
		{Issuer: "cik:0000000001", Status: "listed", ObservedOn: day("2026-11-05"), ReportDate: dayPtr("2026-12-01")},
	}}
	st := f.stores()
	st.OptionEod, st.Catalog, st.Names = threeChains{f}, threeChains{f}, threeChains{f}
	st.Strategy = putStrategy(t)
	if results != nil {
		st.Results = results
	}
	return f, st
}

func runNew(t *testing.T, st CandidateStores) contracts.JobResult {
	t.Helper()
	j, err := New(st, mixedRule{}, threeEntries(), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), runDay, nil)
}

func writtenRows(f *fakes) []store.Candidate {
	var out []store.Candidate
	for _, b := range f.written {
		out = append(out, b...)
	}
	return out
}

func optText2(m map[string]string, k string) *string {
	if v, ok := m[k]; ok {
		return &v
	}
	return nil
}

// oldColumns rebuilds every column of the old row from the fields of a result row alone.
func oldColumns(t *testing.T, r strategyresult.Row) store.Candidate {
	t.Helper()
	var raw []map[string]any
	if err := json.Unmarshal([]byte(r.Legs), &raw); err != nil || len(raw) != 1 {
		t.Fatalf("legs %q: %v", r.Legs, err)
	}
	leg := raw[0]
	var checks []map[string]string
	if err := json.Unmarshal([]byte(r.Checks), &checks); err != nil {
		t.Fatal(err)
	}
	var facts map[string]string
	if err := json.Unmarshal([]byte(r.Facts), &facts); err != nil {
		t.Fatal(err)
	}
	id, _ := strconv.Atoi(leg["instrument_id"].(string))
	mult, _ := strconv.Atoi(strings.SplitN(leg["multiplier"].(string), ".", 2)[0])
	dte, _ := strconv.Atoi(facts["dte"])
	gates := make([]gateJSON, 0, len(checks))
	for _, c := range checks {
		gates = append(gates, gateJSON{c["gate"], c["status"], c["reason"]})
	}
	g, _ := json.Marshal(gates)
	c := store.Candidate{
		SessionDay: r.SessionDay, RunID: r.RunID, Kind: "short_put", InstrumentID: id, RuleVersion: r.RuleVersion,
		Issuer: facts["issuer"], UnderlyingID: r.UnderlyingID, Expiry: contracts.Day(leg["expiry"].(string)), DTE: dte,
		Right: "put", Strike: leg["strike"].(string), Multiplier: mult, BidEod: facts["bid"], Ask: optText2(facts, "ask"),
		Spot: optText2(facts, "spot"), BuyPrice: optText2(facts, "buy_price"), SalePrice: optText2(facts, "sale_price"),
		BreakEven: facts["break_even"], AnnualizedYield: optText2(facts, "annualized_yield"), Delta: optText2(facts, "delta"),
		Iv: optText2(facts, "iv"), Status: r.Status, Gates: string(g), Marks: facts["marks"], Fits: facts["fits"],
		EarningsState: facts["earnings_state"], Extrinsic: optText2(facts, "extrinsic"), Rank: r.Rank,
		RankInUnderlying: r.RankInUnderlying, ComputedAt: r.ComputedAt, SettingsVersion: r.SettingsVersion,
	}
	if n, ok := facts["next_earnings"]; ok {
		d := contracts.Day(n)
		c.NextEarnings = &d
	}
	return c
}

func compareColumns(t *testing.T, got, want store.Candidate) {
	t.Helper()
	got.RunID, want.RunID, got.ComputedAt, want.ComputedAt = "", "", time.Time{}, time.Time{}
	gv, wv := reflect.ValueOf(got), reflect.ValueOf(want)
	for i := 0; i < gv.NumField(); i++ {
		if !reflect.DeepEqual(gv.Field(i).Interface(), wv.Field(i).Interface()) {
			t.Errorf("column %s of %d: got %v, old %v", gv.Type().Field(i).Name, want.InstrumentID, gv.Field(i).Interface(), wv.Field(i).Interface())
		}
	}
}

func putsOf(rows []store.Candidate) map[int]store.Candidate {
	out := map[int]store.Candidate{}
	for _, r := range rows {
		if r.Kind == "short_put" {
			out[r.InstrumentID] = r
		}
	}
	return out
}

func TestShadowRowsEqualOldRows(t *testing.T) {
	t.Parallel()
	results := &fakeResults{}
	f, st := shadowFixture(t, results)
	if res := runNew(t, st); res.Err != nil || res.Returned != 7 || res.Usable != 3 {
		t.Fatalf("result %+v", res)
	}
	old := putsOf(writtenRows(f))
	got := results.all()
	if results.calls != 1 || len(got) != len(old) || len(old) != 7 {
		t.Fatalf("%d calls, %d result rows, %d old rows", results.calls, len(got), len(old))
	}
	statuses := map[string]int{}
	for _, r := range got {
		statuses[r.Status]++
		o, ok := old[mustAtoi(t, r.Key)]
		if !ok {
			t.Fatalf("result key %s has no old row", r.Key)
		}
		if r.RunID != o.RunID || !r.ComputedAt.Equal(o.ComputedAt) || r.Strategy != "short_put" || r.RuleVersion != "put_v1" ||
			r.ParamHash != st.Strategy.Params().Hash || r.ParamHash == "" {
			t.Errorf("row %s: run %q at %v strategy %q version %q hash %q", r.Key, r.RunID, r.ComputedAt, r.Strategy, r.RuleVersion, r.ParamHash)
		}
		compareColumns(t, oldColumns(t, r), o)
	}
	if statuses["ok"] != 3 || statuses["blocked"] != 2 || statuses["unknown"] != 2 {
		t.Fatalf("statuses %v", statuses)
	}
	checkFactsSorted(t, got)
}

func checkFactsSorted(t *testing.T, rows []strategyresult.Row) {
	t.Helper()
	for _, r := range rows {
		if strings.Contains(r.Facts, `"annualized_yield"`) && strings.Index(r.Facts, `"annualized_yield"`) > strings.Index(r.Facts, `"break_even"`) {
			t.Fatalf("facts keys are not sorted: %s", r.Facts)
		}
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestShadowRunsInBothJobs(t *testing.T) {
	t.Parallel()
	plain := &fakeResults{}
	_, st := shadowFixture(t, plain)
	runNew(t, st)

	withCalls := &fakeResults{}
	f, st2 := shadowFixture(t, withCalls)
	j, err := NewWithCalls(st2, &fakeDividends{}, mixedRule{}, threeEntries(), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), runDay, nil); res.Err != nil || res.Returned != 13 {
		t.Fatalf("result %+v", res)
	}
	if withCalls.calls != 1 {
		t.Fatalf("%d result inserts", withCalls.calls)
	}
	var kinds = map[string]int{}
	for _, r := range writtenRows(f) {
		kinds[r.Kind]++
	}
	if kinds["covered_call"] != 3 || kinds["held_call_guard"] != 3 || kinds["short_put"] != 7 {
		t.Fatalf("old kinds %v", kinds)
	}
	a, b := plain.all(), withCalls.all()
	if len(a) != 7 || len(b) != 7 {
		t.Fatalf("%d and %d result rows", len(a), len(b))
	}
	for i := range a {
		x, y := a[i], b[i]
		for _, r := range []strategyresult.Row{y} {
			if r.Strategy != "short_put" || mustAtoi(t, r.Key)%100 > 10 {
				t.Fatalf("a covered-call or guard row is in Results: %+v", r)
			}
		}
		x.RunID, y.RunID = "", ""
		if !reflect.DeepEqual(x, y) {
			t.Fatalf("put row %d differs:\n%+v\n%+v", i, x, y)
		}
	}
	if a[0].RunID == "" || b[0].RunID != putsOf(writtenRows(f))[mustAtoi(t, b[0].Key)].RunID {
		t.Fatal("the run id of the results is not the run id of the old rows")
	}
}

func TestShadowFailureDoesNotFailTheRun(t *testing.T) {
	t.Parallel()
	plainF, plain := shadowFixture(t, nil)
	plain.Strategy = nil
	want := runNew(t, plain)

	f, st := shadowFixture(t, &fakeResults{err: errors.New("boom")})
	st.log = slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	got := runNew(t, st)
	if got.Err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("result %+v, without the shadow %+v", got, want)
	}
	if len(writtenRows(f)) != 7 || !reflect.DeepEqual(writtenRows(f), writtenRows(plainF)) {
		t.Fatal("the old rows differ from the run without the shadow or are not written")
	}
}

// countingStrategy fails the test when anything calls it.
type countingStrategy struct {
	strategy.Strategy
	mu    sync.Mutex
	calls int
}

func (c *countingStrategy) bump() { c.mu.Lock(); c.calls++; c.mu.Unlock() }
func (c *countingStrategy) Evaluate(s strategy.Snapshot) []strategy.Evaluation {
	c.bump()
	return c.Strategy.Evaluate(s)
}
func (c *countingStrategy) Rank(e []strategy.Evaluation) []strategy.Evaluation {
	c.bump()
	return c.Strategy.Rank(e)
}

func TestNilStrategyIsTodaysJob(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"both nil", "results only", "strategy only"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			results := &fakeResults{}
			f, st := shadowFixture(t, nil)
			cs := &countingStrategy{Strategy: st.Strategy}
			switch name {
			case "both nil":
				st.Strategy = nil
			case "results only":
				st.Strategy, st.Results = nil, results
			case "strategy only":
				st.Strategy = cs
			}
			res := runNew(t, st)
			if res.Err != nil || res.Returned != 7 || res.Usable != 3 {
				t.Fatalf("result %+v", res)
			}
			if len(writtenRows(f)) != 7 || results.calls != 0 || cs.calls != 0 {
				t.Fatalf("%d old rows, %d result inserts, %d strategy calls", len(writtenRows(f)), results.calls, cs.calls)
			}
		})
	}
}

type recordHandler struct {
	mu   sync.Mutex
	recs []slog.Record
}

func (h *recordHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	h.recs = append(h.recs, r)
	h.mu.Unlock()
	return nil
}
func (h *recordHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordHandler) WithGroup(string) slog.Handler      { return h }

func TestShadowFailureIsLoggedAtErrorWithRunID(t *testing.T) {
	t.Parallel()
	f, st := shadowFixture(t, &fakeResults{err: errors.New("secret-value-boom")})
	h := &recordHandler{}
	st.log = slog.New(h)
	if res := runNew(t, st); res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	runID := writtenRows(f)[0].RunID
	if runID == "" || len(h.recs) != 1 || h.recs[0].Level != slog.LevelError {
		t.Fatalf("records %+v", h.recs)
	}
	var got string
	h.recs[0].Attrs(func(a slog.Attr) bool {
		if a.Key == "run_id" {
			got = a.Value.String()
		}
		if strings.Contains(a.Value.String(), "secret-value-boom") {
			t.Errorf("attribute %s carries the error text", a.Key)
		}
		return true
	})
	if got != runID {
		t.Fatalf("run_id %q, old rows %q", got, runID)
	}
}

func TestShadowRowsCarryTheSettingsVersionOfTheOldRows(t *testing.T) {
	t.Parallel()
	results := &fakeResults{}
	f, st := shadowFixture(t, results)
	j, err := New(st, mixedRule{}, threeEntries(), fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	if j, err = WithSettingsVersion(j, versionOf(9, nil, new([]time.Time))); err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	old := writtenRows(f)
	got := results.all()
	if len(got) != 7 || len(old) != 7 {
		t.Fatalf("%d result rows, %d old rows", len(got), len(old))
	}
	for _, r := range got {
		if r.SettingsVersion == nil || *r.SettingsVersion != 9 {
			t.Fatalf("settings version of %s: %v", r.Key, r.SettingsVersion)
		}
	}
	for _, r := range old {
		if r.SettingsVersion == nil || *r.SettingsVersion != 9 {
			t.Fatalf("old row without the version")
		}
	}
}
