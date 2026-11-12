package positionmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

const testDay = contracts.Day("2026-11-05")

var t0 = time.Date(2026, 11, 5, 21, 40, 0, 0, time.UTC)

type fakeAcct struct {
	pos map[string][]store.Position
	err map[string]error

	positionCalls int
	lastDay       string
}

func (f *fakeAcct) Positions(_ context.Context, alias, day string) ([]store.Position, error) {
	f.positionCalls++
	f.lastDay = day
	return f.pos[alias], f.err[alias]
}
func (f *fakeAcct) Values(context.Context, string, string) ([]store.AccountValue, error) {
	return nil, nil
}
func (f *fakeAcct) Executions(context.Context, string, string, string) ([]store.Execution, error) {
	return nil, nil
}

type fakeMarket struct {
	broker.Broker
	clk        *clockwork.FakeClock
	mu         sync.Mutex
	calls      [][]broker.Contract
	kinds      []broker.Kind
	quote      func(c broker.Contract) broker.Quote
	err        error
	resolveErr error
	advance    time.Duration
}

func (m *fakeMarket) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	if m.resolveErr != nil {
		return broker.Resolved{}, m.resolveErr
	}
	return broker.Resolved{Listed: true, Contract: broker.Contract{Exchange: "TSEJ"}}, nil
}

func (m *fakeMarket) Snapshot(_ context.Context, k broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, cs)
	m.kinds = append(m.kinds, k)
	m.clk.Advance(m.advance)
	if m.err != nil {
		return nil, m.err
	}
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = m.quote(c)
	}
	return out, nil
}

type fakeWriter struct {
	mu   sync.Mutex
	rows []mark.PositionMark
	err  error
}

func (w *fakeWriter) WriteMarks(_ context.Context, rows []mark.PositionMark) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.rows = append(w.rows, rows...)
	return nil
}

func pos(alias, capture string, id int64, cur string) store.Position {
	return store.Position{
		CaptureID: capture, Alias: alias, CapturedAt: t0, SessionDay: string(testDay), Symbol: "S", SecType: "stock",
		Currency: cur, ConID: id,
	}
}

func okQuote(c broker.Contract) broker.Quote {
	b, a := 1.0, 1.2
	return broker.Quote{Contract: c, Status: broker.StatusOK, Bid: &b, Ask: &a}
}

type env struct {
	j   *job
	m   *fakeMarket
	w   *fakeWriter
	a   *fakeAcct
	clk *clockwork.FakeClock
	log *bytes.Buffer
}

func setup(t *testing.T, aliases []string, ps map[string][]store.Position) env {
	t.Helper()
	clk := clockwork.NewFakeClockAt(t0)
	e := env{
		m: &fakeMarket{clk: clk, quote: okQuote}, w: &fakeWriter{}, a: &fakeAcct{pos: ps, err: map[string]error{}},
		clk: clk, log: &bytes.Buffer{},
	}
	j, err := newJob(e.m, aliases, e.a, e.w, clk, slog.New(slog.NewTextHandler(e.log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	e.j = j
	return e
}

func (e env) run() contracts.JobResult {
	keys, _, _ := e.j.Plan(context.Background(), testDay)
	return e.j.Run(context.Background(), testDay, keys)
}

func kindOf(r contracts.JobResult) contracts.ErrorKind {
	if r.Err == nil {
		return ""
	}
	return r.Err.Kind()
}

func TestRunWritesOneRowPerPositionAtTheMid(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD"), pos("a", "c1", 2, "USD")}})
	res := e.run()
	if res.Err != nil || res.Returned != 2 || res.Usable != 2 {
		t.Fatalf("res = %+v", res)
	}
	r := e.w.rows[0]
	if r.Alias != "a" || r.CaptureID != "c1" || r.Attempt != 1 || r.Price != "1.1" || r.Basis != mark.BasisMid ||
		r.Status != "ok" || r.SessionDay != "2026-11-05" || r.Currency != "USD" || r.Bid != "1" || r.Ask != "1.2" {
		t.Fatalf("row = %+v", r)
	}
	if len(e.m.calls) != 1 || e.m.kinds[0] != broker.KindSnapshot {
		t.Fatalf("calls = %d kinds %v", len(e.m.calls), e.m.kinds)
	}
}

func TestRunBatchesBelowSeventyLines(t *testing.T) {
	t.Parallel()
	var ps []store.Position
	for i := int64(1); i <= 150; i++ {
		ps = append(ps, pos("a", "c1", i, "USD"))
	}
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": ps})
	res := e.run()
	if res.Err != nil || res.Returned != 150 {
		t.Fatalf("res = %+v", res)
	}
	if len(e.m.calls) < 3 {
		t.Fatalf("batches = %d", len(e.m.calls))
	}
	for _, c := range e.m.calls {
		if len(c) >= 70 {
			t.Fatalf("batch of %d lines", len(c))
		}
	}
}

func TestRunBudgetLeavesPositionsUnreachedAndEndsPartial(t *testing.T) {
	t.Parallel()
	var ps []store.Position
	for i := int64(1); i <= 150; i++ {
		ps = append(ps, pos("a", "c1", i, "USD"))
	}
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": ps})
	e.m.advance = 50 * time.Second // the first batch spends the budget
	res := e.run()
	if kindOf(res) != contracts.Retryable || !strings.Contains(res.Err.Error(), "partial") {
		t.Fatalf("err = %v", res.Err)
	}
	if res.Returned != batchSize || len(e.m.calls) != 1 {
		t.Fatalf("returned %d calls %d", res.Returned, len(e.m.calls))
	}
	// the retry marks only the rest, with attempt 1 (no row existed)
	e.m.advance = 0
	res = e.run()
	if res.Err != nil || res.Returned != 150-batchSize {
		t.Fatalf("retry res = %+v", res)
	}
}

func TestRunNoQuoteWritesNoneRowWithStatus(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD"), pos("a", "c1", 2, "USD")}})
	e.m.quote = func(c broker.Contract) broker.Quote {
		if c.ConID == 2 {
			return broker.Quote{Contract: c, Status: broker.StatusNoSubscription}
		}
		return okQuote(c)
	}
	res := e.run()
	if res.Err != nil || res.Returned != 2 || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
	r := e.w.rows[1]
	if r.Basis != mark.BasisNone || r.Price != "" || r.Status != "no_subscription" {
		t.Fatalf("row = %+v", r)
	}
}

func TestRetryInsertsNextAttemptOnlyForNoneRows(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD"), pos("a", "c1", 2, "USD")}})
	e.m.quote = func(c broker.Contract) broker.Quote {
		if c.ConID == 2 {
			return broker.Quote{Contract: c, Status: broker.StatusNoData}
		}
		return okQuote(c)
	}
	e.run()
	e.m.quote = okQuote
	res := e.run()
	if res.Err != nil || res.Returned != 1 || len(e.m.calls) != 2 || len(e.m.calls[1]) != 1 || e.m.calls[1][0].ConID != 2 {
		t.Fatalf("res = %+v calls = %+v", res, e.m.calls)
	}
	r := e.w.rows[2]
	if r.ConID != 2 || r.Attempt != 2 || r.Basis != mark.BasisMid {
		t.Fatalf("row = %+v", r)
	}
	if res := e.run(); res.Returned != 0 || len(e.m.calls) != 2 {
		t.Fatalf("a priced position must not be marked again: %+v", res)
	}
}

func TestRunNoCaptureForMainFails(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{})
	res := e.run()
	if kindOf(res) != contracts.Retryable || !strings.Contains(res.Err.Error(), "no_capture") {
		t.Fatalf("err = %v", res.Err)
	}
}

func TestRunOtherAliasWithoutCaptureOrLoggedOutIsSkipped(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a", "b", "c"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}})
	e.a.err["c"] = errors.New("logged out")
	res := e.run()
	if res.Err != nil || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunOtherAliasWithoutPricesDoesNotFail(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a", "b"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}, "b": {pos("b", "c2", 5, "USD")}})
	e.m.quote = func(c broker.Contract) broker.Quote {
		if c.ConID == 5 {
			return broker.Quote{Contract: c, Status: broker.StatusNoData}
		}
		return okQuote(c)
	}
	if res := e.run(); res.Err != nil || res.Returned != 2 || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunMainWithoutMarksFails(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}})
	e.m.quote = func(c broker.Contract) broker.Quote { return broker.Quote{Contract: c, Status: broker.StatusNoData} }
	res := e.run()
	if kindOf(res) != contracts.Retryable || !strings.Contains(res.Err.Error(), "no_marks") || res.Returned != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunMarketDataGatewayDownFails(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a", "b"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}, "b": {pos("b", "c2", 2, "USD")}})
	e.m.quote = func(c broker.Contract) broker.Quote {
		return broker.Quote{Contract: c, Status: broker.StatusDisconnected}
	}
	res := e.run()
	if kindOf(res) != contracts.Retryable || res.Usable != 0 {
		t.Fatalf("res = %+v", res)
	}
	e = setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}})
	e.m.err = errors.New("socket down")
	if res := e.run(); kindOf(res) != contracts.Retryable || res.Returned != 0 {
		t.Fatalf("snapshot error: %+v", res)
	}
}

func TestRunWriteErrorFails(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}})
	e.w.err = errors.New("db")
	if res := e.run(); kindOf(res) != contracts.Retryable {
		t.Fatalf("res = %+v", res)
	}
}

func TestRunMarksNewestCaptureOnly(t *testing.T) {
	t.Parallel()
	old, nw := pos("a", "old", 1, "USD"), pos("a", "new", 2, "USD")
	old.CapturedAt = t0.Add(-time.Hour)
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {old, nw}})
	if res := e.run(); res.Returned != 1 || e.w.rows[0].CaptureID != "new" {
		t.Fatalf("res = %+v rows = %+v", res, e.w.rows)
	}
}

func TestRunJPYPositionAndUnsupportedSecType(t *testing.T) {
	t.Parallel()
	fut := pos("a", "c1", 3, "USD")
	fut.SecType = "future"
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD"), pos("a", "c1", 2, "JPY"), fut}})
	res := e.run()
	if res.Err != nil || res.Returned != 3 || res.Usable != 2 {
		t.Fatalf("res = %+v", res)
	}
	var sent []broker.Contract
	for _, c := range e.m.calls {
		sent = append(sent, c...)
	}
	if len(sent) != 2 || sent[1].Exchange != "TSEJ" || sent[0].Exchange != "SMART" {
		t.Fatalf("sent = %+v", sent)
	}
	if r := e.w.rows[2]; r.ConID != 3 || r.Basis != mark.BasisNone || r.Status != "no_data" {
		t.Fatalf("row = %+v", r)
	}
}

func TestRunJPYStockWithFailingLookupIsQuoted(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 7, "JPY")}})
	e.m.resolveErr = errors.New("ambiguous")
	res := e.run()
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
	if len(e.m.calls) != 1 || len(e.m.calls[0]) != 1 || e.m.calls[0][0].Exchange != "TSEJ" || e.m.calls[0][0].ConID != 7 {
		t.Fatalf("sent = %+v", e.m.calls)
	}
	if r := e.w.rows[0]; r.Basis == mark.BasisNone || r.Status == "error" {
		t.Fatalf("row = %+v", r)
	}
}

func TestRunErrorsCarryNoAccountText(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"main-alias"}, map[string][]store.Position{})
	res := e.run()
	if res.Err == nil || strings.Count(res.Err.Error(), " ") > 6 {
		t.Fatalf("err = %v", res.Err)
	}
}

func TestProbeFixtureFeedsTheJob(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/probe_snapshot.json")
	if err != nil {
		t.Fatal(err)
	}
	var fx struct {
		Synthetic bool `json:"synthetic"`
		Quotes    []struct {
			ConID  int64    `json:"con_id"`
			Status string   `json:"status"`
			MDT    int      `json:"market_data_type"`
			Bid    *float64 `json:"bid"`
			Ask    *float64 `json:"ask"`
			Last   *float64 `json:"last"`
			Close  *float64 `json:"close"`
		} `json:"quotes"`
	}
	if err := json.Unmarshal(data, &fx); err != nil || !fx.Synthetic {
		t.Fatalf("fixture: %v synthetic=%v", err, fx.Synthetic)
	}
	byID := map[int64]broker.Quote{}
	var ps []store.Position
	for _, q := range fx.Quotes {
		st := map[string]broker.Status{"ok": broker.StatusOK, "no_subscription": broker.StatusNoSubscription}[q.Status]
		byID[q.ConID] = broker.Quote{Status: st, MarketDataType: q.MDT, Bid: q.Bid, Ask: q.Ask, Last: q.Last, Close: q.Close}
		ps = append(ps, pos("a", "c1", q.ConID, "USD"))
	}
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": ps})
	e.m.quote = func(c broker.Contract) broker.Quote { return byID[c.ConID] }
	e.run()
	want := map[int64][2]string{1001: {"1.15", "mid"}, 1002: {"410.5", "last"}, 1003: {"", "none"}, 1004: {"2800.5", "close"}}
	for _, r := range e.w.rows {
		if w := want[r.ConID]; r.Price != w[0] || r.Basis != w[1] {
			t.Errorf("con %d: %q %q, want %v", r.ConID, r.Price, r.Basis, w)
		}
	}
}

func TestScheduleIsTenMinutesAfterTheAccountJob(t *testing.T) {
	t.Parallel()
	j, err := New(&fakeMarket{}, []string{"a"}, &fakeAcct{}, &fakeWriter{}, clockwork.NewFakeClock())
	if err != nil {
		t.Fatal(err)
	}
	if j.ID() != "position_marks" {
		t.Fatalf("id = %q", j.ID())
	}
	s := j.(interface {
		Schedule() (string, string, string, time.Duration)
		RunBound() time.Duration
		RestartSensitive() bool
	})
	cal, anchor, zone, off := s.Schedule()
	if cal != "XNYS" || anchor != "session_close" || zone != "" || off != 40*time.Minute {
		t.Fatalf("schedule = %s %s %q %v", cal, anchor, zone, off)
	}
	if s.RunBound() != 10*time.Minute || !s.RestartSensitive() {
		t.Fatal("run bound or restart sensitivity")
	}
}

func TestNewRefusesMissingParts(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClock()
	var nilAcct store.AccountReader
	var nilW mark.Writer
	var nilB broker.Broker
	for name, f := range map[string]func() error{
		"market":  func() error { _, e := New(nilB, []string{"a"}, &fakeAcct{}, &fakeWriter{}, clk); return e },
		"acct":    func() error { _, e := New(&fakeMarket{}, []string{"a"}, nilAcct, &fakeWriter{}, clk); return e },
		"writer":  func() error { _, e := New(&fakeMarket{}, []string{"a"}, &fakeAcct{}, nilW, clk); return e },
		"aliases": func() error { _, e := New(&fakeMarket{}, nil, &fakeAcct{}, &fakeWriter{}, clk); return e },
	} {
		if f() == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestParseManifestRejectsBadSchedule(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		"id = \"x\"\n[schedule]\nanchor = \"session_close\"\n",
		"id = \"x\"\n[schedule]\ncalendar = \"XNYS\"\nanchor = \"nope\"\n",
		"id = \"x\"\n[schedule]\ncalendar = \"XNYS\"\nanchor = \"session_close\"\noffset = \"abc\"\n",
		"[schedule]\ncalendar = \"XNYS\"\n",
	} {
		if _, err := parseManifest([]byte(s)); err == nil {
			t.Errorf("manifest %q accepted", s)
		}
	}
}

// readWriter is a store whose newest rows come from an earlier process.
type readWriter struct {
	fakeWriter
	stored  []mark.PositionMark
	readErr error
}

func (r *readWriter) Marks(context.Context, string, string) ([]mark.PositionMark, error) {
	return r.stored, r.readErr
}

func TestRetryWithFailingReadDoesNotReuseAttemptOne(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}})
	rw := &readWriter{readErr: errors.New("read failed")}
	j, err := newJob(e.m, []string{"a"}, e.a, rw, e.clk, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), testDay, nil)
	if kindOf(res) != contracts.Retryable || len(rw.rows) != 0 || len(e.m.calls) != 0 {
		t.Fatalf("res = %+v rows = %+v calls = %d", res, rw.rows, len(e.m.calls))
	}
}

func TestRetryReadsStoredNoneRowAndWritesNextAttempt(t *testing.T) {
	t.Parallel()
	e := setup(t, []string{"a"}, map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}})
	rw := &readWriter{stored: []mark.PositionMark{{CaptureID: "c1", ConID: 1, Attempt: 1, Basis: mark.BasisNone}}}
	j, err := newJob(e.m, []string{"a"}, e.a, rw, e.clk, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), testDay, nil)
	if res.Err != nil || len(rw.rows) != 1 || rw.rows[0].Attempt != 2 {
		t.Fatalf("res = %+v rows = %+v", res, rw.rows)
	}
}

func TestPastDayMarksClockDay(t *testing.T) {
	t.Parallel()
	ps := map[string][]store.Position{"a": {pos("a", "c1", 1, "USD")}}
	e := setup(t, []string{"a"}, ps)
	// 2026-11-06 05:00 UTC is 2026-11-06 00:00 EST: testDay is a past day.
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 6, 5, 0, 0, 0, time.UTC))
	e.m.clk = clk
	j, err := newJob(e.m, []string{"a"}, e.a, e.w, clk, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), testDay, nil)
	if res.Err != nil || len(e.w.rows) != 1 || len(e.m.calls) != 1 {
		t.Fatalf("past day: res = %+v rows %d calls %d", res, len(e.w.rows), len(e.m.calls))
	}
	if e.a.lastDay != "2026-11-06" {
		t.Fatalf("positions read for day %q, want the clock's date 2026-11-06", e.a.lastDay)
	}
}
