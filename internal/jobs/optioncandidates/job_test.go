package optioncandidates

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

const runDay = contracts.Day("2026-11-09")

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC) }

func day(s string) types.SerializableDate {
	t, _ := time.Parse(time.DateOnly, s)
	return types.SerializableDate{Time: t}
}

func f64(v float64) *float64 { return &v }

// fakes holds every store of the job; each field a test sets overrides the default.
type fakes struct {
	mu          sync.Mutex
	eodRows     []contracts.OptionEod
	chainDay    contracts.Day
	termIDs     [][]int64
	earnings    map[string][]contracts.EarningsDate
	values      map[string][]store.AccountValue // by alias
	positions   map[string][]store.Position
	written     [][]store.Candidate
	writeErr    error
	inputs      []compute.CandidateInput
	rate        *float64
	sessionDays []string
}

func (f *fakes) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	if from != f.chainDay || to != f.chainDay {
		return nil, errors.New("wrong chain day asked")
	}
	return f.eodRows, nil
}

func (f *fakes) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return []store.OptionEodLatest{{UnderlyingID: 7, SessionDay: f.chainDay}}, nil
}

func (f *fakes) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	f.mu.Lock()
	f.termIDs = append(f.termIDs, ids)
	f.mu.Unlock()
	var out []store.OptionTerms
	for _, id := range ids {
		out = append(out, store.OptionTerms{InstrumentID: id, Right: "P", Strike: "90", Multiplier: "100", ExpirationDate: "2026-12-11"})
	}
	return out, nil
}

func (f *fakes) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return []store.InstrumentName{{InstrumentID: 7, Type: "stock", Name: "ABC"}}, nil
}

func (f *fakes) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	if id != "DGS3MO" || f.rate == nil {
		return nil, nil
	}
	return []contracts.SeriesPoint{{SeriesID: id, ObservationDate: day("2026-11-06"), Value: f.rate}}, nil
}

func (f *fakes) EarningsDates(_ context.Context, issuer string, _, _ contracts.Day) ([]contracts.EarningsDate, error) {
	return f.earnings[issuer], nil
}

func (f *fakes) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	return f.positions[alias], nil
}
func (f *fakes) Values(_ context.Context, alias, _ string) ([]store.AccountValue, error) {
	return f.values[alias], nil
}
func (f *fakes) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

func (f *fakes) InsertCandidates(_ context.Context, rows []store.Candidate) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.mu.Lock()
	f.written = append(f.written, rows)
	f.mu.Unlock()
	return len(rows), nil
}

func (f *fakes) SessionDays(_ context.Context, cal string, _, _ contracts.Day) ([]contracts.SessionDay, error) {
	if cal != "XNYS" {
		return nil, errors.New("wrong calendar")
	}
	var out []contracts.SessionDay
	for _, d := range f.sessionDays {
		out = append(out, contracts.SessionDay{CalendarID: cal, SessionDay: day(d), IsSession: true})
	}
	return out, nil
}

// Evaluate records the input and returns one ok row per quote.
func (f *fakes) Evaluate(in compute.CandidateInput, _ compute.CandidateParams) []compute.Candidate {
	f.mu.Lock()
	f.inputs = append(f.inputs, in)
	f.mu.Unlock()
	var out []compute.Candidate
	for i, q := range in.Quotes {
		out = append(out, compute.Candidate{Kind: compute.KindShortPut, Underlying: in.Underlying, InstrumentID: q.InstrumentID,
			Expiry: q.Expiry, DTE: 32, Right: "P", Strike: q.Strike, Multiplier: q.Multiplier, Bid: q.Bid,
			AnnualizedYield: "0.1", Status: "ok", RankInUnderlying: i + 1,
			Gates: []compute.GateResult{{Gate: "buy_price", Status: compute.GatePass}}})
	}
	return out
}

func newFakes() *fakes {
	f := &fakes{chainDay: "2026-11-09", sessionDays: []string{"2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"}}
	f.eodRows = []contracts.OptionEod{
		{InstrumentID: 11, UnderlyingID: 7, Bid: f64(1.99), Ask: f64(2.05), BidSize: f64(5), OpenInterest: f64(40),
			UnderlyingPrice: f64(100.00049), Status: 1, MarketDataType: 1, CaptureID: "c1"},
	}
	return f
}

func (f *fakes) stores() jobs.CandidateStores {
	return jobs.CandidateStores{OptionEod: f, Catalog: f, Terms: f, Names: f, Series: f, Earnings: f, Accounts: f, Candidates: f, SessionDays: f}
}

func usEntry() jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: "cik:0000000001", Market: "US", BuyPrice: "91.5",
		Contract: broker.Contract{Symbol: "ABC", SecType: "STK"}}
}

func build(t *testing.T, st jobs.CandidateStores, f *fakes, entries ...jobs.WatchEntry) contracts.Job {
	t.Helper()
	j, err := New(st, f, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestRerunIsNewRun(t *testing.T) {
	t.Parallel()
	f := newFakes()
	j := build(t, f.stores(), f, usEntry())
	for range 2 {
		if res := j.Run(context.Background(), runDay, nil); res.Err != nil || res.Returned != 1 || res.Usable != 1 {
			t.Fatalf("result %+v", res)
		}
	}
	if len(f.written) != 2 {
		t.Fatalf("%d writes", len(f.written))
	}
	a, b := f.written[0][0], f.written[1][0]
	if a.RunID == "" || len(a.RunID) != 36 || a.RunID == b.RunID {
		t.Fatalf("run ids %q %q", a.RunID, b.RunID)
	}
	checkRow(t, a)
	if f.inputs[0].Spot != "100.0005" || f.inputs[0].BuyPrice != "91.5" || f.inputs[0].Kinds[0] != compute.KindShortPut {
		t.Fatalf("input %+v", f.inputs[0])
	}
}

func TestOnlyUSEntries(t *testing.T) {
	t.Parallel()
	f := newFakes()
	opt := usEntry()
	opt.Issuer, opt.Contract = "cik:0000000002", broker.Contract{Symbol: "ABC", SecType: "OPT"}
	j := build(t, f.stores(), f, usEntry(), opt)
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil || len(keys) != 1 || keys[0].SubjectKey != "cik:0000000001" || keys[0].DatasetID != "candidates.option_candidate" {
		t.Fatalf("keys %v, %v", keys, err)
	}
	j.Run(context.Background(), runDay, nil)
	if len(f.inputs) != 1 {
		t.Fatalf("%d evaluations", len(f.inputs))
	}
}

func TestStoreErrorFailsRun(t *testing.T) {
	t.Parallel()
	f := newFakes()
	f.writeErr = errors.New("boom")
	res := build(t, f.stores(), f, usEntry()).Run(context.Background(), runDay, nil)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "write_candidates_failed") {
		t.Fatalf("result %+v", res)
	}
}

func TestJPEntryNamedNotSilent(t *testing.T) {
	t.Parallel()
	f := newFakes()
	jp := jobs.WatchEntry{Issuer: "edinet:E00001", Market: "JP", Contract: broker.Contract{Symbol: "7203", SecType: "STK"}}
	j := build(t, f.stores(), f, usEntry(), jp)
	res := j.Run(context.Background(), runDay, nil)
	if res.Err != nil || res.Returned != 1 {
		t.Fatalf("result %+v", res)
	}
	sk := j.(interface{ Skipped() []string }).Skipped()
	if len(sk) != 1 || !strings.Contains(sk[0], "edinet:E00001") {
		t.Fatalf("skipped %v", sk)
	}
}

func TestNilEarningsIsUnknown(t *testing.T) {
	t.Parallel()
	f := newFakes()
	st := f.stores()
	st.Earnings, st.Accounts, st.Series = nil, nil, nil
	res := build(t, st, f, usEntry()).Run(context.Background(), runDay, nil)
	if res.Err != nil || res.Returned != 1 {
		t.Fatalf("result %+v", res)
	}
	in := f.inputs[0]
	if in.Earnings != nil || len(in.Accounts) != 0 || in.Rate != "" {
		t.Fatalf("input %+v", in)
	}
	if f.written[0][0].EarningsState != "none" {
		t.Fatalf("state %q", f.written[0][0].EarningsState)
	}
}

func TestNewRefusesMissingStore(t *testing.T) {
	t.Parallel()
	f := newFakes()
	st := f.stores()
	st.Candidates = nil
	if _, err := New(st, f, nil, fixedClock{}); err == nil {
		t.Fatal("a nil writer was accepted")
	}
}

func checkRow(t *testing.T, a store.Candidate) {
	t.Helper()
	if a.Kind != "short_put" || a.Issuer != "cik:0000000001" || a.UnderlyingID != 7 || a.InstrumentID != 11 || a.Alias != "" {
		t.Fatalf("row %+v", a)
	}
	if a.Right != "put" || a.Multiplier != 100 || a.BidEod != "1.99" || a.Rank == nil || *a.Rank != 1 {
		t.Fatalf("row %+v", a)
	}
	if a.Gates != `[{"gate":"buy_price","status":"pass","reason":""}]` || a.Fits != "{}" || a.Marks != "[]" {
		t.Fatalf("row %+v", a)
	}
}
