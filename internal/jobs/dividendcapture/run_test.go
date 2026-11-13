package dividendcapture

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 11, 9, 21, 30, 0, 0, time.UTC) }

const day = contracts.Day("2026-11-09")

type fakeRef struct {
	mu    sync.Mutex
	asked [][]broker.Contract
	by    map[int64]broker.DividendProjection
	err   error
}

func (f *fakeRef) Dividends(_ context.Context, cs []broker.Contract) ([]broker.DividendProjection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, cs)
	if f.err != nil {
		return nil, f.err
	}
	out := make([]broker.DividendProjection, len(cs))
	for i, c := range cs {
		out[i] = f.by[c.ConID]
		out[i].Contract = c
	}
	return out, nil
}
func (*fakeRef) Close() error { return nil }

// fakeRes maps a con id to an instrument id (conid + 1000); a con id in fail is refused.
type fakeRes struct {
	fail map[int64]bool
	keys []store.Identifier
}

func (r *fakeRes) ResolveInstrument(_ context.Context, k store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	r.keys = append(r.keys, k)
	var n int64
	for _, c := range k.Value {
		n = n*10 + int64(c-'0')
	}
	if r.fail[n] {
		return 0, errors.New("resolver down")
	}
	return n + 1000, nil
}

type memStore struct {
	mu    sync.Mutex
	rows  []contracts.DividendExpectation
	fail  map[int]bool
	calls []upsertCall
}

type upsertCall struct {
	replay bool
	by     string
	n      int
}

func (m *memStore) InsertDividend(context.Context, contracts.DividendExpectation) error {
	return errors.New("InsertDividend must not be called")
}

// upsert replaces the row of the same instrument and session day, as the real upsert replaces an answer row.
func (m *memStore) upsert(replay bool, rows []contracts.DividendExpectation, by string) (fundkey.WriteCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, upsertCall{replay: replay, by: by, n: len(rows)})
	var c fundkey.WriteCount
	for _, r := range rows {
		if m.fail[r.InstrumentID] {
			return c, errors.New("db down")
		}
		replaced := false
		for i, x := range m.rows {
			if x.InstrumentID == r.InstrumentID && x.SessionDay.Equal(r.SessionDay.Time) {
				m.rows[i], replaced = r, true
			}
		}
		if !replaced {
			m.rows = append(m.rows, r)
		}
		c.Written++
	}
	return c, nil
}

func (m *memStore) UpsertDividends(_ context.Context, rows []contracts.DividendExpectation, by string) (fundkey.WriteCount, error) {
	return m.upsert(false, rows, by)
}

func (m *memStore) ReplayDividends(_ context.Context, rows []contracts.DividendExpectation, by string) (fundkey.WriteCount, error) {
	return m.upsert(true, rows, by)
}

// plainStore is a writer with no upsert.
type plainStore struct{ *memStore }

func (plainStore) UpsertDividends() {}

func (m *memStore) Dividends(_ context.Context, id int, from, to contracts.Day) ([]contracts.DividendExpectation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []contracts.DividendExpectation
	for _, x := range m.rows {
		d := x.SessionDay.Format("2006-01-02")
		if x.InstrumentID == id && d >= string(from) && d <= string(to) {
			out = append(out, x)
		}
	}
	return out, nil
}

func entry(issuer, market, sym string, conid int64) jobs.WatchEntry {
	return jobs.WatchEntry{Issuer: issuer, Market: market,
		Contract: broker.Contract{SecType: "STK", Symbol: sym, Exchange: "SMART", Currency: "USD", ConID: conid}}
}

func f64(v float64) *float64 { return &v }
func str(v string) *string   { return &v }

func run(t *testing.T, ref *fakeRef, res *fakeRes, st *memStore, es ...jobs.WatchEntry) contracts.JobResult {
	t.Helper()
	j, err := New(ref, res, st, st, es, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), day, nil)
}

func rowOf(t *testing.T, st *memStore, id int) contracts.DividendExpectation {
	t.Helper()
	for _, r := range st.rows {
		if r.InstrumentID == id {
			return r
		}
	}
	t.Fatalf("no row for %d in %+v", id, st.rows)
	return contracts.DividendExpectation{}
}

func TestPresentRow(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{
		1: {Status: broker.DividendOK, Past12m: f64(0.96), Next12m: f64(1), NextExDate: str("20261112"), NextAmount: f64(0.25)},
	}}
	st := &memStore{}
	res := run(t, ref, &fakeRes{}, st, entry("cik:0000320193", "US", "AAPL", 1))
	if res.Err != nil || res.Returned != 1 || res.Usable != 1 {
		t.Fatalf("result %+v", res)
	}
	r := rowOf(t, st, 1001)
	if r.Status != "present" || r.Source != "ibkr_tick_456" || r.Attempt != 1 || r.Issuer != "cik:0000320193" || r.Currency != "USD" {
		t.Fatalf("row %+v", r)
	}
	if r.NextAmount == nil || *r.NextAmount != "0.25" || r.Past12M == nil || *r.Past12M != "0.96" || r.Next12M == nil || *r.Next12M != "1" {
		t.Fatalf("amounts %+v", r)
	}
	if r.NextExDate == nil || r.NextExDate.Format("2006-01-02") != "2026-11-12" || r.SessionDay.Format("2006-01-02") != string(day) {
		t.Fatalf("dates %+v", r)
	}
}

func TestNoValueRow(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{2: {Status: broker.DividendNoValue}}}
	st := &memStore{}
	run(t, ref, &fakeRes{}, st, entry("cik:0001318605", "US", "TSLA", 2))
	r := rowOf(t, st, 1002)
	if r.Status != "no_value" || r.Source != "ibkr_tick_456" || r.NextExDate != nil || r.NextAmount != nil || r.ErrorCode != "" {
		t.Fatalf("row %+v", r)
	}
}

func TestTimeoutIsErrorRow(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{
		3: {Status: broker.DividendTimeout}, 4: {Status: broker.DividendError, ErrorCode: 200},
	}}
	st := &memStore{}
	res := run(t, ref, &fakeRes{}, st, entry("a", "US", "A", 3), entry("b", "US", "B", 4))
	if a := rowOf(t, st, 1003); a.Status != "error" || a.ErrorCode != "timeout" {
		t.Fatalf("timeout row %+v", a)
	}
	if b := rowOf(t, st, 1004); b.Status != "error" || b.ErrorCode != "ibkr_200" {
		t.Fatalf("error row %+v", b)
	}
	if res.Returned != 2 || res.Usable != 0 {
		t.Fatalf("result %+v", res)
	}
}

func TestUnpassedMarketIsUnsupported(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: {Status: broker.DividendNoValue}}}
	st := &memStore{}
	j, err := newWith(ref, &fakeRes{}, st, st, []jobs.WatchEntry{entry("a", "US", "A", 1), entry("edinet:E1", "JP", "7203", 5)},
		fixedClock{}, []string{"US"})
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), day, nil); res.Err != nil {
		t.Fatal(res.Err)
	}
	r := rowOf(t, st, 1005)
	if r.Status != "unsupported" || r.Source != "none" {
		t.Fatalf("row %+v", r)
	}
	if len(ref.asked) != 1 || len(ref.asked[0]) != 1 || ref.asked[0][0].ConID != 1 {
		t.Fatalf("asked %+v", ref.asked)
	}
}

func TestOneStockFailsOthersWritten(t *testing.T) {
	t.Parallel()
	ok := broker.DividendProjection{Status: broker.DividendOK, NextExDate: str("20261112"), NextAmount: f64(1)}
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: ok, 2: ok, 3: ok}}
	st := &memStore{}
	res := run(t, ref, &fakeRes{fail: map[int64]bool{2: true}}, st,
		entry("a", "US", "A", 1), entry("b", "US", "B", 2), entry("c", "US", "C", 3))
	if res.Returned != 3 {
		t.Fatalf("result %+v", res)
	}
	if rowOf(t, st, 1001).Status != "present" || rowOf(t, st, 1003).Status != "present" {
		t.Fatalf("rows %+v", st.rows)
	}
	// the failed resolve is an error row keyed by the con id; the stock is not asked.
	if r := rowOf(t, st, 2); r.Status != "error" || r.ErrorCode != "resolve_failed" || r.Source != "none" {
		t.Fatalf("row %+v", r)
	}
	if len(ref.asked) != 1 || len(ref.asked[0]) != 2 {
		t.Fatalf("asked %+v", ref.asked)
	}
}

func TestRowKeyedByResolvedInstrument(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{42: {Status: broker.DividendNoValue}}}
	res := &fakeRes{}
	st := &memStore{}
	run(t, ref, res, st, entry("a", "US", "A", 42))
	if len(st.rows) != 1 || st.rows[0].InstrumentID != 1042 {
		t.Fatalf("rows %+v", st.rows)
	}
	if len(res.keys) == 0 || res.keys[0] != (store.Identifier{Scheme: "ibkr_conid", Value: "42"}) {
		t.Fatalf("keys %+v", res.keys)
	}
}

func TestWholeCallFailureIsErrorRows(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{err: errors.New("socket closed")}
	st := &memStore{}
	run(t, ref, &fakeRes{}, st, entry("a", "US", "A", 1))
	if r := rowOf(t, st, 1001); r.Status != "error" || r.ErrorCode != "request_failed" {
		t.Fatalf("row %+v", r)
	}
}

func TestBadConstruction(t *testing.T) {
	t.Parallel()
	st := &memStore{}
	if _, err := New(&fakeRef{}, &fakeRes{}, st, st, []jobs.WatchEntry{entry("a", "US", "A", 1), entry("a", "US", "B", 2)}, fixedClock{}); err == nil {
		t.Fatal("duplicate issuer accepted")
	}
	if _, err := New(nil, &fakeRes{}, st, st, nil, fixedClock{}); err == nil {
		t.Fatal("nil client accepted")
	}
}

func TestEmbeddedManifest(t *testing.T) {
	t.Parallel()
	st := &memStore{}
	var newJob NewDividendCaptureFunc = New
	j, err := newJob(&fakeRef{}, &fakeRes{}, st, st, nil, fixedClock{})
	if err != nil || j.ID() != "dividend_capture" {
		t.Fatalf("%v %v", j, err)
	}
}

func TestRunWithoutUpserterIsRetryable(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: {Status: broker.DividendNoValue}}}
	st := &memStore{}
	j, err := New(ref, &fakeRes{}, plainStore{st}, st, []jobs.WatchEntry{entry("a", "US", "A", 1)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), day, nil)
	var te contracts.TypedError
	if !errors.As(res.Err, &te) || te.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "UpsertDividends") {
		t.Fatalf("err %v", res.Err)
	}
	if len(ref.asked) != 0 || len(st.rows) != 0 {
		t.Fatalf("asked %d rows %d", len(ref.asked), len(st.rows))
	}
}

func TestPastDayIsAnswerDayAndUsesReplay(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{
		1: {Status: broker.DividendOK, NextExDate: str("20261112"), NextAmount: f64(0.25)},
	}}
	st := &memStore{}
	j, err := New(ref, &fakeRes{}, st, st, []jobs.WatchEntry{entry("cik:0000320193", "US", "AAPL", 1)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-11-06", nil)
	if res.Err != nil || len(ref.asked) != 1 || len(st.rows) != 1 {
		t.Fatalf("past day: result %+v, asked %d, rows %d", res, len(ref.asked), len(st.rows))
	}
	if got := st.rows[0].SessionDay.Format("2006-01-02"); got != "2026-11-06" {
		t.Fatalf("SessionDay = %s, want the day given", got)
	}
	if len(st.calls) != 1 || !st.calls[0].replay || st.calls[0].by != "dividend_capture:2026-11-06" || st.calls[0].n != 1 {
		t.Fatalf("calls %+v", st.calls)
	}
	// the clock's day is not a replay.
	st2 := &memStore{}
	j2, _ := New(ref, &fakeRes{}, st2, st2, []jobs.WatchEntry{entry("a", "US", "A", 1)}, fixedClock{})
	j2.Run(context.Background(), day, nil)
	if len(st2.calls) != 1 || st2.calls[0].replay || st2.calls[0].by != "dividend_capture:2026-11-09" {
		t.Fatalf("calls %+v", st2.calls)
	}
}

func TestSecondKeyOfOneInstanceDoesNotCallBroker(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: {Status: broker.DividendNoValue}, 2: {Status: broker.DividendNoValue}}}
	st := &memStore{}
	j, err := New(ref, &fakeRes{}, st, st, []jobs.WatchEntry{entry("a", "US", "A", 1), entry("b", "US", "B", 2)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	r1 := j.Run(context.Background(), day, []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: "a"}})
	r2 := j.Run(context.Background(), day, []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: "b"}})
	if len(ref.asked) != 1 || len(st.rows) != 2 || len(st.calls) != 2 || r1 != r2 || r1.Returned != 2 {
		t.Fatalf("asked %d rows %d calls %d results %+v %+v", len(ref.asked), len(st.rows), len(st.calls), r1, r2)
	}
	// a new instance asks again and replaces the rows by key.
	j2, _ := New(ref, &fakeRes{}, st, st, []jobs.WatchEntry{entry("a", "US", "A", 1), entry("b", "US", "B", 2)}, fixedClock{})
	j2.Run(context.Background(), day, nil)
	if len(ref.asked) != 2 || len(st.rows) != 2 {
		t.Fatalf("asked %d rows %d", len(ref.asked), len(st.rows))
	}
}

func TestFailedFirstKeyIsMemoisedNoBrokerCall(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: {Status: broker.DividendNoValue}}}
	st := &memStore{fail: map[int]bool{1001: true}}
	j, err := New(ref, &fakeRes{}, st, st, []jobs.WatchEntry{entry("a", "US", "A", 1), entry("b", "US", "B", 2)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	r1 := j.Run(context.Background(), day, nil)
	var te contracts.TypedError
	if !errors.As(r1.Err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("first err %v", r1.Err)
	}
	r2 := j.Run(context.Background(), day, nil)
	if r2.Err == nil || r2.Err.Error() != r1.Err.Error() || len(ref.asked) != 1 {
		t.Fatalf("second %+v, asked %d", r2, len(ref.asked))
	}
}

func TestRefusedRowDoesNotStopOthers(t *testing.T) {
	t.Parallel()
	ok := broker.DividendProjection{Status: broker.DividendOK, NextExDate: str("20261112"), NextAmount: f64(1)}
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: ok, 2: ok, 3: ok}}
	st := &memStore{fail: map[int]bool{1002: true}}
	res := run(t, ref, &fakeRes{}, st, entry("a", "US", "A", 1), entry("b", "US", "B", 2), entry("c", "US", "C", 3))
	if res.Err == nil || res.Returned != 2 || res.Usable != 2 || len(st.rows) != 2 || len(st.calls) != 3 {
		t.Fatalf("result %+v rows %d calls %d", res, len(st.rows), len(st.calls))
	}
}

func TestPresentAndResolveFailedRowsReachUpserter(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{
		1: {Status: broker.DividendOK, NextExDate: str("20261112"), NextAmount: f64(0.25)},
	}}
	st := &memStore{}
	run(t, ref, &fakeRes{fail: map[int64]bool{2: true}}, st, entry("a", "US", "A", 1), entry("b", "US", "B", 2))
	if r := rowOf(t, st, 1001); r.Status != "present" || r.NextAmount == nil || *r.NextAmount != "0.25" {
		t.Fatalf("present row %+v", r)
	}
	if r := rowOf(t, st, 2); r.Status != "error" || r.ErrorCode != "resolve_failed" {
		t.Fatalf("resolve_failed row %+v", r)
	}
	for _, c := range st.calls {
		if c.by != "dividend_capture:2026-11-09" || c.replay {
			t.Fatalf("call %+v", c)
		}
	}
}
