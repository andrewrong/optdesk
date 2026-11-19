package reconpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

// fakeTx records the statements of a transaction and answers the run insert with a fixed id.
type fakeTx struct {
	sqls   []string
	args   [][]any
	failAt int // 1-based statement that fails; 0 never
}

func (f *fakeTx) record(sql string, args []any) error {
	f.sqls = append(f.sqls, sql)
	f.args = append(f.args, args)
	if f.failAt == len(f.sqls) {
		return errors.New("boom")
	}
	return nil
}

func (f *fakeTx) InsertReturningID(_ context.Context, sql string, args ...any) (int64, error) {
	return 42, f.record(sql, args)
}

func (f *fakeTx) Exec(_ context.Context, sql string, args ...any) error { return f.record(sql, args) }

// fakeDB runs the function of InTx on one fakeTx and counts the transactions.
type fakeDB struct {
	tx     fakeTx
	inTx   int
	closed bool
}

func (f *fakeDB) InTx(_ context.Context, fn func(tx) error) error {
	f.inTx++
	return fn(&f.tx)
}

func (f *fakeDB) Select(context.Context, string, []any, func(scanner) error) error { return nil }

func (f *fakeDB) Close() error { f.closed = true; return nil }

func sampleRun(items ...reconstore.Item) reconstore.Run {
	at := time.Date(2026, 11, 18, 22, 0, 0, 0, time.UTC)
	return reconstore.Run{PairID: "R6", SessionDay: "2026-11-18", Status: "ok", StartedAt: at, FinishedAt: at.Add(time.Minute),
		N: 3, Matched: 2, Diff: 1, PairsAsked: 1, PairsCompared: 1, ToleranceText: "0.1%", SourceID: "yfinance", Items: items}
}

func TestInsertRunUsesOneTransaction(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	w := &store{db: db}
	id, err := w.InsertRun(context.Background(), sampleRun(
		reconstore.Item{Key: "AAPL|2026-11-18", Company: "AAPL", Kind: "diff"},
		reconstore.Item{Key: "MSFT|2026-11-18", Company: "MSFT", Kind: "skipped", Reason: "source_lacks"}))
	if err != nil || id != 42 {
		t.Fatalf("InsertRun = %d, %v", id, err)
	}
	if db.inTx != 1 {
		t.Fatalf("%d transactions, want 1", db.inTx)
	}
	if len(db.tx.sqls) != 2 || !strings.Contains(db.tx.sqls[0], "INSERT INTO recon.run") ||
		!strings.Contains(db.tx.sqls[1], "INSERT INTO recon.item") {
		t.Fatalf("statements = %v", db.tx.sqls)
	}
	for _, s := range db.tx.sqls {
		if strings.Contains(strings.ToUpper(s), "UPDATE") {
			t.Errorf("an update: %s", s)
		}
	}
	if got := db.tx.args[1][0]; got != int64(42) {
		t.Errorf("item run_id = %v, want the new run id", got)
	}
}

func TestInsertRunWithoutItemsInsertsOnlyTheRun(t *testing.T) {
	t.Parallel()
	db := &fakeDB{}
	if _, err := (&store{db: db}).InsertRun(context.Background(), sampleRun()); err != nil {
		t.Fatal(err)
	}
	if len(db.tx.sqls) != 1 {
		t.Fatalf("statements = %v", db.tx.sqls)
	}
}

func TestInsertRunRefusesBadItemsBeforeTheDatabase(t *testing.T) {
	t.Parallel()
	many := make([]reconstore.Item, reconstore.RunItemsMax+1)
	for i := range many {
		many[i] = reconstore.Item{Key: "k", Kind: "diff"}
	}
	for name, items := range map[string][]reconstore.Item{
		"over the cap": many,
		"a match":      {{Key: "k", Kind: "match"}},
		"unknown kind": {{Key: "k", Kind: "weird"}},
	} {
		db := &fakeDB{}
		if _, err := (&store{db: db}).InsertRun(context.Background(), sampleRun(items...)); err == nil {
			t.Errorf("%s: no error", name)
		}
		if db.inTx != 0 {
			t.Errorf("%s: the database was used", name)
		}
	}
	at := make([]reconstore.Item, reconstore.RunItemsMax)
	for i := range at {
		at[i] = reconstore.Item{Key: "k", Kind: "skipped"}
	}
	if _, err := (&store{db: &fakeDB{}}).InsertRun(context.Background(), sampleRun(at...)); err != nil {
		t.Errorf("exactly the cap: %v", err)
	}
}

func TestInsertRunErrorsPass(t *testing.T) {
	t.Parallel()
	db := &fakeDB{tx: fakeTx{failAt: 2}}
	if _, err := (&store{db: db}).InsertRun(context.Background(), sampleRun(reconstore.Item{Key: "k", Kind: "diff"})); err == nil {
		t.Fatal("no error when the item insert fails")
	}
}
