package main

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/store/strategyresultpg"
)

// fakeResultStore is the results store of the job and of the command; it keeps what it is given and records its close.
type fakeResultStore struct {
	mu     sync.Mutex
	rows   []strategyresult.Row
	closed *atomic.Int32
	err    error
}

func (f *fakeResultStore) InsertResults(_ context.Context, rows []strategyresult.Row) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, rows...)
	return len(rows), nil
}

func (f *fakeResultStore) Results(_ context.Context, day contracts.Day, _ string) ([]strategyresult.Row, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []strategyresult.Row
	for _, r := range f.rows {
		if r.SessionDay == day {
			out = append(out, r)
		}
	}
	return out, f.err
}

func (f *fakeResultStore) Close() error {
	if f.closed != nil {
		f.closed.Add(1)
	}
	return nil
}

// shadowDeps is a deps with fake openers whose job constructor hands its stores to got.
func shadowDeps(closed *atomic.Int32, res *fakeResultStore, resErr error, got *optioncandidates.CandidateStores) deps {
	return deps{
		newClock: func() clock.Clock { return fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)} },
		openCandidateRW: func(context.Context, string) (candidateRW, error) {
			return &fakeCandidateRW{closed: closed}, nil
		},
		openStrategyResults: func(_ context.Context, dsn string) (resultStore, error) {
			if dsn != "cand-dsn" {
				return nil, errors.New("wrong dsn " + dsn)
			}
			if resErr != nil {
				return nil, resErr
			}
			return res, nil
		},
		newCandidateRule: func() compute.OptionCandidates { return fakeRule{} },
		newOptionCandidates: func(st optioncandidates.CandidateStores, _ compute.OptionCandidates, _ []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
			*got = st
			return candJobStub{id: jobOptionCandidates}, nil
		},
	}
}

func candidateJobOf(t *testing.T, d deps, cfg runOnceConfig, extra *closerSet) error {
	t.Helper()
	a, _ := testApp(t, "")
	f := jobFactory{d: d}.withList(oneUSRow())
	_, err := a.candidateJob(context.Background(), cfg, &f, extra)
	return err
}

func TestOptionCandidatesGetsTheStrategyAndResults(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	res := &fakeResultStore{closed: &closed}
	var got optioncandidates.CandidateStores
	var extra closerSet
	if err := candidateJobOf(t, shadowDeps(&closed, res, nil, &got), runOnceConfig{candidateDSN: "cand-dsn"}, &extra); err != nil {
		t.Fatal(err)
	}
	if got.Strategy == nil || got.Strategy.ID() != "short_put" {
		t.Fatalf("strategy %v", got.Strategy)
	}
	if got.Results != strategyresult.Writer(res) {
		t.Fatalf("results %v", got.Results)
	}
	if len(extra.cs) != 2 {
		t.Fatalf("closers %d: the candidate and the results store are both closed with the run", len(extra.cs))
	}
}

func TestNoCandidateDSNLeavesBothNil(t *testing.T) {
	t.Parallel()
	var closed atomic.Int32
	var opened atomic.Int32
	var got optioncandidates.CandidateStores
	d := shadowDeps(&closed, &fakeResultStore{}, nil, &got)
	d.openStrategyResults = func(context.Context, string) (resultStore, error) {
		opened.Add(1)
		return &fakeResultStore{}, nil
	}
	var extra closerSet
	if err := candidateJobOf(t, d, runOnceConfig{}, &extra); err != nil {
		t.Fatal(err)
	}
	if got.Strategy != nil || got.Results != nil || opened.Load() != 0 || len(extra.cs) != 0 {
		t.Fatalf("strategy %v results %v opened %d closers %d", got.Strategy, got.Results, opened.Load(), len(extra.cs))
	}
}

func TestResultsOpenFailureNamesKeyNotValue(t *testing.T) {
	t.Parallel()
	var got optioncandidates.CandidateStores
	d := shadowDeps(nil, nil, errors.New("dial postgres://u:secret-pw@host/db"), &got)
	err := candidateJobOf(t, d, runOnceConfig{candidateDSN: "cand-dsn"}, &closerSet{})
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvCandidate) || strings.Contains(err.Error(), "secret-pw") {
		t.Fatalf("error %v", err)
	}
}

func TestStrategyIdsHaveNeedRows(t *testing.T) {
	t.Parallel()
	list, err := buildStrategies(fakeRule{})
	if err != nil || len(list) == 0 {
		t.Fatalf("strategies %v %v", list, err)
	}
	needs, err := optioncandidates.Needs()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, n := range needs {
		have[n.Strategy] = true
	}
	for _, s := range list {
		if !have[s.ID()] {
			t.Errorf("strategy %q has no [[need]] row", s.ID())
		}
	}
}

// --- the shadow through the real reader ---

var shadowSymbols = map[int64]string{7: "AAA", 8: "BBB"}

// shadowStoresFake answers every reader of the job over two underlyings with one chain row each and keeps what the
// candidate writer is given.
type shadowStoresFake struct {
	mu      sync.Mutex
	written []store.Candidate
}

func (s *shadowStoresFake) OptionEod(_ context.Context, id int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	if from != to {
		d, _ := time.Parse(time.DateOnly, "2026-11-09")
		return []contracts.OptionEod{{SessionDay: types.SerializableDate{Time: d}}}, nil
	}
	f := func(v float64) *float64 { return &v }
	return []contracts.OptionEod{{InstrumentID: int(id) * 10, UnderlyingID: int(id), Bid: f(1.5), Ask: f(1.6),
		BidSize: f(5), OpenInterest: f(40), UnderlyingPrice: f(100 + float64(id)), Status: 1, MarketDataType: 1, CaptureID: "c"}}, nil
}

func (s *shadowStoresFake) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return []store.OptionEodLatest{{UnderlyingID: 7, SessionDay: "2026-11-09"}, {UnderlyingID: 8, SessionDay: "2026-11-09"}}, nil
}

func (s *shadowStoresFake) OptionTerms(_ context.Context, ids []int64) ([]store.OptionTerms, error) {
	var out []store.OptionTerms
	for _, id := range ids {
		out = append(out, store.OptionTerms{InstrumentID: id, Right: "P", Strike: "90", Multiplier: "100", ExpirationDate: "2026-12-11"})
	}
	return out, nil
}

func (s *shadowStoresFake) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	var out []store.InstrumentName
	for _, id := range ids {
		out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: shadowSymbols[id]})
	}
	return out, nil
}

func (s *shadowStoresFake) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return nil, nil
}

func (s *shadowStoresFake) SessionDays(_ context.Context, cal string, _, _ contracts.Day) ([]contracts.SessionDay, error) {
	var out []contracts.SessionDay
	for _, d := range []string{"2026-11-05", "2026-11-06", "2026-11-09"} {
		t, _ := time.Parse(time.DateOnly, d)
		out = append(out, contracts.SessionDay{CalendarID: cal, SessionDay: types.SerializableDate{Time: t}, IsSession: true})
	}
	return out, nil
}

func (s *shadowStoresFake) InsertCandidates(_ context.Context, rows []store.Candidate) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written = append(s.written, rows...)
	return len(rows), nil
}

// mixedPutRule answers an ok, a blocked and an unknown put for each underlying; every figure depends on the underlying.
type mixedPutRule struct{}

func (mixedPutRule) Evaluate(in compute.CandidateInput, _ compute.CandidateParams) []compute.Candidate {
	base := map[string]int{"AAA": 100, "BBB": 200}[in.Underlying]
	put := func(n int, status, yield string, rankIn int) compute.Candidate {
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
	return []compute.Candidate{put(1, "ok", "0.12", 2), put(2, "ok", "0.20", 1), put(3, "blocked", "", 0), put(4, "unknown", "", 0)}
}

type resultsOnly struct{ rows []strategyresult.Row }

func (r resultsOnly) Results(context.Context, contracts.Day, string) ([]strategyresult.Row, error) {
	return r.rows, nil
}

type oldReaderMustNotRun struct{ t *testing.T }

func (o oldReaderMustNotRun) Candidates(context.Context, contracts.Day, string) ([]store.Candidate, error) {
	o.t.Error("the old reader was asked although the day has a result run")
	return nil, nil
}

func TestShadowThroughRealReaderEqualsOldRows(t *testing.T) {
	t.Parallel()
	fk := &shadowStoresFake{}
	res := &fakeResultStore{}
	rule := mixedPutRule{}
	list, err := buildStrategies(rule)
	if err != nil {
		t.Fatal(err)
	}
	st := optioncandidates.CandidateStores{
		OptionEod: fk, Catalog: fk, Terms: fk, Names: fk, Series: fk, Candidates: fk, SessionDays: fk,
		Aliases: []string{"A", "B"}, Strategy: list[0], Results: res,
	}
	entries := []jobs.WatchEntry{
		{Issuer: "cik:0000000001", Market: "US", BuyPrice: "91.5", Contract: broker.Contract{Symbol: "AAA", SecType: "STK"}},
		{Issuer: "cik:0000000002", Market: "US", BuyPrice: "91.5", Contract: broker.Contract{Symbol: "BBB", SecType: "STK"}},
	}
	clk := fakeClock{time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC)}
	j, err := optioncandidates.New(st, rule, entries, clk)
	if err != nil {
		t.Fatal(err)
	}
	if r := j.Run(context.Background(), "2026-11-09", nil); r.Err != nil {
		t.Fatal(r.Err)
	}
	if len(fk.written) == 0 || len(res.rows) != len(fk.written) {
		t.Fatalf("old rows %d, result rows %d", len(fk.written), len(res.rows))
	}
	reader := strategyresultpg.NewCandidateReader(resultsOnly{res.rows}, oldReaderMustNotRun{t})
	got, err := reader.Candidates(context.Background(), "2026-11-09", "short_put")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(fk.written) {
		t.Fatalf("read %d rows, wrote %d", len(got), len(fk.written))
	}
	byKey := map[string]store.Candidate{}
	for _, w := range fk.written {
		byKey[w.Alias+"|"+strconv.Itoa(w.InstrumentID)] = w
	}
	for _, g := range got {
		w, ok := byKey[g.Alias+"|"+strconv.Itoa(g.InstrumentID)]
		if !ok {
			t.Fatalf("row %d has no old row", g.InstrumentID)
		}
		if !reflect.DeepEqual(g, w) {
			t.Errorf("row %d differs:\nresult %+v\nold    %+v", g.InstrumentID, g, w)
		}
	}
}
