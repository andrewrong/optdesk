package optionopenpg

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// fakeDB records the statements of the store and fails the Exec numbered failAt.
type fakeDB struct {
	sqls   []string
	args   [][]any
	affect int64
	failAt int // 1-based; 0 never
	inTx   int
	closed bool
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.inTx++
	return fn(f)
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	f.sqls = append(f.sqls, sql)
	f.args = append(f.args, args)
	if f.failAt == len(f.sqls) {
		return 0, errors.New("boom")
	}
	return f.affect, nil
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, _ func(scanner) error) error {
	f.sqls = append(f.sqls, sql)
	f.args = append(f.args, args)
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

func kind(err error) contracts.ErrorKind {
	const none = contracts.ErrorKind("")
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te.Kind()
	}
	return none
}

func newStore() (*store, *fakeDB) {
	f := &fakeDB{}
	return &store{db: f}, f
}

func TestEmptyInputsMakeNoDatabaseCall(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, f := newStore()
	for _, err := range []error{s.InsertOptionOpen(ctx, nil), s.PutPlan(ctx, nil), s.AddNotListed(ctx, nil)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	if f.inTx != 0 || len(f.sqls) != 0 {
		t.Fatalf("calls: %d transactions, %v", f.inTx, f.sqls)
	}
}

func TestInsertOptionOpenRefusesRefreshAndScreensSentinels(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, f := newStore()
	if err := s.InsertOptionOpen(ctx, []contracts.OptionOpen{{Slot: openquote.SlotRefresh}}); err == nil || kind(err) != contracts.Permanent {
		t.Fatalf("a refresh row: %v", err)
	}
	if len(f.sqls) != 0 {
		t.Fatal("a refused row reached the database")
	}
	m1, nan, ok := -1.0, math.NaN(), 1.25
	row := contracts.OptionOpen{Slot: "0945", Delta: &m1, Gamma: &nan, Bid: &ok, IvDecimal: &m1}
	if err := s.InsertOptionOpen(ctx, []contracts.OptionOpen{row, row}); err != nil {
		t.Fatal(err)
	}
	if f.inTx != 1 || len(f.sqls) != 2 || !strings.Contains(f.sqls[0], "ON CONFLICT DO NOTHING") {
		t.Fatalf("one transaction with two inserts expected: %d %d", f.inTx, len(f.sqls))
	}
	a := f.args[0]
	if len(a) != 35 {
		t.Fatalf("%d arguments, the statement takes 35", len(a))
	}
	// delta, gamma, bid, iv_decimal sit at 32, 33, 19, 31 (1-based).
	if a[31].(*float64) != nil || a[32].(*float64) != nil || a[30].(*float64) != nil {
		t.Errorf("delta, gamma and iv must be null: %v %v %v", a[31], a[32], a[30])
	}
	if got := a[18].(*float64); got == nil || *got != 1.25 {
		t.Errorf("bid = %v", got)
	}
}

func TestWriteErrorIsRetryableAndConstraintIsPermanent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, f := newStore()
	f.failAt = 1
	err := s.PutPlan(ctx, []contracts.OptionOpenItem{{}})
	if err == nil || kind(err) != contracts.Retryable {
		t.Fatalf("err = %v, kind %v", err, kind(err))
	}
}

func TestBadDayIsPermanent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, f := newStore()
	if _, err := s.OptionOpen(ctx, "nope"); kind(err) != contracts.Permanent {
		t.Errorf("OptionOpen: %v", err)
	}
	if _, err := s.Plan(ctx, "2026-13-01", "0945"); kind(err) != contracts.Permanent {
		t.Errorf("Plan: %v", err)
	}
	if err := s.PutOption(ctx, openquote.OptionRef{Expiry: "x"}); kind(err) != contracts.Permanent {
		t.Errorf("PutOption: %v", err)
	}
	if _, err := s.RemoveNotListedExpired(ctx, "x"); kind(err) != contracts.Permanent {
		t.Errorf("RemoveNotListedExpired: %v", err)
	}
	if len(f.sqls) != 0 {
		t.Fatal("a bad day reached the database")
	}
}

func TestReadsGoThroughViewsAndEmptyIsNotNil(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, f := newStore()
	if got, err := s.OptionOpen(ctx, "2026-11-11"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("%#v %v", got, err)
	}
	if got, err := s.Plan(ctx, "2026-11-11", "0945"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("%#v %v", got, err)
	}
	if !strings.Contains(f.sqls[0], "rd.option_open_r1") || strings.Contains(f.sqls[0], "obs.option_open") {
		t.Errorf("OptionOpen sql: %s", f.sqls[0])
	}
	if !strings.Contains(f.sqls[1], "rd.option_open_item_r1") || strings.Contains(f.sqls[1], "plan.option_open_item") {
		t.Errorf("Plan sql: %s", f.sqls[1])
	}
	ids := f.args[1][2].([]int64)
	if ids == nil || len(ids) != 0 {
		t.Errorf("no ids must pass an empty array, got %#v", f.args[1][2])
	}
}

func TestMarksAndContracts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, f := newStore()
	f.affect = 3
	n, err := s.RemoveNotListedByUnderlying(ctx, 9)
	if err != nil || n != 3 {
		t.Fatalf("n = %d, %v", n, err)
	}
	ok, err := s.RemoveNotListed(ctx, openquote.NotListedKey{UnderlyingID: 9, Expiry: "2026-12-18", Strike: "100", Right: "P", TradingClass: "AAPL"})
	if err != nil || !ok {
		t.Fatalf("removed = %v, %v", ok, err)
	}
	f.affect = 0
	if ok, _ := s.RemoveNotListed(ctx, openquote.NotListedKey{Expiry: "2026-12-18"}); ok {
		t.Error("nothing removed must report false")
	}
	exp := types.SerializableDate{Time: time.Date(2026, 12, 18, 0, 0, 0, 0, time.UTC)}
	if err := s.AddNotListed(ctx, []contracts.OptionNotListed{{
		UnderlyingID: 9, Expiry: exp, Strike: "100", Right: "P", TradingClass: "AAPL",
	}}); err != nil {
		t.Fatal(err)
	}
	last := f.sqls[len(f.sqls)-1]
	if !strings.Contains(last, "DO UPDATE SET checked_at") {
		t.Errorf("a mark must be replaced: %s", last)
	}
	if err := s.PutOption(ctx, openquote.OptionRef{Expiry: "2026-12-18", Strike: "180", Right: "P"}); err != nil {
		t.Fatal(err)
	}
	if last := f.sqls[len(f.sqls)-1]; !strings.Contains(last, "DO UPDATE SET instrument_id") {
		t.Errorf("a contract must be updated: %s", last)
	}
	if err := s.Close(); err != nil || !f.closed {
		t.Fatalf("close: %v %v", err, f.closed)
	}
}
