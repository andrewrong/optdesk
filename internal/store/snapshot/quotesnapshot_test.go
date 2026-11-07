package snapshot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func quoteRow(t *testing.T) contracts.QuoteSnapshot {
	t.Helper()
	ts := time.Date(2026, 11, 6, 15, 0, 0, 0, time.UTC)
	code := "E1"
	return contracts.QuoteSnapshot{
		SessionDay: day(t, "2026-11-06"), CaptureID: "00000000-0000-4000-8000-000000000011", Slot: "1000",
		InstrumentID: 9, ReceivedAt: ts, Generation: 1, MarketDataType: 2, EventTs: &ts, Status: 4,
		ErrorCode: &code, Currency: "USD", Bid: fp(1.25), Ask: fp(1.35), BidSize: fp(10), AskSize: fp(12),
		Last: fp(1.3), LastSize: fp(2), Volume: fp(40),
	}
}

func newQuoteStore(f *fakeDB) *quoteSnapshotStore { return &quoteSnapshotStore{db: f} }

func TestQuoteInsertSQL(t *testing.T) {
	t.Parallel()
	if !strings.HasSuffix(insertQuoteSnapshotSQL, "ON CONFLICT DO NOTHING") {
		t.Fatalf("insert does not end ON CONFLICT DO NOTHING: %s", insertQuoteSnapshotSQL)
	}
	if strings.Contains(insertQuoteSnapshotSQL, "*") || strings.Contains(selectQuoteSnapshotSQL, "*") {
		t.Fatal("SQL contains *")
	}
	if !strings.Contains(selectQuoteSnapshotSQL, "FROM rd.quote_snapshot_r1") ||
		!strings.Contains(selectQuoteSnapshotSQL, "ORDER BY session_day, slot, capture_id") {
		t.Fatalf("select: %s", selectQuoteSnapshotSQL)
	}
	f := &fakeDB{}
	if err := newQuoteStore(f).InsertQuoteSnapshots(context.Background(), []contracts.QuoteSnapshot{quoteRow(t)}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(insertQuoteSnapshotSQL, "$"); n != len(f.execs[0]) {
		t.Fatalf("%d placeholders, %d arguments", n, len(f.execs[0]))
	}
	if f.execSQL[0] != insertQuoteSnapshotSQL {
		t.Fatal("the statement is not insertQuoteSnapshotSQL")
	}
}

func TestQuoteInsertOneTransactionForSeveralRows(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	a, b := quoteRow(t), quoteRow(t)
	b.Slot = "1500" // same instrument and capture, another slot: a different key
	if err := newQuoteStore(f).InsertQuoteSnapshots(context.Background(), []contracts.QuoteSnapshot{a, b}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || len(f.execs) != 2 {
		t.Fatalf("events %v, execs %d", f.events, len(f.execs))
	}
	if f.execs[0][2] != "1000" || f.execs[1][2] != "1500" || f.execs[0][3] != f.execs[1][3] || f.execs[0][1] != f.execs[1][1] {
		t.Fatalf("both slots must be sent with the same capture and instrument: %v %v", f.execs[0], f.execs[1])
	}
}

func TestQuoteInsertEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	if err := newQuoteStore(f).InsertQuoteSnapshots(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(f.events)+len(f.execs)+len(f.ctxs) != 0 {
		t.Fatalf("calls made: %v", f.events)
	}
}

func TestQuoteContextReachesEveryCall(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxKey{}, "x")
	f := &fakeDB{}
	s := newQuoteStore(f)
	if err := s.InsertQuoteSnapshots(ctx, []contracts.QuoteSnapshot{quoteRow(t), quoteRow(t)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QuoteSnapshots(ctx, 9, "2026-11-01", "2026-11-30"); err != nil {
		t.Fatal(err)
	}
	if len(f.ctxs) != 4 { // InTx, two Exec, Select
		t.Fatalf("calls: %d", len(f.ctxs))
	}
	for i, c := range f.ctxs {
		if c.Value(ctxKey{}) != "x" {
			t.Fatalf("call %d lost the context", i)
		}
	}
}

func TestQuoteInsertScreensSentinels(t *testing.T) {
	t.Parallel()
	// argument index of each numeric column in insertQuoteSnapshotSQL
	cases := []struct {
		col    string
		idx    int
		mutate func(*contracts.QuoteSnapshot, float64)
	}{
		{"bid", 11, func(r *contracts.QuoteSnapshot, v float64) { r.Bid = fp(v) }},
		{"ask", 12, func(r *contracts.QuoteSnapshot, v float64) { r.Ask = fp(v) }},
		{"bid_size", 13, func(r *contracts.QuoteSnapshot, v float64) { r.BidSize = fp(v) }},
		{"ask_size", 14, func(r *contracts.QuoteSnapshot, v float64) { r.AskSize = fp(v) }},
		{"last", 15, func(r *contracts.QuoteSnapshot, v float64) { r.Last = fp(v) }},
		{"last_size", 16, func(r *contracts.QuoteSnapshot, v float64) { r.LastSize = fp(v) }},
		{"volume", 17, func(r *contracts.QuoteSnapshot, v float64) { r.Volume = fp(v) }},
	}
	baseF := &fakeDB{}
	if err := newQuoteStore(baseF).InsertQuoteSnapshots(context.Background(), []contracts.QuoteSnapshot{quoteRow(t)}); err != nil {
		t.Fatal(err)
	}
	want := baseF.execs[0]
	for _, c := range cases {
		for name, v := range map[string]float64{"-1": -1, "NaN": math.NaN(), "+Inf": math.Inf(1)} {
			t.Run(c.col+" "+name, func(t *testing.T) {
				t.Parallel()
				r := quoteRow(t)
				c.mutate(&r, v)
				f := &fakeDB{}
				if err := newQuoteStore(f).InsertQuoteSnapshots(context.Background(), []contracts.QuoteSnapshot{r}); err != nil {
					t.Fatal(err)
				}
				got := f.execs[0]
				for i := range got {
					if i == c.idx {
						if p, ok := got[i].(*float64); !ok || p != nil {
							t.Fatalf("%s: arg %d = %v, want NULL", c.col, i, got[i])
						}
						continue
					}
					if !reflect.DeepEqual(got[i], want[i]) {
						t.Fatalf("arg %d changed: %v, want %v", i, got[i], want[i])
					}
				}
				if got[8] != 4 {
					t.Fatalf("status changed to %v", got[8])
				}
			})
		}
	}
}

func TestQuoteDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	f := &fakeDB{execErr: boom, selErr: boom}
	s := newQuoteStore(f)
	err := s.InsertQuoteSnapshots(context.Background(), []contracts.QuoteSnapshot{quoteRow(t)})
	assertKind(t, err, contracts.Retryable, boom)
	_, err = s.QuoteSnapshots(context.Background(), 9, "2026-11-01", "2026-11-30")
	assertKind(t, err, contracts.Retryable, boom)
}

func TestQuoteMalformedDayIsPermanent(t *testing.T) {
	t.Parallel()
	for _, c := range [][2]contracts.Day{{"2026-13-01", "2026-11-30"}, {"2026-11-01", "tomorrow"}} {
		f := &fakeDB{}
		_, err := newQuoteStore(f).QuoteSnapshots(context.Background(), 9, c[0], c[1])
		assertKind(t, err, contracts.Permanent, nil)
		if f.selSQL != "" {
			t.Fatal("the database was called")
		}
	}
}

func TestQuoteReaderRangeAndOrder(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 11, 6, 16, 0, 0, 0, time.FixedZone("x", 3600))
	d := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	var nilF *float64
	var nilS *string
	var nilT *time.Time
	row := func(capture, slot string) []any {
		return []any{d, capture, slot, 9, ts, 1, 2, &ts, 1, nilS, "USD", fp(1), nilF, nilF, nilF, nilF, nilF, nilF}
	}
	_ = nilT
	f := &fakeDB{rows: [][]any{row("c1", "1000"), row("c2", "1000"), row("c1", "1500")}}
	got, err := newQuoteStore(f).QuoteSnapshots(context.Background(), 9, "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if f.selArgs[0] != int64(9) || !f.selArgs[1].(time.Time).Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) ||
		!f.selArgs[2].(time.Time).Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("args %v", f.selArgs)
	}
	var order []string
	for _, r := range got {
		order = append(order, fmt.Sprintf("%s %s", r.CaptureID, r.Slot))
	}
	if !reflect.DeepEqual(order, []string{"c1 1000", "c2 1000", "c1 1500"}) {
		t.Fatalf("order %v", order)
	}
	if got[0].ReceivedAt.Location() != time.UTC || got[0].EventTs.Location() != time.UTC ||
		got[0].SessionDay.Time != d || *got[0].Bid != 1 {
		t.Fatalf("row %+v", got[0])
	}
	empty, err := newQuoteStore(&fakeDB{}).QuoteSnapshots(context.Background(), 9, "2026-11-01", "2026-11-30")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty: %v %v", empty, err)
	}
}

func TestQuoteClose(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	if err := newQuoteStore(f).Close(); err != nil || !f.closed {
		t.Fatal("not closed")
	}
}
