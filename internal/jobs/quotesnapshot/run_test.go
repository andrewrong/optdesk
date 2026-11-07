package quotesnapshot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const testDay = contracts.Day("2026-11-05")

func fakeClockAt(t *testing.T, local string) *clockwork.FakeClock {
	t.Helper()
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	at, err := time.ParseInLocation("2006-01-02 15:04", local, loc)
	if err != nil {
		t.Fatal(err)
	}
	return clockwork.NewFakeClockAt(at)
}

type fakeBroker struct {
	log     *[]string
	calls   [][]broker.Contract
	status  func(c broker.Contract) broker.Status
	onCall  func(n int)
	snapErr error
}

func (b *fakeBroker) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, nil
}
func (b *fakeBroker) ResolveContract(context.Context, broker.Contract) (broker.Resolved, error) {
	return broker.Resolved{}, nil
}
func (b *fakeBroker) Close() error { return nil }

func (b *fakeBroker) Snapshot(_ context.Context, kind broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	if kind != broker.KindSnapshot {
		return nil, fmt.Errorf("kind %d", kind)
	}
	b.calls = append(b.calls, cs)
	if b.log != nil {
		*b.log = append(*b.log, "snapshot")
	}
	if b.onCall != nil {
		b.onCall(len(b.calls))
	}
	if b.snapErr != nil {
		return nil, b.snapErr
	}
	out := make([]broker.Quote, len(cs))
	for i, c := range cs {
		st := broker.StatusOK
		if b.status != nil {
			st = b.status(c)
		}
		px, sz := float64(c.ConID), float64(c.ConID%7)
		ev := time.Date(2026, 11, 5, 14, 45, 0, 0, time.UTC)
		out[i] = broker.Quote{Contract: c, Status: st, MarketDataType: 1, Generation: c.ConID % 3,
			ReceivedAt: ev.Add(time.Second), EventTs: &ev, Bid: &px, Ask: &px, Last: &px, LastSize: &sz, BidSize: &sz, AskSize: &sz}
	}
	return out, nil
}

type fakeResolver struct {
	err  error
	keys []store.Identifier
}

func (r *fakeResolver) ResolveInstrument(_ context.Context, key store.Identifier, _ store.InstrumentSpec, _ []store.Identifier) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	r.keys = append(r.keys, key)
	n, _ := strconv.ParseInt(key.Value, 10, 64)
	return n + 1000, nil
}

type fakeWriter struct {
	log   *[]string
	calls [][]contracts.QuoteSnapshot
	err   error
}

func (w *fakeWriter) InsertQuoteSnapshots(_ context.Context, rows []contracts.QuoteSnapshot) error {
	if w.err != nil {
		return w.err
	}
	w.calls = append(w.calls, rows)
	if w.log != nil {
		*w.log = append(*w.log, "write")
	}
	return nil
}

func slotKey(s string) contracts.ExpectedKey {
	return contracts.ExpectedKey{DatasetID: "obs.quote_snapshot", SubjectKey: s}
}

// manifestWith builds a manifest of n instruments with con ids 1..n and extra top-level lines.
func manifestWith(n int, extra string) string {
	var sb strings.Builder
	sb.WriteString("id = \"quote_snapshot\"\n" + extra + "\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "[[instrument]]\nsymbol = \"S%d\"\nsec_type = \"STK\"\ncon_id = %d\ncurrency = \"USD\"\n", i, i)
	}
	return sb.String()
}

func newJob(t *testing.T, extra string, b broker.Broker, r store.InstrumentResolver, w store.QuoteSnapshotWriter, now string) *job {
	t.Helper()
	return newJobN(t, 3, extra, b, r, w, fakeClockAt(t, now))
}

func newJobN(t *testing.T, n int, extra string, b broker.Broker, r store.InstrumentResolver, w store.QuoteSnapshotWriter, clk clock.Clock) *job {
	t.Helper()
	j, err := build([]byte(manifestWith(n, extra)), b, r, w, clk)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestRunWritesRows(t *testing.T) {
	t.Parallel()
	var log []string
	b := &fakeBroker{log: &log}
	w := &fakeWriter{log: &log}
	j := newJobN(t, 120, "chunk = 50", b, &fakeResolver{}, w, fakeClockAt(t, "2026-11-05 09:50"))
	res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{slotKey("0945")})
	if res.Err != nil || res.Returned != 120 || res.Usable != 120 {
		t.Fatalf("res = %+v", res)
	}
	var sizes []int
	for _, c := range b.calls {
		sizes = append(sizes, len(c))
	}
	if fmt.Sprint(sizes) != "[50 50 20]" || b.calls[0][0].ConID != 1 || b.calls[1][0].ConID != 51 || b.calls[2][0].ConID != 101 {
		t.Fatalf("calls = %v", sizes)
	}
	if got := strings.Join(log, ","); got != "snapshot,write,snapshot,write,snapshot,write" {
		t.Fatalf("order = %s", got)
	}
	capture := w.calls[0][0].CaptureID
	if capture == "" {
		t.Fatal("no capture id")
	}
	for _, rows := range w.calls {
		for _, r := range rows {
			checkRow(t, r, capture)
		}
	}
}

func checkRow(t *testing.T, r contracts.QuoteSnapshot, capture string) {
	t.Helper()
	con := int64(r.InstrumentID - 1000)
	if r.Slot != "0945" || r.CaptureID != capture || r.Status != 1 || r.Currency != "USD" ||
		r.SessionDay.Format("2006-01-02") != string(testDay) {
		t.Fatalf("row = %+v", r)
	}
	if r.LastSize == nil || *r.LastSize != float64(con%7) || *r.BidSize != float64(con%7) || *r.AskSize != float64(con%7) {
		t.Fatalf("sizes of %d: %+v", con, r)
	}
	if r.EventTs == nil || !r.EventTs.Equal(time.Date(2026, 11, 5, 14, 45, 0, 0, time.UTC)) {
		t.Fatalf("event_ts = %v", r.EventTs)
	}
	if r.Generation != int(con%3) {
		t.Fatalf("generation of %d = %d", con, r.Generation)
	}
}

func TestCaptureIDStable(t *testing.T) {
	t.Parallel()
	run := func(day contracts.Day, slot, now string) string {
		w := &fakeWriter{}
		j := newJob(t, "", &fakeBroker{}, &fakeResolver{}, w, now)
		if res := j.Run(context.Background(), day, []contracts.ExpectedKey{slotKey(slot)}); res.Err != nil {
			t.Fatal(res.Err)
		}
		return w.calls[0][0].CaptureID
	}
	a := run(testDay, "0945", "2026-11-05 09:50")
	if a != run(testDay, "0945", "2026-11-05 09:51") {
		t.Fatal("two runs of one key gave two capture ids")
	}
	if a == run("2026-11-06", "0945", "2026-11-06 09:50") {
		t.Fatal("another day gave the same capture id")
	}
	if a == run(testDay, "1230", "2026-11-05 12:35") {
		t.Fatal("another slot gave the same capture id")
	}
}

func TestBudget(t *testing.T) {
	t.Parallel()
	clk := fakeClockAt(t, "2026-11-05 09:46")
	b := &fakeBroker{onCall: func(n int) {
		if n == 1 {
			clk.Advance(9 * time.Minute)
		}
	}}
	w := &fakeWriter{}
	j := newJobN(t, 120, "chunk = 50\nbudget_minutes = 8\ngrace_minutes = 30", b, &fakeResolver{}, w, clk)
	res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{slotKey("0945")})
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("err = %v", res.Err)
	}
	if len(b.calls) != 1 || len(w.calls) != 1 || len(w.calls[0]) != 50 || res.Returned != 50 {
		t.Fatalf("calls %d, writes %d, res %+v", len(b.calls), len(w.calls), res)
	}
}

func TestNoQuote(t *testing.T) {
	t.Parallel()
	w := &fakeWriter{}
	b := &fakeBroker{status: func(broker.Contract) broker.Status { return broker.StatusNoData }}
	j := newJob(t, "", b, &fakeResolver{}, w, "2026-11-05 09:50")
	res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{slotKey("0945")})
	if res.Err == nil || res.Err.Kind() != contracts.NoValue || !strings.Contains(res.Err.Error(), "no quote") {
		t.Fatalf("err = %v", res.Err)
	}
	if len(w.calls) != 1 || len(w.calls[0]) != 3 || w.calls[0][0].Status != 2 {
		t.Fatalf("writes = %v", w.calls)
	}

	b2 := &fakeBroker{status: func(c broker.Contract) broker.Status {
		if c.ConID == 2 {
			return broker.StatusOK
		}
		return broker.StatusNoData
	}}
	j = newJob(t, "", b2, &fakeResolver{}, &fakeWriter{}, "2026-11-05 09:50")
	if res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{slotKey("0945")}); res.Err != nil || res.Usable != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestGuards(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, now string
		day       contracts.Day
		kind      contracts.ErrorKind
		text      string
	}{
		{"wrong day", "2026-11-06 09:50", testDay, contracts.NoValue, "wrong day"},
		{"before slot", "2026-11-05 09:44", testDay, "", ""},
		{"missed slot", "2026-11-05 09:56", testDay, contracts.NoValue, "missed slot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, w, r := &fakeBroker{}, &fakeWriter{}, &fakeResolver{}
			j := newJob(t, "", b, r, w, tc.now)
			res := j.Run(context.Background(), tc.day, []contracts.ExpectedKey{slotKey("0945")})
			if len(b.calls) != 0 || len(w.calls) != 0 || len(r.keys) != 0 {
				t.Fatal("a call was made")
			}
			if tc.kind == "" {
				if res.Err != nil || res.Returned != 0 || res.Usable != 0 {
					t.Fatalf("res = %+v", res)
				}
				return
			}
			if res.Err == nil || res.Err.Kind() != tc.kind || !strings.Contains(res.Err.Error(), tc.text) {
				t.Fatalf("err = %v", res.Err)
			}
		})
	}
}
