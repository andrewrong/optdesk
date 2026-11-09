package optioncandidates

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeDividends answers the rows it holds and records the arguments of each call.
type fakeDividends struct {
	mu    sync.Mutex
	rows  []contracts.DividendExpectation
	err   error
	calls []dividendCall
}

type dividendCall struct {
	id       int
	from, to contracts.Day
}

func (d *fakeDividends) Dividends(_ context.Context, id int, from, to contracts.Day) ([]contracts.DividendExpectation, error) {
	d.mu.Lock()
	d.calls = append(d.calls, dividendCall{id, from, to})
	d.mu.Unlock()
	return d.rows, d.err
}

func dividendRow(status, exDate, amount string) contracts.DividendExpectation {
	r := contracts.DividendExpectation{InstrumentID: 7, SessionDay: day("2026-11-09"), Attempt: 1}
	if status == "present" {
		r.Status += "present"
		ex := day(exDate)
		r.NextExDate, r.NextAmount = &ex, &amount
	} else {
		r.Status += "no_value"
	}
	return r
}

func buildCalls(t *testing.T, st jobs.CandidateStores, div *fakeDividends, rule compute.OptionCandidates) contracts.Job {
	t.Helper()
	j, err := NewWithCalls(st, div, rule, []jobs.WatchEntry{usEntry()}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestCallsNeedDividendReader(t *testing.T) {
	t.Parallel()
	f := newFakes()
	if _, err := NewWithCalls(f.stores(), nil, f, nil, fixedClock{}); err == nil {
		t.Fatal("a nil dividend reader was accepted")
	}
	st := f.stores()
	st.Candidates = nil
	if _, err := NewWithCalls(st, &fakeDividends{}, f, nil, fixedClock{}); err == nil {
		t.Fatal("a nil writer was accepted")
	}
	div := &fakeDividends{err: errors.New("boom")}
	res := buildCalls(t, f.stores(), div, f).Run(context.Background(), runDay, nil)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "read_dividend_failed") {
		t.Fatalf("result %+v", res)
	}
	if len(f.written) != 0 {
		t.Fatalf("%d writes after a failed read", len(f.written))
	}
}

func TestPutsUnchangedByCalls(t *testing.T) {
	t.Parallel()
	a, b := newFakes(), newFakes()
	if res := build(t, a.stores(), a, usEntry()).Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	div := &fakeDividends{}
	cj := buildCalls(t, b.stores(), div, b)
	if res := cj.Run(context.Background(), runDay, nil); res.Err != nil || res.Returned != 1 {
		t.Fatalf("result %+v", res)
	}
	if len(a.written) != 1 || len(b.written) != 1 {
		t.Fatalf("writes %d and %d", len(a.written), len(b.written))
	}
	want, got := a.written[0], b.written[0]
	for i := range want {
		want[i].RunID = ""
	}
	for i := range got {
		got[i].RunID = ""
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("puts differ:\n%+v\n%+v", want, got)
	}
	in := b.inputs[0]
	wantKinds := []compute.CandidateKind{compute.KindShortPut, compute.KindCoveredCall, compute.KindHeldCallGuard}
	if !reflect.DeepEqual(in.Kinds, wantKinds) || in.Dividend != nil {
		t.Fatalf("input %+v", in)
	}
	if len(div.calls) != 1 || div.calls[0] != (dividendCall{7, "2026-10-26", runDay}) {
		t.Fatalf("dividend calls %+v", div.calls)
	}
	if skipped := cj.(interface{ Skipped() []string }).Skipped(); len(skipped) != 0 {
		t.Fatalf("skipped %v", skipped)
	}
}

// callTerms answers a put and a call for each instrument asked: 11 is the put, 21 the call.
type callTerms struct{ *fakes }

func (c callTerms) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, id := range ids {
		right, strike := "P", "90"
		if id == 21 {
			right, strike = "C", "100"
		}
		out = append(out, store.OptionTerms{InstrumentID: id, Right: right, Strike: strike, Multiplier: "100", ExpirationDate: "2026-12-11"})
	}
	return out, nil
}

func heldFakes(bid float64) (*fakes, jobs.CandidateStores) {
	f := newFakes()
	f.eodRows = append(f.eodRows, contracts.OptionEod{InstrumentID: 21, UnderlyingID: 7, Bid: f64(bid), Ask: f64(bid + 0.05),
		BidSize: f64(5), OpenInterest: f64(40), UnderlyingPrice: f64(110), Status: 1, MarketDataType: 1, CaptureID: "c1"})
	f.eodRows[0].UnderlyingPrice = f64(110)
	pos := []store.Position{
		{Symbol: "ABC", SecType: "OPT", Right: "call", ConID: 21, Quantity: "-1"},
		{Symbol: "ABC", SecType: "OPT", Right: "call", ConID: 99, Quantity: "1"},
		{Symbol: "XYZ", SecType: "OPT", Right: "call", ConID: 98, Quantity: "-1"},
	}
	val := []store.AccountValue{{Tag: "AvailableFunds", Value: "100000", Currency: "USD"}}
	f.positions = map[string][]store.Position{"A": pos, "B": pos}
	f.values = map[string][]store.AccountValue{"A": val, "B": val}
	st := f.stores()
	st.Terms = callTerms{f}
	return f, st
}

func guardRows(t *testing.T, f *fakes) []store.Candidate {
	t.Helper()
	if len(f.written) != 1 {
		t.Fatalf("%d writes", len(f.written))
	}
	var out []store.Candidate
	for _, r := range f.written[0] {
		if r.Kind == "held_call_guard" {
			out = append(out, r)
		}
	}
	return out
}

func TestHeldCallGuardRows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		bid      float64
		row      contracts.DividendExpectation
		status   string
		gate     string
		marks    string
		exAmount string
	}{
		{"block", 10.40, dividendRow("present", "2026-11-19", "0.50"), "blocked",
			`{"gate":"guard","status":"block","reason":"early_assignment_risk"}`, "[]", "0.50"},
		{"ok", 10.60, dividendRow("present", "2026-11-19", "0.50"), "ok",
			`{"gate":"guard","status":"pass","reason":""}`, "[]", "0.50"},
		{"no_value", 10.40, dividendRow("no_value", "", ""), "ok",
			`{"gate":"guard","status":"pass","reason":""}`, `["no_dividend"]`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f, st := heldFakes(c.bid)
			div := &fakeDividends{rows: []contracts.DividendExpectation{dividendRow("no_value", "", ""), c.row}}
			if res := buildCalls(t, st, div, optioncand.New()).Run(context.Background(), runDay, nil); res.Err != nil {
				t.Fatal(res.Err)
			}
			rows := guardRows(t, f)
			if len(rows) != 2 || rows[0].Alias != "A" || rows[1].Alias != "B" {
				t.Fatalf("guard rows %+v", rows)
			}
			for _, r := range rows {
				checkGuardRow(t, r, c.status, c.gate, c.marks, c.exAmount)
			}
			for _, r := range f.written[0] {
				if r.Kind == "short_put" && (r.NextExDate != nil || r.ExAmount != nil) {
					t.Fatalf("a put carries a dividend: %+v", r)
				}
			}
		})
	}
}

// kindsRule answers one ok put and one ok covered call, so that the ranks of both start at 1.
type kindsRule struct{}

func (kindsRule) Evaluate(in compute.CandidateInput, _ compute.CandidateParams) []compute.Candidate {
	row := func(k compute.CandidateKind, id, right string) compute.Candidate {
		return compute.Candidate{Kind: k, Underlying: in.Underlying, InstrumentID: id, Expiry: "2026-12-11", DTE: 32,
			Right: right, Strike: "100", Multiplier: "100", Bid: "1", AnnualizedYield: "0.1", Status: "ok"}
	}
	return []compute.Candidate{row(compute.KindShortPut, "11", "P"), row(compute.KindCoveredCall, "21", "C")}
}

func TestRanksArePerKind(t *testing.T) {
	t.Parallel()
	f, st := heldFakes(10.4)
	if res := buildCalls(t, st, &fakeDividends{}, kindsRule{}).Run(context.Background(), runDay, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	rows := f.written[0]
	if len(rows) != 2 || rows[0].Kind != "short_put" || rows[1].Kind != "covered_call" {
		t.Fatalf("rows %+v", rows)
	}
	for _, r := range rows {
		if r.Rank == nil || *r.Rank != 1 {
			t.Fatalf("row %s rank %v", r.Kind, r.Rank)
		}
	}
}

func checkGuardRow(t *testing.T, r store.Candidate, status, gate, marks, exAmount string) {
	t.Helper()
	if r.InstrumentID != 21 || r.Status != status || r.Rank != nil || r.Right != "call" {
		t.Fatalf("row %+v", r)
	}
	if r.Gates != "["+gate+"]" || r.Marks != marks {
		t.Fatalf("gates %s marks %s", r.Gates, r.Marks)
	}
	if exAmount == "" {
		if r.ExAmount != nil || r.NextExDate != nil {
			t.Fatalf("ex fields %+v", r)
		}
		return
	}
	if r.ExAmount == nil || *r.ExAmount != exAmount || r.NextExDate == nil || *r.NextExDate != "2026-11-19" {
		t.Fatalf("ex fields %v %v", r.ExAmount, r.NextExDate)
	}
}
