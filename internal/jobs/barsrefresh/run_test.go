package barsrefresh

import (
	"context"
	"errors"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
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

// memBars is a writer and a reader on a map by key (instrument, basis, day). UpsertPriceBars keeps
// the rules of the store: a new key is inserted (adjust_epoch 1, as the column default), a row that
// differs in a number or the currency is replaced and an equal one (100.50 against 100.5) is not.
type memBars struct {
	mu    sync.Mutex
	rows  []contracts.PriceBar
	calls [][]contracts.PriceBar // the rows of every UpsertPriceBars call
	by    []string               // the writtenBy of every call
}

func (m *memBars) InsertPriceBars(_ context.Context, rows []contracts.PriceBar) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows = append(m.rows, rows...)
	return len(rows), nil
}

func sameNumber(a, b string) bool {
	x, ok1 := new(big.Rat).SetString(a)
	y, ok2 := new(big.Rat).SetString(b)
	return ok1 && ok2 && x.Cmp(y) == 0
}

func sameVolume(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return sameNumber(*a, *b)
}

func (m *memBars) UpsertPriceBars(_ context.Context, rows []contracts.PriceBar, writtenBy string) (bar.BarWriteCount, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, append([]contracts.PriceBar(nil), rows...))
	m.by = append(m.by, writtenBy)
	var n bar.BarWriteCount
	for _, r := range rows {
		at := -1
		for i, o := range m.rows {
			if o.InstrumentID == r.InstrumentID && o.Basis == r.Basis && o.SessionDay.Equal(r.SessionDay.Time) {
				at = i
			}
		}
		switch {
		case at < 0:
			r.AdjustEpoch = 1
			m.rows = append(m.rows, r)
			n.Written++
		case sameNumber(m.rows[at].Open, r.Open) && sameNumber(m.rows[at].High, r.High) && sameNumber(m.rows[at].Low, r.Low) &&
			sameNumber(m.rows[at].Close, r.Close) && sameVolume(m.rows[at].Volume, r.Volume) &&
			m.rows[at].Currency == r.Currency && m.rows[at].Source == r.Source:
			n.Unchanged++
		default:
			r.AdjustEpoch, r.ReceivedAt = m.rows[at].AdjustEpoch, m.rows[at].ReceivedAt
			m.rows[at] = r
			n.Written++
		}
	}
	return n, nil
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
	sort.Slice(out, func(i, j int) bool { return out[i].SessionDay.Before(out[j].SessionDay.Time) })
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

func TestStockKeyStoresTradesBarsAsStockInstrument(t *testing.T) {
	t.Parallel()
	var specs []store.InstrumentSpec
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "90.0000"))}
	st := &memBars{}
	j, err := New(&fakeBroker{listed: true}, h, specRecorder{specs: &specs}, st, st, nil, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	j.(*job).m.AdjustedCadenceDays = 0
	res := j.Run(context.Background(), runDay, keysOf("stock:XLE"))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(h.queries) != 1 || h.queries[0].Contract.SecType != "STK" || h.queries[0].Contract.Symbol != "XLE" || h.queries[0].Kind != broker.BarTrades {
		t.Fatalf("queries %+v", h.queries)
	}
	if len(specs) != 1 || specs[0].Type != "stock" || specs[0].Name != "XLE" {
		t.Fatalf("specs %+v", specs)
	}
	if rows := st.epoch(1); len(rows) != 1 || rows[0].Basis != "trades" {
		t.Fatalf("rows %+v", st.rows)
	}
}

// specRecorder is a resolver that keeps the spec of every instrument it creates.
type specRecorder struct{ specs *[]store.InstrumentSpec }

func (s specRecorder) ResolveInstrument(_ context.Context, k store.Identifier, spec store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	*s.specs = append(*s.specs, spec)
	return strconv.ParseInt(k.Value, 10, 64)
}

// unlistedBroker finds every contract except the symbol it does not list.
type unlistedBroker struct {
	broker.Broker
	unlisted string
}

func (u unlistedBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	c.ConID = 9000 + int64(len(c.Symbol))*10 + int64(c.Symbol[len(c.Symbol)-1]-'A')
	return broker.Resolved{Listed: c.Symbol != u.unlisted, Contract: c}, nil
}

func TestUnresolvedStockFailsOnlyItsOwnKey(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "90.0000"))}
	st := &memBars{}
	j, err := New(unlistedBroker{unlisted: "XLK"}, h, fakeResolver{}, st, st, nil, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	j.(*job).m.AdjustedCadenceDays = 0
	res := j.Run(context.Background(), runDay, keysOf("stock:XLE", "stock:XLK", "stock:XLU"))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("err %v", res.Err)
	}
	if len(st.rows) != 2 || res.Usable != 2 || len(h.queries) != 2 {
		t.Fatalf("rows %+v result %+v queries %d", st.rows, res, len(h.queries))
	}
}

// distinctBroker gives each symbol its own contract id, so no two stocks share an instrument.
type distinctBroker struct {
	unlistedBroker
	ids map[string]int64
}

func (d distinctBroker) ResolveContract(ctx context.Context, c broker.Contract) (broker.Resolved, error) {
	got, err := d.unlistedBroker.ResolveContract(ctx, c)
	if err != nil {
		return got, err
	}
	if _, ok := d.ids[c.Symbol]; !ok {
		d.ids[c.Symbol] = 5000 + int64(len(d.ids))
	}
	got.Contract.ConID = d.ids[c.Symbol]
	return got, nil
}

func TestUnresolvedSMHKeepsRun(t *testing.T) {
	t.Parallel()
	h := &fakeHistory{answer: fixed(mkbar("2026-11-09", "90.0000"))}
	st := &memBars{}
	j, err := New(distinctBroker{unlistedBroker{unlisted: "SMH"}, map[string]int64{}}, h, fakeResolver{}, st, st, nil, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	j.(*job).m.AdjustedCadenceDays = 0
	j.(*job).m.BackfillNamesPerRun = 100
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	var stocks []contracts.ExpectedKey
	for _, k := range keys {
		if strings.HasPrefix(k.SubjectKey, "stock:") {
			stocks = append(stocks, k)
		}
	}
	if len(stocks) != 17 {
		t.Fatalf("stock keys %d", len(stocks))
	}
	res := j.Run(context.Background(), runDay, stocks)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent ||
		!strings.Contains(res.Err.Error(), "stock:SMH") || !strings.Contains(res.Err.Error(), "not listed") {
		t.Fatalf("err %v", res.Err)
	}
	if res.Usable != 16 || len(h.queries) != 16 {
		t.Fatalf("result %+v queries %d", res, len(h.queries))
	}
}
