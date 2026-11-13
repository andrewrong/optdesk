package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

func builtOptionEod(t *testing.T) contracts.Job {
	t.Helper()
	clk := fakeClock{time.Date(2026, 11, 9, 7, 0, 0, 0, time.UTC)}
	watch, err := parseWatch([]byte(jpStockWatch))
	if err != nil {
		t.Fatal(err)
	}
	j, err := buildOptionEodJP(newDeps(), unreachableBroker{}, &memResolver{}, &memSink{}, watch, clk)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

// The writer is handed to the job the constructor built, and the same job comes back. (The call count of a run is
// asserted by TestOptionEodRunCallsWriterOnce.)
func TestWrapOptionEodWithWriter(t *testing.T) {
	t.Parallel()
	built := builtOptionEod(t)
	got, err := withOptionEodReasons(built, nil, &fakeRunStore{})
	if err != nil || got != built {
		t.Fatalf("got %v, %v; want the built job", got, err)
	}
	// A job another package built cannot take the writer: the wiring fails, it does not drop the writer.
	if _, err := withOptionEodReasons(reasonForeignJob{}, nil, &fakeRunStore{}); err == nil {
		t.Fatal("a foreign job took the writer")
	}
	// A failed build keeps its error and is not wrapped.
	boom := errors.New("boom")
	if _, err := withOptionEodReasons(nil, boom, &fakeRunStore{}); !errors.Is(err, boom) {
		t.Fatalf("err %v, want the build error", err)
	}
}

func TestWrapOptionEodNilWriterUnchanged(t *testing.T) {
	t.Parallel()
	built := builtOptionEod(t)
	got, err := withOptionEodReasons(built, nil, nil)
	if err != nil || got != built {
		t.Fatalf("got %v, %v; want the job as built", got, err)
	}
	// Even a foreign job passes with no writer.
	if got, err := withOptionEodReasons(reasonForeignJob{}, nil, nil); err != nil || got != (reasonForeignJob{}) {
		t.Fatalf("foreign job: %v, %v", got, err)
	}
}

// The reader the site opened with the pages line reaches the page input; the demo profile has none.
func TestOptionDataPageCarriesReasons(t *testing.T) {
	t.Parallel()
	st := opsTodayFixture()
	a := opsTodayApp(t, st)
	var closers []io.Closer
	track := func(c io.Closer, err error) bool {
		if err == nil {
			closers = append(closers, c)
		}
		return err == nil
	}
	v, ok := openOpsTodayRuns(t.Context(), a, track)
	if !ok {
		t.Fatal("open failed")
	}
	rd := siteReaders{ext: map[string]any{siteOpsTodayKey: v}}
	r := optionReasonReader(rd)
	if r == nil {
		t.Fatal("no reason reader for the option data page")
	}
	rows, err := r.RunsOn(context.Background(), "2026-11-13")
	if err != nil || len(rows) != 1 {
		t.Fatalf("reader answered %v, %v", rows, err)
	}
	if _, ok := r.(runview.ReasonWriter); ok {
		t.Fatal("the page reader can write")
	}
	if st.openedDSN != "postgres://pages" {
		t.Fatalf("reader opened with %q, want the pages line", st.openedDSN)
	}
	// The reader's note is in the page newOptionDataPage builds from the same input.
	d := demoOption{}
	st.reasons = []runview.Reason{{Code: "field_missing", Subject: "AAPL",
		Detail: "open_interest missing 200/200 usable quotes: not_received_grace 200"}}
	page := newOptionDataPage(time.UTC, "b", sitePageInput{UnderlyingID: demoOptionIDs[0], Symbol: "AAPL", Eod: d, Quotes: d, Terms: d, Reasons: r})
	out, err := page.Build(t.Context(), nil, "2026-11-06", time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC))
	if err != nil || !strings.Contains(string(out), "open_interest missing 200/200") {
		t.Fatalf("note absent: %v", err)
	}
	if optionReasonReader(siteReaders{}) != nil {
		t.Fatal("a site with no run store has a reader")
	}
}

// reasonForeignJob is a job this package of the test built, not one of optioneod.
type reasonForeignJob struct{ contracts.Job }

// usWatch is one synthetic US stock.
const usWatch = `
[[underlying]]
issuer = "cik:0000000001"
market = "US"
symbol = "TEST"
sec_type = "STK"
exchange = "SMART"
currency = "USD"
con_id = 1000001
`

// noOIBroker answers one option chain whose quotes carry no open_interest, with the reason the broker reports.
type noOIBroker struct{ unreachableBroker }

func (noOIBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return []broker.Chain{{Exchange: "SMART", TradingClass: "TEST", Multiplier: "100",
		Expirations: []string{"20261204", "20261218"}, Strikes: []string{"95", "100", "105"}}}, nil
}

func (noOIBroker) ResolveContract(_ context.Context, c broker.Contract) (broker.Resolved, error) {
	c.ConID = int64(len(c.Strike)) + 100
	return broker.Resolved{Listed: true, Contract: c}, nil
}

func (noOIBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	px := 100.0
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		one := 1.0
		out[i] = broker.Quote{Contract: c, Last: &px, Bid: &one, Ask: &one, UndPrice: &px, Status: broker.StatusOK,
			ReceivedAt: time.Date(2026, 11, 12, 22, 0, 0, 0, time.UTC),
			Missing:    []broker.MissingField{{Field: "open_interest", Reason: broker.MissingNotReceivedGrace}}}
	}
	return out, nil
}

// replaceCounter is a reason writer that counts its calls.
type replaceCounter struct {
	mu    sync.Mutex
	calls int
}

func (c *replaceCounter) ReplaceReasons(context.Context, contracts.Day, string, string, []runview.Reason) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return nil
}

func TestOptionEodRunCallsWriterOnce(t *testing.T) {
	t.Parallel()
	watch, err := parseWatch([]byte(usWatch))
	if err != nil {
		t.Fatal(err)
	}
	clk := fakeClock{time.Date(2026, 11, 12, 22, 0, 0, 0, time.UTC)}
	j, err := buildOptionEod(newDeps(), noOIBroker{}, &memResolver{}, &memSink{}, watch, clk)
	if err != nil {
		t.Fatal(err)
	}
	w := &replaceCounter{}
	if j, err = withOptionEodReasons(j, nil, w); err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(t.Context(), "2026-11-12")
	if err != nil || len(keys) != 1 {
		t.Fatalf("plan %v, %v", keys, err)
	}
	if res := j.Run(t.Context(), "2026-11-12", keys); res.Err != nil || res.Usable == 0 {
		t.Fatalf("run %+v", res)
	}
	if w.calls != 1 {
		t.Fatalf("writer called %d times, want 1", w.calls)
	}
}
