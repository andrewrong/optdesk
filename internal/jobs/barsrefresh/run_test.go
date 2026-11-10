package barsrefresh

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

const runDay = contracts.Day("2026-11-09")

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 40, 0, 0, time.UTC) }

// fakeBroker answers ResolveContract only.
type fakeBroker struct {
	broker.Broker
	mu       sync.Mutex
	resolved int
	listed   bool
}

func (f *fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolved++
	c.ConID = 7000 + int64(f.resolved)
	return broker.Resolved{Listed: f.listed, Contract: c}, nil
}

// fakeResolver maps the con id to the instrument id: the con id itself.
type fakeResolver struct{}

func (fakeResolver) ResolveInstrument(_ context.Context, k store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	return strconv.ParseInt(k.Value, 10, 64)
}

// fakeHistory answers the bars its answer function gives and records every query.
type fakeHistory struct {
	mu      sync.Mutex
	queries []broker.BarQuery
	answer  func(q broker.BarQuery) ([]broker.Bar, error)
}

func (f *fakeHistory) Bars(_ context.Context, q broker.BarQuery) ([]broker.Bar, error) {
	f.mu.Lock()
	f.queries = append(f.queries, q)
	f.mu.Unlock()
	return f.answer(q)
}
func (f *fakeHistory) Close() error { return nil }

// memBars is a writer and a reader; the reader gives the highest epoch of each day.
type memBars struct {
	mu   sync.Mutex
	rows []contracts.PriceBar
}

func (m *memBars) InsertPriceBars(_ context.Context, rows []contracts.PriceBar) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, rows...)
	return len(rows), nil
}

func (m *memBars) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	win := map[string]contracts.PriceBar{}
	for _, r := range m.rows {
		d := r.SessionDay.Format(dayLayout)
		if int64(r.InstrumentID) != id || string(r.Basis) != basis || d < string(from) || d > string(to) {
			continue
		}
		if w, ok := win[d]; !ok || r.AdjustEpoch > w.AdjustEpoch {
			win[d] = r
		}
	}
	var out []contracts.PriceBar
	for _, r := range win {
		out = append(out, r)
	}
	return out, nil
}

func (m *memBars) epoch(e int) []contracts.PriceBar {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []contracts.PriceBar
	for _, r := range m.rows {
		if r.AdjustEpoch == e {
			out = append(out, r)
		}
	}
	return out
}

func stored(id int, day, close string, epoch int) contracts.PriceBar {
	d, _ := time.Parse(dayLayout, day)
	return contracts.PriceBar{
		InstrumentID: id, Basis: "trades", AdjustEpoch: epoch, Close: close, Open: close, High: close, Low: close,
		SessionDay: types.SerializableDate{Time: d}, Source: "ibkr", Currency: "USD",
	}
}

func mkbar(day, close string) broker.Bar {
	return broker.Bar{Day: day, Open: close, High: close, Low: close, Close: close}
}

func entry(issuer, sym string, conID int64) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: "US", Contract: broker.Contract{SecType: "STK", Symbol: sym, Exchange: "SMART", Currency: "USD", ConID: conID}}
}

func build(t *testing.T, h *fakeHistory, st *memBars, entries ...jobs.WatchEntry) (contracts.Job, *fakeBroker) {
	t.Helper()
	b := &fakeBroker{listed: true}
	j, err := New(b, h, fakeResolver{}, st, st, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	j.(*job).m.AdjustedCadenceDays = 0 // the weekly adjusted_last step has its own tests
	return j, b
}

func keysOf(subjects ...string) []contracts.ExpectedKey {
	var ks []contracts.ExpectedKey
	for _, s := range subjects {
		ks = append(ks, contracts.ExpectedKey{DatasetID: datasetID, SubjectKey: s})
	}
	return ks
}

func fixed(bars ...broker.Bar) func(broker.BarQuery) ([]broker.Bar, error) {
	return func(broker.BarQuery) ([]broker.Bar, error) { return bars, nil }
}

func TestFirstPullBackfills(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-06", "10.0000"), mkbar("2026-11-09", "11.0000"))}
	st := &memBars{}
	j, _ := build(t, h, st, entry("cik:0000000001", "AAA", 101))
	res := j.Run(context.Background(), runDay, keysOf("cik:0000000001"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].DurationDays != 400 || h.queries[0].Kind != broker.BarTrades || h.queries[0].End != "" {
		t.Fatalf("queries %+v", h.queries)
	}
	if rows := st.epoch(1); len(rows) != 2 || rows[0].InstrumentID != 101 || rows[0].Currency != "USD" {
		t.Fatalf("rows %+v", st.rows)
	}
	if res.Returned != 2 || res.Usable != 2 {
		t.Fatalf("result %+v", res)
	}
}

func TestBackfillCapDefersWithoutRequest(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "10.0000"))}
	st := &memBars{}
	// three names against a cap of 2 (the manifest says 10; the test lowers it)
	var entries []jobs.WatchEntry
	for i := 1; i <= 3; i++ {
		entries = append(entries, entry("cik:"+strconv.Itoa(i), "N"+strconv.Itoa(i), int64(100+i)))
	}
	b := &fakeBroker{listed: true}
	jb, err := New(b, h, fakeResolver{}, st, st, entries, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	jb.(*job).m.BackfillNamesPerRun = 2
	jb.(*job).m.AdjustedCadenceDays = 0
	var empty int
	for _, e := range entries {
		res := jb.Run(context.Background(), runDay, keysOf(e.Issuer))
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		if res.Returned == 0 {
			empty++
		}
	}
	if len(h.queries) != 2 || empty != 1 {
		t.Fatalf("requests %d, empty answers %d", len(h.queries), empty)
	}
	// the name that started a first pull stays eligible on a retry of the same day
	if res := jb.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
}

func TestDailyPullInsertsOnlyNewDays(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-06", "10.0000"), mkbar("2026-11-09", "11.0000"))}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-06", "10.0000", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].DurationDays != 5 {
		t.Fatalf("queries %+v", h.queries)
	}
	if len(st.rows) != 2 || st.rows[1].SessionDay.Format(dayLayout) != "2026-11-09" || st.rows[1].AdjustEpoch != 1 {
		t.Fatalf("rows %+v", st.rows)
	}
}

func TestRestatedDayRepullsUnderNextEpoch(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.DurationDays == 5 {
			return []broker.Bar{mkbar("2026-11-09", "50.0000")}, nil
		}
		return []broker.Bar{mkbar("2026-11-05", "49.0000"), mkbar("2026-11-06", "49.5000"), mkbar("2026-11-09", "50.0000")}, nil
	}}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-05", "98.0000", 1), stored(101, "2026-11-09", "100.0000", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 2 || h.queries[1].DurationDays != 400 {
		t.Fatalf("queries %+v", h.queries)
	}
	rows := st.epoch(2)
	if len(rows) != 3 || len(st.rows) != 5 {
		t.Fatalf("rows %+v", st.rows)
	}
	for _, r := range st.rows[2:] {
		if r.AdjustEpoch != 2 {
			t.Fatalf("pulled row at epoch %d", r.AdjustEpoch)
		}
	}
}

func TestWithinToleranceIsNotRestated(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "100.0005"))}
	st := &memBars{rows: []contracts.PriceBar{stored(101, "2026-11-09", "100.0000", 1)}}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || len(st.epoch(2)) != 0 || len(st.rows) != 1 {
		t.Fatalf("queries %d rows %+v", len(h.queries), st.rows)
	}
}

func TestOneKeyFailureDoesNotStopOthers(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.Contract.ConID == 101 {
			return nil, errors.New("boom")
		}
		return []broker.Bar{mkbar("2026-11-09", "10.0000")}, nil
	}}
	st := &memBars{}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101), entry("cik:2", "BBB", 102))
	res := j.Run(context.Background(), runDay, keysOf("cik:1", "cik:2"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("err %v", res.Err)
	}
	if len(st.rows) != 1 || st.rows[0].InstrumentID != 102 || res.Usable != 1 {
		t.Fatalf("rows %+v result %+v", st.rows, res)
	}
}

func TestIndexConIDResolvedOnce(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(q broker.BarQuery) ([]broker.Bar, error) {
		if q.DurationDays == 5 {
			return []broker.Bar{mkbar("2026-11-09", "50.0000")}, nil
		}
		return []broker.Bar{mkbar("2026-11-09", "50.0000")}, nil
	}}
	// stored 100 against pulled 50 forces a second pull of the same key in one run
	st := &memBars{rows: []contracts.PriceBar{stored(7001, "2026-11-09", "100.0000", 1)}}
	j, b := build(t, h, st)
	res := j.Run(context.Background(), runDay, keysOf("index:SPX"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 2 || h.queries[0].Contract.ConID != 7001 || h.queries[1].Contract.ConID != 7001 || b.resolved != 1 {
		t.Fatalf("queries %+v resolved %d", h.queries, b.resolved)
	}
}

func TestIndexNotListedIsPermanent(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed()}
	j, b := build(t, h, &memBars{})
	b.listed = false
	res := j.Run(context.Background(), runDay, keysOf("index:SPX"))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || len(h.queries) != 0 {
		t.Fatalf("err %v queries %d", res.Err, len(h.queries))
	}
}

func TestNoBarsIsUnusableNotZero(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed()}
	st := &memBars{}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	res := j.Run(context.Background(), runDay, keysOf("cik:1"))
	if res.Err != nil || res.Usable != 0 || len(st.rows) != 0 {
		t.Fatalf("result %+v rows %+v", res, st.rows)
	}
}

type retryable struct{}

func (retryable) Error() string             { return "paced" }
func (retryable) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestPacedIsRetryable(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: func(broker.BarQuery) ([]broker.Bar, error) { return nil, retryable{} }}
	st := &memBars{}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	res := j.Run(context.Background(), runDay, keysOf("cik:1"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(st.rows) != 0 {
		t.Fatalf("result %+v", res)
	}
}

func TestBadPulledCloseIsPermanentAndWritesNothing(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-08", "10.0000"), mkbar("2026-11-09", "n/a"))}
	st := &memBars{}
	j, _ := build(t, h, st, entry("cik:1", "AAA", 101))
	res := j.Run(context.Background(), runDay, keysOf("cik:1"))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || len(st.rows) != 0 {
		t.Fatalf("result %+v rows %+v", res, st.rows)
	}
}

// pairWinner reads like rd.price_bar_r1: winner = highest epoch of the pair, then the window.
type pairWinner struct{ memBars }

func (p *pairWinner) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	top := 0
	for _, r := range p.rows {
		if int64(r.InstrumentID) == id {
			top = max(top, r.AdjustEpoch)
		}
	}
	var out []contracts.PriceBar
	for _, r := range p.rows {
		d := r.SessionDay.Format(dayLayout)
		if int64(r.InstrumentID) == id && r.AdjustEpoch == top && d >= string(from) && d <= string(to) {
			out = append(out, r)
		}
	}
	return out, nil
}

func TestReaddedNameWritesAtOrAbovePairEpoch(t *testing.T) {
	t.Parallel()
	st := &pairWinner{memBars{rows: []contracts.PriceBar{stored(101, "2024-01-02", "10.0000", 2)}}}
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "50.0000"))}
	b := &fakeBroker{listed: true}
	j, err := New(b, h, fakeResolver{}, st, st, []jobs.WatchEntry{entry("cik:1", "A", 101)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	j.(*job).m.AdjustedCadenceDays = 0
	if res := j.Run(context.Background(), runDay, keysOf("cik:1")); res.Err != nil {
		t.Fatal(res.Err)
	}
	got, _ := st.PriceBars(context.Background(), 101, "trades", "2026-11-09", "2026-11-09")
	if len(got) != 1 {
		t.Fatalf("the bar written today is not a winner row: %d rows", len(got))
	}
}
