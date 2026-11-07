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

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

type ctxKey struct{}

// fakeDB records every call; Exec keeps the first row of a key like ON CONFLICT DO NOTHING.
type fakeDB struct {
	events  []string
	execs   [][]any
	execSQL []string
	ctxs    []context.Context
	rows    [][]any
	selSQL  string
	selArgs []any
	execErr error
	selErr  error
	closed  bool
}

type fakeTx struct{ f *fakeDB }

func (t fakeTx) Exec(ctx context.Context, sql string, args ...any) error {
	t.f.ctxs = append(t.f.ctxs, ctx)
	t.f.execSQL = append(t.f.execSQL, sql)
	if t.f.execErr != nil {
		return t.f.execErr
	}
	t.f.execs = append(t.f.execs, args)
	return nil
}

func (f *fakeDB) InTx(ctx context.Context, fn func(execer) error) error {
	f.ctxs = append(f.ctxs, ctx)
	f.events = append(f.events, "begin")
	if err := fn(fakeTx{f}); err != nil {
		f.events = append(f.events, "rollback")
		return err
	}
	f.events = append(f.events, "commit")
	return nil
}

type rowScanner struct{ vals []any }

func (s rowScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(s.vals[i]))
	}
	return nil
}

func (f *fakeDB) Select(ctx context.Context, sql string, args []any, scan func(scanner) error) error {
	f.ctxs = append(f.ctxs, ctx)
	f.selSQL, f.selArgs = sql, args
	if f.selErr != nil {
		return f.selErr
	}
	for _, r := range f.rows {
		if err := scan(rowScanner{r}); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

func day(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatal(err)
	}
	return types.SerializableDate{Time: d}
}

func baseRow(t *testing.T) contracts.OptionEod {
	t.Helper()
	ts := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	code := "E1"
	terms := 7
	oi := day(t, "2026-11-05")
	return contracts.OptionEod{
		SessionDay: day(t, "2026-11-06"), CaptureID: "00000000-0000-4000-8000-000000000001", InstrumentID: 9,
		UnderlyingID: 8, Generation: 1, MarketDataType: 2, ReceivedAt: ts, EventTs: &ts, Status: 4,
		ErrorCode: &code, Currency: "USD", SnapshotKind: "eod", TermsVersionID: &terms,
		Bid: fp(1.25), Ask: fp(1.35), BidSize: fp(10), AskSize: fp(12), Last: fp(1.3), Volume: fp(250),
		OpenInterest: fp(1800), OiAsofDate: &oi, UnderlyingPrice: fp(450.5), UnderlyingCcy: "USD",
		UnderlyingEventTs: &ts, IvBidDecimal: fp(0.24), IvMidDecimal: fp(0.25), IvAskDecimal: fp(0.26),
		IvDecimal: fp(0.25), Delta: fp(0.45), Gamma: fp(0.03), ThetaPerCalendarDay: fp(-0.08), VegaPerVolPoint: fp(0.11),
	}
}

func newStore(f *fakeDB) *optionEodStore { return &optionEodStore{db: f} }

func TestInsertSQL(t *testing.T) {
	t.Parallel()
	if !strings.HasSuffix(insertOptionEodSQL, "ON CONFLICT DO NOTHING") {
		t.Fatalf("insert does not end ON CONFLICT DO NOTHING: %s", insertOptionEodSQL)
	}
	if strings.Contains(insertOptionEodSQL, "*") || strings.Contains(selectOptionEodSQL, "*") {
		t.Fatal("SQL contains *")
	}
	if !strings.Contains(selectOptionEodSQL, "FROM rd.option_eod_r1") ||
		!strings.Contains(selectOptionEodSQL, "ORDER BY session_day, instrument_id, capture_id") {
		t.Fatalf("select: %s", selectOptionEodSQL)
	}
	f := &fakeDB{}
	if err := newStore(f).InsertOptionEod(context.Background(), []contracts.OptionEod{baseRow(t)}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(insertOptionEodSQL, "$"); n != len(f.execs[0]) {
		t.Fatalf("%d placeholders, %d arguments", n, len(f.execs[0]))
	}
	if f.execSQL[0] != insertOptionEodSQL {
		t.Fatal("the statement is not insertOptionEodSQL")
	}
}

func TestInsertOneTransactionForSeveralRows(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	a, b := baseRow(t), baseRow(t)
	b.InstrumentID = 10
	if err := newStore(f).InsertOptionEod(context.Background(), []contracts.OptionEod{a, b}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.events, []string{"begin", "commit"}) || len(f.execs) != 2 {
		t.Fatalf("events %v, execs %d", f.events, len(f.execs))
	}
}

func TestInsertEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	if err := newStore(f).InsertOptionEod(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(f.events)+len(f.execs)+len(f.ctxs) != 0 {
		t.Fatalf("calls made: %v", f.events)
	}
}

func TestContextReachesEveryCall(t *testing.T) {
	t.Parallel()
	ctx := context.WithValue(context.Background(), ctxKey{}, "x")
	f := &fakeDB{}
	s := newStore(f)
	if err := s.InsertOptionEod(ctx, []contracts.OptionEod{baseRow(t), baseRow(t)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.OptionEod(ctx, 8, "2026-11-01", "2026-11-30"); err != nil {
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

func TestInsertScreensSentinels(t *testing.T) {
	t.Parallel()
	// argument index of each column in insertOptionEodSQL
	idx := map[string]int{"bid": 13, "volume": 18, "iv_decimal": 27, "delta": 28, "gamma": 29, "theta": 30}
	cases := []struct {
		name   string
		col    string
		mutate func(*contracts.OptionEod)
	}{
		{"bid -1", "bid", func(r *contracts.OptionEod) { r.Bid = fp(-1) }},
		{"iv_decimal -1", "iv_decimal", func(r *contracts.OptionEod) { r.IvDecimal = fp(-1) }},
		{"gamma -1", "gamma", func(r *contracts.OptionEod) { r.Gamma = fp(-1) }},
		{"delta -1", "delta", func(r *contracts.OptionEod) { r.Delta = fp(-1) }},
		{"theta -1", "theta", func(r *contracts.OptionEod) { r.ThetaPerCalendarDay = fp(-1) }},
		{"theta NaN", "theta", func(r *contracts.OptionEod) { r.ThetaPerCalendarDay = fp(math.NaN()) }},
		{"volume +Inf", "volume", func(r *contracts.OptionEod) { r.Volume = fp(math.Inf(1)) }},
	}
	base := baseRow(t)
	baseF := &fakeDB{}
	if err := newStore(baseF).InsertOptionEod(context.Background(), []contracts.OptionEod{base}); err != nil {
		t.Fatal(err)
	}
	want := baseF.execs[0]
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := baseRow(t)
			c.mutate(&r)
			f := &fakeDB{}
			if err := newStore(f).InsertOptionEod(context.Background(), []contracts.OptionEod{r}); err != nil {
				t.Fatal(err)
			}
			got := f.execs[0]
			for i := range got {
				if i == idx[c.col] {
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

func TestDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	f := &fakeDB{execErr: boom, selErr: boom}
	s := newStore(f)
	err := s.InsertOptionEod(context.Background(), []contracts.OptionEod{baseRow(t)})
	assertKind(t, err, contracts.Retryable, boom)
	_, err = s.OptionEod(context.Background(), 8, "2026-11-01", "2026-11-30")
	assertKind(t, err, contracts.Retryable, boom)
}

func assertKind(t *testing.T, err error, kind contracts.ErrorKind, cause error) {
	t.Helper()
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != kind {
		t.Fatalf("error %v, want kind %s", err, kind)
	}
	if cause != nil && !errors.Is(err, cause) {
		t.Fatalf("error %v does not wrap %v", err, cause)
	}
}

func TestMalformedDayIsPermanent(t *testing.T) {
	t.Parallel()
	for _, c := range [][2]contracts.Day{{"2026-13-01", "2026-11-30"}, {"2026-11-01", "tomorrow"}} {
		f := &fakeDB{}
		_, err := newStore(f).OptionEod(context.Background(), 8, c[0], c[1])
		assertKind(t, err, contracts.Permanent, nil)
		if f.selSQL != "" {
			t.Fatal("the database was called")
		}
	}
}

func TestReaderRangeAndOrder(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 11, 6, 22, 0, 0, 0, time.FixedZone("x", 3600))
	d := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	var nilF *float64
	var nilS *string
	var nilT *time.Time
	var nilI *int
	var nilD *time.Time
	row := func(capture string, inst int) []any {
		return []any{d, capture, inst, 8, 1, 2, ts, &ts, 1, nilS, "USD", "eod", nilI,
			fp(1), nilF, nilF, nilF, nilF, nilF, nilF, nilD, nilF, "USD", nilT,
			nilF, nilF, nilF, nilF, nilF, nilF, nilF, nilF}
	}
	f := &fakeDB{rows: [][]any{row("c1", 1), row("c2", 1), row("c1", 2)}}
	got, err := newStore(f).OptionEod(context.Background(), 8, "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatal(err)
	}
	if f.selArgs[0] != int64(8) || !f.selArgs[1].(time.Time).Equal(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)) ||
		!f.selArgs[2].(time.Time).Equal(time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("args %v", f.selArgs)
	}
	var order []string
	for _, r := range got {
		order = append(order, fmt.Sprintf("%s %d", r.CaptureID, r.InstrumentID))
	}
	if !reflect.DeepEqual(order, []string{"c1 1", "c2 1", "c1 2"}) {
		t.Fatalf("order %v", order)
	}
	if got[0].ReceivedAt.Location() != time.UTC || got[0].SessionDay.Time != d || *got[0].Bid != 1 {
		t.Fatalf("row %+v", got[0])
	}
	empty, err := newStore(&fakeDB{}).OptionEod(context.Background(), 8, "2026-11-01", "2026-11-30")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty: %v %v", empty, err)
	}
}

func TestClose(t *testing.T) {
	t.Parallel()
	f := &fakeDB{}
	if err := newStore(f).Close(); err != nil || !f.closed {
		t.Fatal("not closed")
	}
}
