package dividendcapture

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
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
	mu   sync.Mutex
	rows []contracts.DividendExpectation
	fail map[int]bool
}

func (m *memStore) InsertDividend(_ context.Context, r contracts.DividendExpectation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail[r.InstrumentID] {
		return errors.New("db down")
	}
	m.rows = append(m.rows, r)
	return nil
}

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

func TestSecondRunWritesNoRow(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{1: {Status: broker.DividendNoValue}}}
	st := &memStore{}
	e := entry("a", "US", "A", 1)
	run(t, ref, &fakeRes{}, st, e)
	run(t, ref, &fakeRes{}, st, e)
	if len(st.rows) != 1 || len(ref.asked) != 1 {
		t.Fatalf("rows %d, calls %d", len(st.rows), len(ref.asked))
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

func TestPastDayRunsClockDay(t *testing.T) {
	t.Parallel()
	ref := &fakeRef{by: map[int64]broker.DividendProjection{
		1: {Status: broker.DividendOK, NextExDate: str("20261112"), NextAmount: f64(0.25)},
	}}
	rs := &fakeRes{}
	st := &memStore{}
	j, err := New(ref, rs, st, st, []jobs.WatchEntry{entry("cik:0000320193", "US", "AAPL", 1)}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), "2026-11-06", nil)
	if res.Err != nil || len(ref.asked) != 1 || len(st.rows) != 1 {
		t.Fatalf("past day: result %+v, asked %d, rows %d", res, len(ref.asked), len(st.rows))
	}
	if got := st.rows[0].SessionDay.Format("2006-01-02"); got != string(day) {
		t.Fatalf("SessionDay = %s, want the clock's date %s", got, day)
	}
}
