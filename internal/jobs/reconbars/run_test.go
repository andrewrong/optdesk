package reconbars_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute/recon"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/reconbars"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

const runDay = contracts.Day("2026-11-09")

// now is near the system time because a context deadline counts in it; the fake clock does not move.
var now = clockwork.NewRealClock().Now().UTC().Truncate(time.Second)

type oursRow struct{ Symbol, Currency, Day, Close string }

type fixtureOurs struct{ Rows []oursRow }

type fixtureSource struct {
	Bars       []struct{ Symbol, Currency, Day, Close string }
	Unanswered []struct{ Symbol, Currency, Code string }
}

func load[T any](t *testing.T, path string) T {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// fakeResolver maps the con id to the instrument id: the con id itself.
type fakeResolver struct{ ids map[string]int64 }

func (f fakeResolver) ResolveInstrument(_ context.Context, k store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	return f.ids[k.Value], nil
}

// fakeReader serves the winner rows per instrument id and records the calls.
type fakeReader struct {
	mu    sync.Mutex
	rows  map[int64][]contracts.PriceBar
	calls []readCall
}

type readCall struct {
	id       int64
	basis    string
	from, to contracts.Day
}

func (f *fakeReader) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, readCall{id, basis, from, to})
	return f.rows[id], nil
}

// fakeSource answers with answer and records every call and its deadline.
type fakeSource struct {
	mu        sync.Mutex
	answer    sources.BarResult
	err       error
	reqs      []sources.BarRequest
	deadlines []time.Time
}

func (f *fakeSource) DailyBars(ctx context.Context, req sources.BarRequest) (sources.BarResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	d, _ := ctx.Deadline()
	f.deadlines = append(f.deadlines, d)
	return f.answer, f.err
}

type fakeWriter struct {
	mu   sync.Mutex
	runs []reconstore.Run
}

func (f *fakeWriter) InsertRun(_ context.Context, r reconstore.Run) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.runs = append(f.runs, r)
	return int64(len(f.runs)), nil
}

func (f *fakeWriter) Close() error { return nil }

type fixture struct {
	src *fakeSource
	rd  *fakeReader
	w   *fakeWriter
	job contracts.Job
	clk clockwork.Clock
}

var conIDs = map[string]int64{"AAPL": 1, "MSFT": 2, "7203": 3}

func entry(sym, cur, market string) jobs.WatchEntry {
	return jobs.WatchEntry{Market: market, Contract: broker.Contract{SecType: "STK", Symbol: sym, Currency: cur, ConID: conIDs[sym]}}
}

func pb(id int64, day, close string) contracts.PriceBar {
	d, _ := time.Parse("2006-01-02", day)
	return contracts.PriceBar{InstrumentID: int(id), Basis: "trades", Close: close, SessionDay: types.SerializableDate{Time: d}}
}

// build wires the fixtures of testdata; tweak may change them before the job is built.
func build(t *testing.T, tweak func(*fixtureOurs, *fixtureSource)) *fixture {
	t.Helper()
	ours := load[fixtureOurs](t, "testdata/bars_ours.json")
	ans := load[fixtureSource](t, "testdata/source_answer.json")
	if tweak != nil {
		tweak(&ours, &ans)
	}
	f := &fixture{src: &fakeSource{}, rd: &fakeReader{rows: map[int64][]contracts.PriceBar{}}, w: &fakeWriter{},
		clk: clockwork.NewFakeClockAt(now)}
	for _, r := range ours.Rows {
		id := conIDs[r.Symbol]
		f.rd.rows[id] = append(f.rd.rows[id], pb(id, r.Day, r.Close))
	}
	f.src.answer = sources.BarResult{Bars: []sources.RefBar{}, Unanswered: []sources.BarUnanswered{}}
	for _, b := range ans.Bars {
		f.src.answer.Bars = append(f.src.answer.Bars, sources.RefBar{
			Pair: sources.BarPair{Symbol: b.Symbol, Currency: b.Currency}, Day: contracts.Day(b.Day), Close: b.Close})
	}
	for _, u := range ans.Unanswered {
		f.src.answer.Unanswered = append(f.src.answer.Unanswered, sources.BarUnanswered{
			Pair: sources.BarPair{Symbol: u.Symbol, Currency: u.Currency}, Code: u.Code})
	}
	entries := []jobs.WatchEntry{entry("AAPL", "USD", "US"), entry("MSFT", "USD", "US"), entry("7203", "JPY", "JP"),
		entry("XXXX", "EUR", "EU")}
	res := fakeResolver{ids: map[string]int64{"1": 1, "2": 2, "3": 3}}
	j, err := reconbars.New(f.src, res, f.rd, f.w, recon.Compare, entries, f.clk)
	if err != nil {
		t.Fatal(err)
	}
	f.job = j
	return f
}

func (f *fixture) run(t *testing.T) contracts.JobResult {
	t.Helper()
	keys, _, err := f.job.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	return f.job.Run(context.Background(), runDay, keys)
}

func (f *fixture) onlyRun(t *testing.T) reconstore.Run {
	t.Helper()
	if len(f.w.runs) != 1 {
		t.Fatalf("%d runs written, want 1", len(f.w.runs))
	}
	return f.w.runs[0]
}

func kinds(r reconstore.Run) map[string]string {
	out := map[string]string{}
	for _, it := range r.Items {
		out[it.Key] = it.Kind
	}
	return out
}

func TestReadsWinnerRows(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	f.run(t)
	if len(f.rd.calls) != 3 {
		t.Fatalf("reads %+v", f.rd.calls)
	}
	for _, c := range f.rd.calls {
		if c.basis != "trades" || c.from != "2026-11-03" || c.to != runDay {
			t.Fatalf("read %+v", c)
		}
	}
	r := f.onlyRun(t)
	// our AAPL 11-09 differs from the source beyond 0.1 percent; the day 11-02 of 7203 lies outside the window
	if r.PairID != "R6" || r.SessionDay != runDay || r.SourceID != "yfinance" || r.Diff != 1 || r.Matched != 2 {
		t.Fatalf("run %+v", r)
	}
	if _, ok := kinds(r)["7203|JPY|2026-11-02"]; ok {
		t.Fatalf("a day outside the window is compared: %+v", r.Items)
	}
}

func TestUnansweredPairSkipped(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	f.run(t)
	r := f.onlyRun(t)
	got := kinds(r)
	if got["MSFT|USD|2026-11-09"] != "skipped" || r.Skipped != 1 || r.PairsAsked != 3 || r.PairsCompared != 2 {
		t.Fatalf("run %+v", r)
	}
	for _, it := range r.Items {
		if it.Kind == "skipped" && (it.Reason != "rate_limited" || it.Company != "MSFT") {
			t.Fatalf("item %+v", it)
		}
	}
}

func TestSourceOnlyDayMissingOurs(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	f.run(t)
	r := f.onlyRun(t)
	if kinds(r)["AAPL|USD|2026-11-05"] != "missing_ours" || r.MissingOurs != 1 || r.MissingOther != 0 {
		t.Fatalf("run %+v", r)
	}
}

func TestDayOnlyOursIsSkipped(t *testing.T) {
	t.Parallel()
	f := build(t, func(o *fixtureOurs, _ *fixtureSource) {
		o.Rows = append(o.Rows, oursRow{"7203", "JPY", "2026-11-06", "2950.0000"})
	})
	f.run(t)
	r := f.onlyRun(t)
	if kinds(r)["7203|JPY|2026-11-06"] != "skipped" || r.MissingOther != 0 {
		t.Fatalf("run %+v", r)
	}
	for _, it := range r.Items {
		if it.Key == "7203|JPY|2026-11-06" && it.Reason != recon.SourceLacks {
			t.Fatalf("item %+v", it)
		}
	}
}

type codedError struct{ code string }

func (e codedError) Error() string             { return "x" }
func (e codedError) Kind() contracts.ErrorKind { return contracts.Retryable }
func (e codedError) Code() string              { return e.code }

func TestSourceDownFailedReason(t *testing.T) {
	t.Parallel()
	for _, code := range []string{"rate_limited", "network", "timeout", "source_changed"} {
		f := build(t, nil)
		f.src.answer, f.src.err = sources.BarResult{}, codedError{code}
		res := f.run(t)
		r := f.onlyRun(t)
		if res.Err == nil || r.Status != "failed" || r.Reason != code || len(r.Items) != 0 || r.N != 0 || r.PairsCompared != 0 {
			t.Fatalf("%s: run %+v result %+v", code, r, res)
		}
	}
	f := build(t, nil)
	f.src.answer, f.src.err = sources.BarResult{}, errors.New("no code at all")
	f.run(t)
	if r := f.onlyRun(t); r.Status != "failed" || r.Reason != "network" {
		t.Fatalf("run %+v", r)
	}
}

func TestPartialRunHasReason(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		unanswered []sources.BarUnanswered
		reason     string
	}{
		"first other than no_data": {[]sources.BarUnanswered{
			{Pair: sources.BarPair{Symbol: "MSFT", Currency: "USD"}, Code: "no_data"},
			{Pair: sources.BarPair{Symbol: "7203", Currency: "JPY"}, Code: "timeout"},
			{Pair: sources.BarPair{Symbol: "AAPL", Currency: "USD"}, Code: "network"},
		}, "timeout"},
		"all no_data": {[]sources.BarUnanswered{{Pair: sources.BarPair{Symbol: "MSFT", Currency: "USD"}, Code: "no_data"}}, ""},
		"none":        {nil, ""},
	} {
		f := build(t, func(_ *fixtureOurs, s *fixtureSource) { s.Unanswered = nil })
		f.src.answer.Unanswered = c.unanswered
		f.run(t)
		r := f.onlyRun(t)
		want := "partial"
		if c.unanswered == nil {
			want = "ok"
		}
		if r.Status != want || r.Reason != c.reason {
			t.Fatalf("%s: status %q reason %q", name, r.Status, r.Reason)
		}
	}
}

func TestOneDailyBarsCallWithRunBoundDeadline(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	f.run(t)
	if len(f.src.reqs) != 1 {
		t.Fatalf("%d calls", len(f.src.reqs))
	}
	req := f.src.reqs[0]
	if !slices.Equal(req.Pairs, []sources.BarPair{{Symbol: "AAPL", Currency: "USD"}, {Symbol: "MSFT", Currency: "USD"}, {Symbol: "7203", Currency: "JPY"}}) ||
		req.From != "2026-11-03" || req.To != runDay {
		t.Fatalf("request %+v", req)
	}
	if want := now.Add(28 * time.Minute); !f.src.deadlines[0].Equal(want) {
		t.Fatalf("deadline %v, want %v", f.src.deadlines[0], want)
	}
}

func TestMatchItemsNotStored(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	f.run(t)
	r := f.onlyRun(t)
	if r.Matched != 2 {
		t.Fatalf("matched %d", r.Matched)
	}
	for _, it := range r.Items {
		if it.Kind == "match" || it.Company == "" || it.Company == it.Key {
			t.Fatalf("item %+v", it)
		}
	}
	if len(r.Items) != r.Diff+r.MissingOurs+r.MissingOther+r.Skipped {
		t.Fatalf("items %d of %+v", len(r.Items), r)
	}
}

func TestTwoRunsSameDay(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	f.run(t)
	f.run(t)
	if len(f.w.runs) != 2 || f.src.reqs == nil || len(f.src.reqs) != 2 {
		t.Fatalf("%d runs, %d calls", len(f.w.runs), len(f.src.reqs))
	}
}

func TestKilledRunWritesNothing(t *testing.T) {
	t.Parallel()
	f := build(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.src.err = context.Canceled
	res := f.job.Run(ctx, runDay, nil)
	if res.Err == nil || len(f.w.runs) != 0 {
		t.Fatalf("result %+v, %d runs", res, len(f.w.runs))
	}
}
