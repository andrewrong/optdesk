package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

const liveDay = "2026-11-11"

type liveClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *liveClock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *liveClock) add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// liveCal answers a session of the day around 09:30 to 16:00 New York time; open false is a holiday.
type liveCal struct{ open bool }

func (c liveCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	if !c.open || id != "XNYS" {
		return clock.Session{CalendarID: id, Day: day}, nil
	}
	return clock.Session{CalendarID: id, Day: day, IsSession: true,
		OpenAt: time.Date(2026, 11, 11, 14, 30, 0, 0, time.UTC), CloseAt: time.Date(2026, 11, 11, 21, 0, 0, 0, time.UTC)}, nil
}

func (c liveCal) NextSession(context.Context, string, contracts.Day) (clock.Session, error) {
	return clock.Session{}, errors.New("unused")
}

type livePlan struct{ items []contracts.OptionOpenItem }

func (p livePlan) Plan(_ context.Context, _ contracts.Day, slot string, _ ...int64) ([]contracts.OptionOpenItem, error) {
	if slot != "0945" {
		return nil, nil
	}
	return p.items, nil
}

type liveInsts struct{}

func (liveInsts) Instruments(_ context.Context, ids []int64) ([]store.Instrument, error) {
	var out []store.Instrument
	for _, id := range ids {
		in := store.Instrument{ID: id, Identifiers: []store.Identifier{{Scheme: "ibkr_conid", Value: strconv.FormatInt(1000+id, 10)}}}
		in.Spec = store.InstrumentSpec{Name: "OPT", Currency: "USD"}
		switch id {
		case 1:
			in.Spec.Name = "AAPL"
		case 2:
			in.Spec.Name = "MSFT"
		}
		out = append(out, in)
	}
	return out, nil
}

type liveRefs struct{}

func (liveRefs) ListOptions(_ context.Context, u int64) ([]openquote.OptionRef, error) {
	var out []openquote.OptionRef
	for i := int64(0); i < 50; i++ {
		id := u*100 + i
		out = append(out, openquote.OptionRef{UnderlyingID: u, Currency: "USD", Exchange: "SMART", TradingClass: "X", Multiplier: "100",
			Expiry: "2026-12-18", Strike: strconv.FormatInt(100+i, 10), Right: "P", InstrumentID: id})
	}
	return out, nil
}

// liveItems are n planned contracts of underlying u: ids u*100 .. u*100+n-1, held class for the first.
func liveItems(u, n int) []contracts.OptionOpenItem {
	day, _ := time.Parse(time.DateOnly, liveDay)
	var out []contracts.OptionOpenItem
	for i := 0; i < n; i++ {
		class := "puts"
		if i == 0 {
			class = "held"
		}
		out = append(out, contracts.OptionOpenItem{UnderlyingID: u, InstrumentID: u*100 + i, Rank: i, Class: gen.OptionOpenItemClass(class),
			SessionDay: types.SerializableDate{Time: day}})
	}
	return out
}

type liveBroker struct {
	broker.Broker
	calls   atomic.Int32
	sizes   []int
	mu      sync.Mutex
	block   chan struct{}
	entered chan struct{}
	err     error
}

func (b *liveBroker) Snapshot(_ context.Context, _ broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	b.calls.Add(1)
	b.mu.Lock()
	b.sizes = append(b.sizes, len(cs))
	b.mu.Unlock()
	if b.entered != nil {
		b.entered <- struct{}{}
	}
	if b.block != nil {
		<-b.block
	}
	if b.err != nil {
		return nil, b.err
	}
	bid := 1.5
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		out[i] = broker.Quote{Contract: c, Status: broker.StatusOK, Bid: &bid, Ask: &bid, ReceivedAt: time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)}
	}
	return out, nil
}

type liveRig struct {
	l   *liveQuotes
	clk *liveClock
	brk *liveBroker
}

type storedRows []contracts.OptionOpen

func (s storedRows) OptionOpen(context.Context, contracts.Day) ([]contracts.OptionOpen, error) {
	return s, nil
}

func newLiveRig(t *testing.T, open bool, items []contracts.OptionOpenItem, at time.Time) *liveRig {
	t.Helper()
	r := &liveRig{clk: &liveClock{t: at}, brk: &liveBroker{}}
	l, err := newLive(liveDeps{
		Clock: r.clk, Calendar: liveCal{open: open}, Plan: livePlan{items}, Contracts: liveRefs{}, Insts: liveInsts{},
		Dial:  func(context.Context) (broker.Broker, error) { return r.brk, nil },
		After: func(time.Duration) <-chan time.Time { c := make(chan time.Time, 1); c <- time.Time{}; return c },
	})
	if err != nil {
		t.Fatal(err)
	}
	r.l = l
	return r
}

// 11:00 New York time, outside the pull window.
var liveNoon = time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)

func TestLiveRefreshOverlaysRowsWithTheRefreshSlot(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	stored := contracts.OptionOpen{InstrumentID: 102, Slot: "0945", ReceivedAt: liveNoon.Add(-time.Hour)}
	stored2 := contracts.OptionOpen{InstrumentID: 999, Slot: "0945", ReceivedAt: liveNoon.Add(-time.Hour)}
	r.l.d.Stored = storedRows{stored, stored2}
	a := r.l.refresh(t.Context(), []string{"aapl"})
	if a.Notice != "3 of 3 quotes fresh" || a.Fresh != 3 || a.Total != 3 {
		t.Fatalf("answer %+v", a)
	}
	rows, err := r.l.OptionOpen(t.Context(), liveDay)
	if err != nil || len(rows) != 4 {
		t.Fatalf("rows %d err %v", len(rows), err)
	}
	for _, row := range rows {
		want := openquote.SlotRefresh
		if row.InstrumentID == 999 {
			want = "0945"
		}
		if row.Slot != want {
			t.Errorf("instrument %d slot %q want %q", row.InstrumentID, row.Slot, want)
		}
	}
	if rows[0].InstrumentID != 100 || rows[3].InstrumentID != 999 {
		t.Errorf("not by instrument id: %d %d", rows[0].InstrumentID, rows[3].InstrumentID)
	}
}

func TestLiveRefreshReusesUnderSixtySeconds(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	r.l.refresh(t.Context(), []string{"AAPL"})
	r.clk.add(41 * time.Second)
	a := r.l.refresh(t.Context(), []string{"AAPL"})
	if a.Notice != "reused, 41 s old" || r.brk.calls.Load() != 1 {
		t.Fatalf("%+v calls %d", a, r.brk.calls.Load())
	}
	r.clk.add(20 * time.Second)
	r.l.refresh(t.Context(), []string{"AAPL"})
	if r.brk.calls.Load() != 2 {
		t.Fatalf("a 61 s old symbol was not asked again: %d", r.brk.calls.Load())
	}
}

func TestLiveRefreshMarketClosedAsksNothing(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, false, liveItems(1, 3), liveNoon)
	r.l.d.Stored = storedRows{{InstrumentID: 100, Slot: "0945", ReceivedAt: time.Date(2026, 11, 11, 14, 45, 0, 0, time.UTC)}}
	a := r.l.refresh(t.Context(), []string{"AAPL"})
	if a.Notice != "market closed; stored quotes of 09:45" || r.brk.calls.Load() != 0 {
		t.Fatalf("%+v calls %d", a, r.brk.calls.Load())
	}
	// after the close of an early-close day the same: the calendar's close is the end of the session
	r = newLiveRig(t, true, liveItems(1, 3), time.Date(2026, 11, 11, 21, 0, 0, 0, time.UTC))
	if a := r.l.refresh(t.Context(), []string{"AAPL"}); !strings.HasPrefix(a.Notice, "market closed") || r.brk.calls.Load() != 0 {
		t.Fatalf("%+v", a)
	}
}

func TestLiveRefreshNoPlanAsksNothing(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, nil, liveNoon)
	a := r.l.refresh(t.Context(), []string{"AAPL"})
	if a.Notice != "no plan today" || r.brk.calls.Load() != 0 {
		t.Fatalf("%+v calls %d", a, r.brk.calls.Load())
	}
}

func TestLiveRefreshAsksAtMostFortyContractsInChunksOfTwenty(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, append(liveItems(1, 30), liveItems(2, 30)...), liveNoon)
	a := r.l.refresh(t.Context(), []string{"AAPL", "MSFT"})
	if a.Total != 40 || len(r.brk.sizes) != 2 || r.brk.sizes[0] != 20 || r.brk.sizes[1] != 20 {
		t.Fatalf("%+v sizes %v", a, r.brk.sizes)
	}
}

func TestLiveRefreshInsidePullWindowAsksTenInOneChunk(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 30), time.Date(2026, 11, 11, 14, 50, 0, 0, time.UTC))
	a := r.l.refresh(t.Context(), []string{"AAPL"})
	if a.Total != 10 || len(r.brk.sizes) != 1 || r.brk.sizes[0] != 10 {
		t.Fatalf("%+v sizes %v", a, r.brk.sizes)
	}
}

func TestLivePullWindowIs0934To1010Eastern(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	cases := []struct {
		at   time.Time
		want bool
	}{
		{time.Date(2026, 11, 11, 14, 33, 0, 0, time.UTC), false},
		{time.Date(2026, 11, 11, 14, 34, 0, 0, time.UTC), true},
		{time.Date(2026, 11, 11, 15, 10, 0, 0, time.UTC), true},
		{time.Date(2026, 11, 11, 15, 11, 0, 0, time.UTC), false},
	}
	for _, c := range cases {
		r.clk.t = c.at
		if got := r.l.inPullWindow(r.clk.Now()); got != c.want {
			t.Errorf("%s: inPullWindow %v want %v", c.at.Format("15:04 UTC"), got, c.want)
		}
	}
}

func TestLiveRefreshHeldFirst(t *testing.T) {
	t.Parallel()
	items := liveItems(1, 50)
	items[0], items[49] = items[49], items[0] // the held item is the last of the plan
	items[0].Rank, items[49].Rank = 0, 49
	r := newLiveRig(t, true, items, liveNoon)
	var first int64
	r.l.d.Dial = func(context.Context) (broker.Broker, error) {
		return &firstAsked{liveBroker: r.brk, first: &first}, nil
	}
	r.l.refresh(t.Context(), []string{"AAPL"})
	if first != 1000+100 {
		t.Fatalf("first contract asked %d, want the held one", first)
	}
}

type firstAsked struct {
	*liveBroker
	first *int64
	once  sync.Once
}

func (f *firstAsked) Snapshot(ctx context.Context, k broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	f.once.Do(func() { *f.first = cs[0].ConID })
	return f.liveBroker.Snapshot(ctx, k, cs)
}

func TestLiveRefreshAtMostThirtyCallsAnHour(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	for i := 0; i < liveMaxPerHour; i++ {
		r.clk.add(61 * time.Second)
		r.l.refresh(t.Context(), []string{"AAPL"})
	}
	// 30 calls in 31 minutes: the 31st is refused until the first leaves the hour
	r.clk.add(61 * time.Second)
	a := r.l.refresh(t.Context(), []string{"AAPL"})
	if !strings.Contains(a.Notice, "at most 30") || r.brk.calls.Load() != 30 {
		t.Fatalf("%+v calls %d", a, r.brk.calls.Load())
	}
}

func TestLiveRefreshWaitsForARunningCall(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, append(liveItems(1, 3), liveItems(2, 3)...), liveNoon)
	r.brk.block, r.brk.entered = make(chan struct{}), make(chan struct{}, 4)
	done := make(chan liveAnswer)
	go func() { done <- r.l.refresh(t.Context(), []string{"AAPL"}) }()
	<-r.brk.entered
	// other symbols: the wait ends at 3 s (the fake After fires at once) with the busy notice, nothing asked
	if a := r.l.refresh(t.Context(), []string{"MSFT"}); a.Notice != "another refresh is running, try again" {
		t.Fatalf("other symbols: %+v", a)
	}
	// same symbols: wait for the first call, then reuse its rows
	waiting := make(chan struct{}, 4)
	r.l.d.After = func(time.Duration) <-chan time.Time {
		waiting <- struct{}{}
		return make(chan time.Time)
	}
	same := make(chan liveAnswer)
	go func() { same <- r.l.refresh(t.Context(), []string{"AAPL"}) }()
	<-waiting // the second call found the slot taken and is waiting for it
	close(r.brk.block)
	first, second := <-done, <-same
	if first.Total != 3 || !strings.HasPrefix(second.Notice, "reused, ") || r.brk.calls.Load() != 1 {
		t.Fatalf("first %+v second %+v calls %d", first, second, r.brk.calls.Load())
	}
}

func TestLiveRefreshDeadGatewayFallsBackWith200(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	r.l.d.Dial = func(context.Context) (broker.Broker, error) { return nil, errors.New("no socket") }
	h := siteHandler(nil, r.clk, liveDay, siteReaders{live: r.l}, echoHandler, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/options/refresh", strings.NewReader(`{"symbols":["AAPL"]}`)))
	var got liveAnswer
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.Notice != "pyside is not reachable; stored quotes shown" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	// a gateway that answers every item with an error: the rows are dropped, the stored ones stay
	r = newLiveRig(t, true, liveItems(1, 3), liveNoon)
	r.brk.err = errors.New("boom")
	a := r.l.refresh(t.Context(), []string{"AAPL"})
	rows, _ := r.l.OptionOpen(t.Context(), liveDay)
	if a.Fresh != 0 || a.Total != 3 || !strings.HasPrefix(a.Notice, "0 of 3 quotes fresh, 3 from ") || len(rows) != 0 {
		t.Fatalf("%+v rows %d", a, len(rows))
	}
}

func TestLiveRouteWithoutTheFlagIs404(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	h := siteHandler(a, candidatesClock, liveDay, siteReaders{}, echoHandler, http.NotFoundHandler())
	for _, m := range []string{http.MethodPost, http.MethodGet} {
		if rec := candidatesGet(t, h, m, "/options/refresh"); rec.Code != 404 {
			t.Errorf("%s: %d", m, rec.Code)
		}
	}
}

func TestLiveRouteOnlyPostAndBadBody(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	h := siteHandler(nil, r.clk, liveDay, siteReaders{live: r.l}, echoHandler, http.NotFoundHandler())
	if rec := candidatesGet(t, h, http.MethodGet, "/options/refresh"); rec.Code != 405 {
		t.Errorf("GET: %d", rec.Code)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/options/refresh", strings.NewReader(`nope`)))
	if rec.Code != 400 || r.brk.calls.Load() != 0 {
		t.Errorf("bad body: %d", rec.Code)
	}
}

func TestLiveStateEnabledFollowsTheSession(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, liveItems(1, 3), liveNoon)
	if s := r.l.state(t.Context(), []string{"AAPL"}); !s.Enabled || len(s.Symbols) != 1 {
		t.Errorf("open: %+v", s)
	}
	r.l.refresh(t.Context(), []string{"AAPL"})
	if s := r.l.state(t.Context(), []string{"AAPL"}); s.Notice != "3 of 3 quotes fresh" || s.Fresh != 3 || s.Total != 3 {
		t.Errorf("after a call: %+v", s)
	}
	closed := newLiveRig(t, false, nil, liveNoon)
	if s := closed.l.state(t.Context(), []string{"AAPL"}); s.Enabled || s.Reason != "market closed" {
		t.Errorf("closed: %+v", s)
	}
	var off *liveQuotes
	if s := off.state(t.Context(), []string{"AAPL"}); s.Enabled || s.Reason == "" {
		t.Errorf("flag off: %+v", s)
	}
}

func TestLiveMarketsJPLunchBreakIsClosed(t *testing.T) {
	t.Parallel()
	r := newLiveRig(t, true, nil, liveNoon)
	jp := r.l.markets[1]
	if jp.lunch[0] != "1130" || jp.lunch[1] != "1230" {
		t.Fatalf("lunch %v", jp.lunch)
	}
	r.l.d.Calendar = liveJPCal{}
	at := func(h, m int) time.Time { return time.Date(2026, 11, 11, h, m, 0, 0, jp.zone) }
	if !r.l.open(t.Context(), jp, at(10, 0)) || r.l.open(t.Context(), jp, at(12, 0)) || !r.l.open(t.Context(), jp, at(13, 0)) {
		t.Error("the lunch break is not closed")
	}
}

type liveJPCal struct{ liveCal }

func (liveJPCal) Session(_ context.Context, id string, day contracts.Day) (clock.Session, error) {
	tk := time.FixedZone("JST", 9*3600)
	return clock.Session{CalendarID: id, Day: day, IsSession: true,
		OpenAt: time.Date(2026, 11, 11, 9, 0, 0, 0, tk), CloseAt: time.Date(2026, 11, 11, 15, 0, 0, 0, tk)}, nil
}

func TestServeLiveQuotesRefusesNonLoopbackListen(t *testing.T) {
	t.Parallel()
	for _, addr := range []string{"192.0.2.1:8080", ":8080", "localhost:8080"} {
		a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
		a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
			t.Error("a store was opened")
			return nil, nil
		}
		err := runServe(a, []string{"--live-quotes", "--addr", addr})
		if err == nil || !strings.Contains(err.Error(), "--live-quotes") {
			t.Fatalf("%s: err %v", addr, err)
		}
	}
}
