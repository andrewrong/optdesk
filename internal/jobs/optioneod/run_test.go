package optioneod

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

const runToml = `
id = "option_eod"
version = 1
max_expiries = 1
max_strikes_per_side = 1
chunk = 2
budget_minutes = 8
[[underlying]]
symbol = "TEST"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1000001
mic = "XNYS"
calendar_id = "XNYS"
price_increment = "0.01"
lot_size = "1"
[[underlying]]
symbol = "OTHER"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1000002
`

var testKey = contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "TEST"}

const testDay = contracts.Day("2026-11-06")

type snapCall struct {
	kind broker.Kind
	cs   []broker.Contract
}

type fakeBroker struct {
	price       *float64
	noUndPrice  bool // option quotes carry no UndPrice
	chains      []broker.Chain
	notListed   map[string]bool // by "strike right"
	status      map[int64]broker.Status
	snaps       []snapCall
	resolves    int
	resolved    []broker.Contract // every contract given to ResolveContract
	afterSnap   func(optionCalls int)
	snapErr     error
	chainErr    error
	resolveErr  error
	optionCalls int
	full        bool                  // option quotes carry volume and the greeks
	onChains    func()                // called by ListChains
	noOI        bool                  // option quotes carry no open_interest
	missing     []broker.MissingField // Missing of every option quote
}

func num(v float64) *float64 { return &v }

func newBroker() *fakeBroker {
	return &fakeBroker{
		price: num(100),
		chains: []broker.Chain{{Exchange: "SMART", TradingClass: "TEST", Multiplier: "100",
			Expirations: []string{"20261105", "20261106", "20261113"}, Strikes: []string{"95", "100", "105", "200"}}},
		notListed: map[string]bool{},
		status:    map[int64]broker.Status{},
	}
}

func conID(c broker.Contract) int64 {
	s, _ := strconv.ParseFloat(c.Strike, 64)
	id := int64(s) * 10
	if c.Right == "P" {
		id++
	}
	return id
}

func (b *fakeBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	if b.onChains != nil {
		b.onChains()
	}
	return b.chains, b.chainErr
}

func (b *fakeBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	b.resolves++
	b.resolved = append(b.resolved, c)
	if b.resolveErr != nil {
		return broker.Resolved{}, b.resolveErr
	}
	if b.notListed[c.Strike+" "+c.Right] {
		return broker.Resolved{}, nil
	}
	c.ConID = conID(c)
	return broker.Resolved{Listed: true, Contract: c}, nil
}

func (b *fakeBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	b.snaps = append(b.snaps, snapCall{kind, cs})
	if b.snapErr != nil {
		return nil, b.snapErr
	}
	if kind == broker.KindSnapshot {
		return []broker.Quote{{Contract: cs[0], Last: b.price}}, nil
	}
	b.optionCalls++
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = broker.Quote{Contract: c, Status: b.status[c.ConID], Generation: 7, MarketDataType: 1,
			ReceivedAt: time.Date(2026, 11, 6, 22, 0, 0, 0, time.UTC), Bid: num(1), Ask: num(2), Last: num(1.5), UndPrice: num(100)}
		if b.full {
			out[i].Volume, out[i].OpenInterest, out[i].ImpliedVol = num(3), num(4), num(0.2)
			out[i].Delta, out[i].Gamma, out[i].Theta, out[i].Vega = num(0.5), num(0.1), num(-0.1), num(0.2)
		}
		if b.noOI {
			out[i].OpenInterest = nil
		}
		out[i].Missing = b.missing
		if b.noUndPrice {
			out[i].UndPrice = nil
		}
	}
	if b.afterSnap != nil {
		b.afterSnap(b.optionCalls)
	}
	return out, nil
}

func (b *fakeBroker) Close() error { return nil }

func (b *fakeBroker) calls() int { return len(b.snaps) + b.resolves }

type resolveCall struct {
	key   store.Identifier
	spec  store.InstrumentSpec
	extra []store.Identifier
}

type fakeResolver struct {
	calls []resolveCall
	err   error
}

func (r *fakeResolver) ResolveInstrument(_ context.Context, key store.Identifier, spec store.InstrumentSpec, extra []store.Identifier) (int64, error) {
	r.calls = append(r.calls, resolveCall{key, spec, extra})
	if r.err != nil {
		return 0, r.err
	}
	n, _ := strconv.ParseInt(key.Value, 10, 64)
	return n + 5000000, nil
}

type fakeWriter struct {
	calls [][]contracts.OptionEod
	err   error
}

func (w *fakeWriter) InsertOptionEod(_ context.Context, rows []contracts.OptionEod) error {
	if w.err != nil {
		return w.err
	}
	w.calls = append(w.calls, rows)
	return nil
}

func (w *fakeWriter) all() []contracts.OptionEod {
	var out []contracts.OptionEod
	for _, c := range w.calls {
		out = append(out, c...)
	}
	return out
}

type kindErr struct{ kind contracts.ErrorKind }

func (e *kindErr) Error() string             { return "boom" }
func (e *kindErr) Kind() contracts.ErrorKind { return e.kind }

type rig struct {
	b   *fakeBroker
	res *fakeResolver
	w   *fakeWriter
	clk *clockwork.FakeClock
	job *job
}

func newRig(t *testing.T, at string) *rig {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now, err := time.ParseInLocation("2006-01-02 15:04", at, loc)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{b: newBroker(), res: &fakeResolver{}, w: &fakeWriter{}, clk: clockwork.NewFakeClockAt(now)}
	r.job, err = build([]byte(runToml), r.b, r.res, r.w, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (r *rig) run(day contracts.Day, k contracts.ExpectedKey) contracts.JobResult {
	return r.job.Run(context.Background(), day, []contracts.ExpectedKey{k})
}

func kindOf(res contracts.JobResult) contracts.ErrorKind {
	if res.Err == nil {
		return ""
	}
	return res.Err.Kind()
}

func TestRunWritesRows(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 17:00")
	r.b.status[950] = broker.StatusNoData
	res := r.run(testDay, testKey)
	if res.Err != nil || res.Returned != 4 || res.Usable != 3 {
		t.Fatalf("result %+v", res)
	}
	var shape []int
	for _, s := range r.b.snaps {
		shape = append(shape, int(s.kind)*10+len(s.cs))
	}
	if want := []int{21, 2, 2}; !reflect.DeepEqual(shape, want) { // kind*10+contracts: snapshot of 1, then two option chunks of 2
		t.Fatalf("snapshot calls %v, want %v", shape, want)
	}
	if len(r.w.calls) != 2 {
		t.Fatalf("want one write per chunk, got %d", len(r.w.calls))
	}
	want := uuid.NewSHA1(uuid.NameSpaceOID, []byte("2026-11-06|obs.option_eod|TEST|2026-11-06T22:00:00Z")).String()
	rows := r.w.all()
	wantIDs := []int{5000950, 5000951, 5001000, 5001001}
	for i, row := range rows {
		if row.CaptureID != want || row.SnapshotKind != "session" || row.InstrumentID != wantIDs[i] || row.UnderlyingID != 6000001 ||
			row.Generation != 7 || row.SessionDay.Format(time.DateOnly) != "2026-11-06" || row.Currency != "USD" {
			t.Fatalf("row %d = %+v", i, row)
		}
	}
	if rows[0].Status != 2 || rows[1].Status != 1 {
		t.Fatalf("statuses %d %d", rows[0].Status, rows[1].Status)
	}
	checkResolves(t, r.res.calls)
}

// checkResolves wants the underlying and then four options, an option with the osi identifier of 21 characters.
func checkResolves(t *testing.T, calls []resolveCall) {
	t.Helper()
	// the underlying plus four options were resolved; an option carries the osi identifier of 21 characters
	if len(calls) != 5 {
		t.Fatalf("resolver calls %d", len(calls))
	}
	first := calls[1]
	wantOpt := resolveCall{key: store.Identifier{Scheme: "ibkr_conid", Value: "950"},
		spec: store.InstrumentSpec{Type: "option", Name: "TEST 20261106 C 95", Currency: "USD", MIC: "XNYS", CalendarID: "XNYS", Status: "active",
			PriceIncrement: "0.01", LotSize: "100"},
		extra: []store.Identifier{{Scheme: "osi", Value: "TEST  261106C00095000"}}}
	if !reflect.DeepEqual(first, wantOpt) || len(first.extra[0].Value) != 21 {
		t.Fatalf("option resolve %+v", first)
	}
	wantUnd := resolveCall{key: store.Identifier{Scheme: "ibkr_conid", Value: "1000001"},
		spec: store.InstrumentSpec{Type: "stock", Name: "TEST", Currency: "USD", MIC: "XNYS", CalendarID: "XNYS", Status: "active",
			PriceIncrement: "0.01", LotSize: "1"}}
	if !reflect.DeepEqual(calls[0], wantUnd) {
		t.Fatalf("underlying resolve %+v", calls[0])
	}
}

func TestCaptureIDStable(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 17:00")
	first := r.run(testDay, testKey)
	second := r.run(testDay, testKey) // same start instant: the fake clock did not move
	r.b.chains = append(r.b.chains, broker.Chain{Exchange: "SMART", TradingClass: "OTHER", Multiplier: "100",
		Expirations: r.b.chains[0].Expirations, Strikes: r.b.chains[0].Strikes})
	other := r.run(testDay, contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "OTHER"})
	if first.Err != nil || second.Err != nil || other.Err != nil {
		t.Fatal(first, second, other)
	}
	rows := r.w.all()
	if len(rows) != 12 {
		t.Fatalf("rows %d", len(rows))
	}
	if rows[0].CaptureID != rows[4].CaptureID {
		t.Fatal("two runs of one key with one start must share the capture id")
	}
	if rows[8].CaptureID == rows[0].CaptureID {
		t.Fatal("another underlying must give another capture id")
	}
	start := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	if captureID("2026-11-09", testKey, start) == captureID(testDay, testKey, start) {
		t.Fatal("another day must give another capture id")
	}
	if captureID(testDay, testKey, start) != captureID(testDay, testKey, start) {
		t.Fatal("one (day, key, start) must give one capture id")
	}
	if captureID(testDay, testKey, start) == captureID(testDay, testKey, start.Add(time.Minute)) {
		t.Fatal("another start must give another capture id")
	}
}

func TestTwoRunsTwoCaptures(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 11:00")
	if res := r.run(testDay, testKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	r.clk.Advance(30 * time.Minute)
	if res := r.run(testDay, testKey); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(r.w.calls) != 4 {
		t.Fatalf("writes %d", len(r.w.calls))
	}
	ids := map[string]int{}
	for _, row := range r.w.all() {
		ids[row.CaptureID]++
	}
	if len(ids) != 2 {
		t.Fatalf("capture ids %v, want two", ids)
	}
	for id, n := range ids {
		if n != 4 {
			t.Fatalf("capture %s holds %d rows, want 4", id, n)
		}
	}
}

func TestNotListed(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 17:00")
	r.b.notListed["95 P"] = true
	res := r.run(testDay, testKey)
	if res.Err != nil || res.Returned != 3 || res.Usable != 3 {
		t.Fatalf("result %+v", res)
	}
	asked := 0
	for _, s := range r.b.snaps[1:] {
		for _, c := range s.cs {
			asked++
			if c.Strike == "95" && c.Right == "P" {
				t.Fatal("a pair that is not listed must not be asked for a quote")
			}
		}
	}
	if asked != 3 || len(r.w.all()) != 3 {
		t.Fatalf("asked %d wrote %d", asked, len(r.w.all()))
	}
}

func TestBudget(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 17:00")
	r.b.afterSnap = func(n int) {
		if n == 1 {
			r.clk.Advance(9 * time.Minute)
		}
	}
	res := r.run(testDay, testKey)
	if kindOf(res) != contracts.Retryable || res.Returned != 2 || res.Usable != 2 {
		t.Fatalf("result %+v", res)
	}
	if r.b.optionCalls != 1 || len(r.w.calls) != 1 || len(r.w.all()) != 2 {
		t.Fatalf("option snapshots %d, writes %d", r.b.optionCalls, len(r.w.calls))
	}
}

func TestPastDayRefuses(t *testing.T) {
	t.Parallel()
	const wantText = "option_eod: day 2026-11-05 is not the current day; a past option chain cannot be collected"
	check := func(t *testing.T, r *rig, day contracts.Day, k contracts.ExpectedKey) {
		t.Helper()
		res := r.run(day, k)
		if kindOf(res) != contracts.Permanent || res.Err.Error() != wantText {
			t.Fatalf("result %+v", res)
		}
		if r.b.calls() != 0 || r.b.resolves != 0 || r.res.calls != nil && len(r.res.calls) != 0 || len(r.w.calls) != 0 {
			t.Fatalf("calls %d, resolves %d, writes %d", r.b.calls(), r.b.resolves, len(r.w.calls))
		}
	}
	t.Run("US", func(t *testing.T) {
		t.Parallel()
		check(t, newRig(t, "2026-11-06 11:00"), "2026-11-05", testKey)
	})
	t.Run("US future day", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 11:00")
		if res := r.run("2026-11-07", testKey); kindOf(res) != contracts.Permanent || r.b.calls() != 0 {
			t.Fatalf("result %+v", res)
		}
	})
	t.Run("JP", func(t *testing.T) {
		t.Parallel()
		check(t, newJPRig(t, "2026-11-06 10:30"), "2026-11-05", jpKey)
	})
	t.Run("current day runs at any time", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 20:00")
		if res := r.run(testDay, testKey); res.Err != nil || len(r.w.calls) == 0 {
			t.Fatalf("result %+v", res)
		}
	})
}

// wantFailure runs the key and wants a result of the kind, no usable row and no write.
func wantFailure(t *testing.T, r *rig, kind contracts.ErrorKind, usable int) {
	t.Helper()
	res := r.run(testDay, testKey)
	if kindOf(res) != kind || res.Usable != usable || len(r.w.calls) != 0 {
		t.Fatalf("result %+v, writes %d, want kind %s", res, len(r.w.calls), kind)
	}
}

func TestErrors(t *testing.T) {
	t.Parallel()
	retry := &kindErr{contracts.Retryable}
	t.Run("broker snapshot", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.snapErr = retry
		wantFailure(t, r, contracts.Retryable, 0)
	})
	t.Run("broker chains", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.chainErr = retry
		wantFailure(t, r, contracts.Retryable, 0)
	})
	t.Run("broker resolve contract", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.resolveErr = retry
		wantFailure(t, r, contracts.Retryable, 0)
	})
	t.Run("resolver", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.res.err = retry
		wantFailure(t, r, contracts.Retryable, 0)
		if r.b.optionCalls != 0 {
			t.Fatal("no quote is asked after a resolver error")
		}
	})
	t.Run("untyped broker error is permanent", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.chainErr = errors.New("plain")
		wantFailure(t, r, contracts.Permanent, 0)
	})
	t.Run("writer", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.w.err = errors.New("disk")
		wantFailure(t, r, contracts.Retryable, 0)
	})
	t.Run("no data and timeout still write", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.status[950] = broker.StatusNoData
		r.b.status[951] = broker.StatusTimeout
		res := r.run(testDay, testKey)
		rows := r.w.all()
		if res.Err != nil || res.Returned != 4 || res.Usable != 2 || rows[0].Status != 2 || rows[1].Status != 5 {
			t.Fatalf("%+v %+v", res, rows)
		}
		if code := rows[1].ErrorCode; code == nil || *code != "TIMEOUT" {
			t.Fatalf("error code %v", code)
		}
	})
	t.Run("no price is retryable without a write", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.price = nil
		wantFailure(t, r, contracts.Retryable, 0)
	})
	t.Run("empty selection is permanent", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, "2026-11-06 17:00")
		r.b.chains = []broker.Chain{{Expirations: []string{"20261105"}, Strikes: []string{"100"}}}
		wantFailure(t, r, contracts.Permanent, 0)
	})
}

func TestUnderlyingPriceFallback(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 17:00")
	r.b.price = nil
	// Close and UndPrice come back through the underlying snapshot when Last is absent
	snap := &closeOnly{fakeBroker: r.b}
	r.job.b = snap
	if res := r.run(testDay, testKey); res.Err != nil || res.Returned != 4 {
		t.Fatalf("%+v", res)
	}
}

func TestRunStoresFetchedUnderlyingPrice(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		noUndPrice bool
		want       float64
	}{{"quote without UndPrice takes the fetched price", true, 100}, {"quote with UndPrice keeps its own", false, 100}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, "2026-11-06 17:00")
			r.b.price = num(101)
			r.b.noUndPrice = tc.noUndPrice
			if res := r.run(testDay, testKey); res.Err != nil || res.Returned != 4 {
				t.Fatalf("%+v", res)
			}
			want := tc.want
			if tc.noUndPrice {
				want = 101
			}
			for i, row := range r.w.all() {
				if row.UnderlyingPrice == nil || *row.UnderlyingPrice != want {
					t.Fatalf("row %d underlying_price %v, want %v", i, row.UnderlyingPrice, want)
				}
			}
		})
	}
}

type closeOnly struct{ *fakeBroker }

func (c *closeOnly) Snapshot(ctx context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	if kind == broker.KindSnapshot {
		c.snaps = append(c.snaps, snapCall{kind, cs})
		return []broker.Quote{{Close: num(100)}}, nil
	}
	return c.fakeBroker.Snapshot(ctx, kind, cs)
}

func TestNewRequiresPorts(t *testing.T) {
	t.Parallel()
	if _, err := New(nil, nil, nil, nil); err == nil {
		t.Fatal("want error")
	}
	b := newBroker()
	j, err := New(b, &fakeResolver{}, &fakeWriter{}, clockwork.NewFakeClock())
	if err != nil || j.ID() != "option_eod" {
		t.Fatal(j, err)
	}
	w, ok := j.(interface {
		Window() (string, time.Duration, time.Duration)
	})
	if !ok {
		t.Fatal("job must carry Window")
	}
	if cal, open, closeOff := w.Window(); cal != "XNYS" || open != 90*time.Minute || closeOff != 0 {
		t.Fatal(cal, open, closeOff)
	}
}

// spyRig is a rig whose first underlying is SPY, with no trading_class in the manifest.
func spyRig(t *testing.T, chains []broker.Chain) *rig {
	t.Helper()
	r := newRig(t, "2026-11-06 17:00")
	var err error
	r.job, err = build([]byte(strings.Replace(runToml, `symbol = "TEST"`, `symbol = "SPY"`, 1)), r.b, r.res, r.w, r.clk)
	if err != nil {
		t.Fatal(err)
	}
	r.b.chains = chains
	return r
}

func TestChainPrefersSymbolClassOverFirst(t *testing.T) {
	t.Parallel()
	r := spyRig(t, []broker.Chain{
		{Exchange: "SMART", TradingClass: "2SPY", Multiplier: "100", Expirations: []string{"20261106"}, Strikes: []string{"95", "100", "105", "200"}},
		{Exchange: "SMART", TradingClass: "SPY", Multiplier: "100", Expirations: []string{"20261105", "20261106", "20261113"},
			Strikes: []string{"90", "95", "100", "105", "110", "200"}},
	})
	k := contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "SPY"}
	if res := r.run(testDay, k); res.Err != nil || res.Returned == 0 {
		t.Fatalf("%+v", res)
	}
	n := 0
	for _, s := range r.b.snaps {
		if s.kind != broker.KindSnapshot {
			for _, c := range s.cs {
				n++
				if c.TradingClass != "SPY" {
					t.Fatalf("snapshot contract %+v", c)
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no option contract snapshotted")
	}
	if len(r.b.resolved) == 0 {
		t.Fatal("no contract resolved")
	}
	for _, c := range r.b.resolved {
		if c.TradingClass != "SPY" {
			t.Fatalf("resolved contract %+v", c)
		}
	}
}

func TestChainNoSymbolClassFailsKey(t *testing.T) {
	t.Parallel()
	r := spyRig(t, []broker.Chain{{Exchange: "SMART", TradingClass: "2SPY", Multiplier: "100",
		Expirations: []string{"20261106"}, Strikes: []string{"95", "100", "105"}}})
	k := contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "SPY"}
	res := r.run(testDay, k)
	if kindOf(res) != contracts.Permanent || !strings.Contains(res.Err.Error(), "no chain of trading class SPY") {
		t.Fatalf("%+v", res)
	}
	for _, s := range r.b.snaps {
		if s.kind != broker.KindSnapshot {
			t.Fatalf("option snapshot called: %+v", s)
		}
	}
	if len(r.w.calls) != 0 {
		t.Fatal("rows written")
	}
}

type noSpace struct{ symbol.Translator }

func (noSpace) OSIRoot(s string) string { return strings.ReplaceAll(s, " ", "") }

func TestRootNilTranslator(t *testing.T) {
	t.Parallel()
	if got := (&job{}).root("BRK B"); got != "BRK B" {
		t.Fatalf("nil translator: got %q", got)
	}
	if got := (&job{sym: noSpace{}}).root("BRK B"); got != "BRKB" {
		t.Fatalf("translator: got %q", got)
	}
}
