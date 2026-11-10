package markpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

// fakeDB records the statements and answers Select with canned rows.
type fakeDB struct {
	execs   []string
	args    [][]any
	selects []string
	rows    [][]any
	failAt  int // 1-based Exec that fails; 0 never
	inTx    int
	closed  bool
}

type fakeScan []any

func (f fakeScan) Scan(dest ...any) error {
	for i, d := range dest {
		switch p := d.(type) {
		case *string:
			*p = f[i].(string)
		case *int64:
			*p = f[i].(int64)
		case *int:
			*p = f[i].(int)
		case *time.Time:
			*p = f[i].(time.Time)
		}
	}
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.inTx++
	return fn(f)
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) (int64, error) {
	f.execs = append(f.execs, sql)
	f.args = append(f.args, args)
	if f.failAt == len(f.execs) {
		return 0, errors.New("boom")
	}
	return 1, nil
}

func (f *fakeDB) Select(_ context.Context, sql string, _ []any, scan func(scanner) error) error {
	f.selects = append(f.selects, sql)
	for _, r := range f.rows {
		if err := scan(fakeScan(r)); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Close() error { f.closed = true; return nil }

func sample(con int64, attempt int, basis, price string) mark.PositionMark {
	return mark.PositionMark{CaptureID: "main:2026-11-09:1", Alias: "main", MarkedAt: time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC),
		SessionDay: "2026-11-09", ConID: con, Attempt: attempt, Currency: "USD", Basis: basis, Price: price, Status: "ok"}
}

func TestWriteMarksOneTransactionDoNothingOnConflict(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	var s mark.Store = &store{db: db}
	if err := s.WriteMarks(context.Background(), []mark.PositionMark{sample(1, 1, mark.BasisMid, "1.25"), sample(2, 1, mark.BasisNone, "")}); err != nil {
		t.Fatal(err)
	}
	if db.inTx != 1 || len(db.execs) != 2 {
		t.Fatalf("want 1 transaction of 2 inserts, got %d and %d", db.inTx, len(db.execs))
	}
	for _, q := range db.execs {
		if !strings.Contains(q, "ON CONFLICT DO NOTHING") || strings.Contains(strings.ToUpper(q), "UPDATE") {
			t.Fatalf("insert must be ON CONFLICT DO NOTHING: %s", q)
		}
	}
	if db.args[0][7] != "1.25" || db.args[1][7] != nil {
		t.Fatalf("an empty decimal is null: %#v / %#v", db.args[0][7], db.args[1][7])
	}
}

func TestWriteMarksEmptyMakesNoCall(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	if err := (&store{db: db}).WriteMarks(context.Background(), nil); err != nil || db.inTx != 0 {
		t.Fatalf("err %v, transactions %d", err, db.inTx)
	}
}

func TestWriteMarksRefusesBadRows(t *testing.T) {
	t.Parallel()
	bad := []mark.PositionMark{
		sample(1, 0, mark.BasisMid, "1"), // attempt from 1
		func() mark.PositionMark { m := sample(1, 1, mark.BasisMid, "1"); m.SessionDay = "yesterday"; return m }(),
		func() mark.PositionMark { m := sample(1, 1, mark.BasisMid, "1"); m.CaptureID = ""; return m }(),
	}
	for _, m := range bad {
		db := &fakeDB{}
		err := (&store{db: db}).WriteMarks(context.Background(), []mark.PositionMark{m})
		var te contracts.TypedError
		if err == nil || !errors.As(err, &te) || te.Kind() != contracts.Permanent || db.inTx != 0 {
			t.Fatalf("%#v: want a Permanent error and no transaction, got %v", m, err)
		}
	}
}

func TestWriteMarksDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	db := &fakeDB{failAt: 2}
	err := (&store{db: db}).WriteMarks(context.Background(), []mark.PositionMark{sample(1, 1, mark.BasisMid, "1"), sample(2, 1, mark.BasisMid, "1")})
	var te contracts.TypedError
	if err == nil || !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}

func TestMarksReadsTheViewOnlyAndIsNeverNil(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	got, err := (&store{db: db}).Marks(context.Background(), "main", "2026-11-09")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("want an empty slice, got %#v, %v", got, err)
	}
	q := db.selects[0]
	if !strings.Contains(q, "rd.account_position_mark_r1") || strings.Contains(q, "account.position_mark") {
		t.Fatalf("Marks must read the view only: %s", q)
	}
}

func TestMarksMapsRows(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC)
	db := &fakeDB{rows: [][]any{{"c1", "main", at, "2026-11-09", int64(7), 2, "USD", "1.25", "mid", "1.2", "1.3", "", "", 1, "ok"}}}
	got, err := (&store{db: db}).Marks(context.Background(), "main", "2026-11-09")
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	m := got[0]
	if m.ConID != 7 || m.Attempt != 2 || m.Price != "1.25" || m.Bid != "1.2" || m.Last != "" || m.SessionDay != "2026-11-09" || m.Status != "ok" {
		t.Fatalf("%#v", m)
	}
}

func TestCloseClosesTheDatabase(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	if err := (&store{db: db}).Close(); err != nil || !db.closed {
		t.Fatal("not closed")
	}
}
