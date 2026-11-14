package macroeventpg

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

// fakeDB records the statements; Select answers the stored latest date with stored.
type fakeDB struct {
	stored string
	execs  []string
	args   [][]any
	inTx   int
	failAt int // 1-based Exec that fails; 0 never
}

type fakeScan []any

func (f fakeScan) Scan(dest ...any) error {
	for i, d := range dest {
		if p, ok := d.(*string); ok {
			*p = f[i].(string)
		}
	}
	return nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(tx) error) error {
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

func (f *fakeDB) Select(_ context.Context, _ string, _ []any, scan func(scanner) error) error {
	return scan(fakeScan{f.stored})
}

func (f *fakeDB) Close() error { return nil }

func ev(date string) macroevent.Event {
	return macroevent.Event{Kind: "cpi", Source: "fred", SourceRef: "10", Date: contracts.Day(date), RunDay: "2026-11-14"}
}

// has returns the index of the first statement equal to sql, or -1.
func (f *fakeDB) has(sql string) int {
	for i, q := range f.execs {
		if q == sql {
			return i
		}
	}
	return -1
}

func TestReplaceFutureEmptyAnswerDeletesNothing(t *testing.T) {
	t.Parallel()
	db := &fakeDB{stored: "2026-12-10"}
	out, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Held || out.Deleted != 0 || db.has(deleteSQL) >= 0 {
		t.Fatalf("empty answer must delete nothing: %+v %v", out, db.execs)
	}
	i := db.has(holdSQL)
	if i < 0 || db.args[i][0] != "cpi" || db.args[i][1] != "2026-11-14" {
		t.Fatalf("hold of the run day expected: %v %v", db.execs, db.args)
	}
}

func TestReplaceFutureShorterAnswerHeld(t *testing.T) {
	t.Parallel()
	db := &fakeDB{stored: "2026-12-10"}
	out, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14",
		[]macroevent.Event{ev("2026-11-20")})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Held || out.Written != 1 || db.has(deleteSQL) >= 0 || db.has(clearSQL) >= 0 {
		t.Fatalf("shorter answer keeps the stored rows: %+v %v", out, db.execs)
	}
	if i := db.has(holdSQL); i < 0 || db.args[i][1] != "2026-11-14" {
		t.Fatalf("held_day must be the run day: %v %v", db.execs, db.args)
	}
}

func TestReplaceFutureMovedLastDateHeld(t *testing.T) {
	t.Parallel()
	db := &fakeDB{stored: "2026-12-10"}
	out, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14",
		[]macroevent.Event{ev("2026-11-20"), ev("2026-12-05")})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Held || out.Written != 2 || out.Deleted != 0 || db.has(deleteSQL) >= 0 {
		t.Fatalf("a last date moved earlier deletes nothing: %+v %v", out, db.execs)
	}
	if db.has(holdSQL) < 0 {
		t.Fatalf("hold expected: %v", db.execs)
	}
}

func TestReplaceFutureKeepsRunDayRow(t *testing.T) {
	t.Parallel()
	db := &fakeDB{stored: "2026-12-10"}
	out, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14",
		[]macroevent.Event{ev("2026-11-20"), ev("2026-12-10")})
	if err != nil {
		t.Fatal(err)
	}
	i := db.has(deleteSQL)
	if i < 0 || out.Held {
		t.Fatalf("a passing delete expected: %+v %v", out, db.execs)
	}
	// The delete takes the run day as its date bound, so a row dated the run day (2026-11-14) is outside it: the
	// statement keeps rows with event_date <= $3.
	if db.args[i][2] != "2026-11-14" {
		t.Fatalf("delete bound must be the run day: %v", db.args[i])
	}
	if dates := db.args[i][3].([]string); len(dates) != 2 || dates[0] != "2026-11-20" || dates[1] != "2026-12-10" {
		t.Fatalf("the answer dates are the kept set: %v", dates)
	}
}

func TestReplaceFutureDeletesMovedDate(t *testing.T) {
	t.Parallel()
	db := &fakeDB{stored: "2026-12-10"}
	out, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14",
		[]macroevent.Event{ev("2026-11-25"), ev("2026-12-10")})
	if err != nil {
		t.Fatal(err)
	}
	if out.Held || out.Deleted != 1 || db.inTx != 1 {
		t.Fatalf("one transaction with a delete: %+v tx %d", out, db.inTx)
	}
	d, c := db.has(deleteSQL), db.has(clearSQL)
	if d < 0 || c < d || db.has(holdSQL) >= 0 {
		t.Fatalf("delete then clear, no hold: %v", db.execs)
	}
	if db.args[c][0] != "cpi" {
		t.Fatalf("held_day is cleared for the kind: %v", db.args[c])
	}
}

func TestReplaceFutureRefusesBadInput(t *testing.T) {
	t.Parallel()
	bad := []macroevent.Event{
		{Kind: "ppi", Source: "fred", Date: "2026-11-20"},
		{Kind: "cpi", Source: "file", Date: "2026-11-20"},
		{Kind: "cpi", Source: "fred", Date: "soon"},
	}
	for _, e := range bad {
		db := &fakeDB{}
		_, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14", []macroevent.Event{e})
		var te contracts.TypedError
		if err == nil || !errors.As(err, &te) || te.Kind() != contracts.Permanent || db.inTx != 0 {
			t.Fatalf("%+v: want Permanent and no transaction, got %v", e, err)
		}
	}
}

func TestReplaceFutureDatabaseErrorIsRetryable(t *testing.T) {
	t.Parallel()
	db := &fakeDB{failAt: 1}
	_, err := (&store{db: db}).ReplaceFuture(context.Background(), "cpi", "fred", "2026-11-14", []macroevent.Event{ev("2026-11-20")})
	var te contracts.TypedError
	if err == nil || !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("want Retryable, got %v", err)
	}
}
