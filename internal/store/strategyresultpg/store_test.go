package strategyresultpg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

// fakeDB is an in-memory database seam: rows keyed by the primary key, a transaction that commits or discards its
// writes, and a Select that filters by the day and strategy arguments and returns rows in insertion order.
type fakeDB struct {
	rows     []strategyresult.Row
	calls    int
	commits  int
	rollback int
	failKey  string // an Exec of a row with this key fails
}

var errBoom = errors.New("boom")

type fakeTx struct {
	db      *fakeDB
	pending []strategyresult.Row
}

func (t *fakeTx) Exec(_ context.Context, _ string, a ...any) (int64, error) {
	key := a[5].(string)
	if key == t.db.failKey {
		return 0, errBoom
	}
	r := strategyresult.Row{SessionDay: contracts.Day(a[0].(time.Time).Format(time.DateOnly)), RunID: a[1].(string),
		Strategy: a[2].(string), RuleVersion: a[3].(string), ParamHash: a[4].(string), Key: key,
		UnderlyingID: a[6].(int), Status: a[7].(string), Rank: a[8].(*int), RankInUnderlying: a[9].(*int),
		Legs: a[10].(string), Checks: a[11].(string), Facts: a[12].(string), ComputedAt: a[13].(time.Time),
		SettingsVersion: a[14].(*int64)}
	for _, e := range append(append([]strategyresult.Row{}, t.db.rows...), t.pending...) {
		if e.SessionDay == r.SessionDay && e.RunID == r.RunID && e.Strategy == r.Strategy && e.Key == r.Key {
			return 0, nil
		}
	}
	t.pending = append(t.pending, r)
	return 1, nil
}

func (f *fakeDB) InTx(_ context.Context, fn func(execer) error) error {
	f.calls++
	tx := &fakeTx{db: f}
	if err := fn(tx); err != nil {
		f.rollback++
		return err
	}
	f.commits++
	f.rows = append(f.rows, tx.pending...)
	return nil
}

func (f *fakeDB) Select(_ context.Context, _ string, args []any, scan func(scanner) error) error {
	f.calls++
	day := contracts.Day(args[0].(time.Time).Format(time.DateOnly))
	for _, r := range f.rows {
		if r.SessionDay == day && r.Strategy == args[1].(string) {
			if err := scan(rowScanner{r}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

// rowScanner hands the stored row to the destinations of scanRow.
type rowScanner struct{ r strategyresult.Row }

func (s rowScanner) Scan(dest ...any) error {
	t, _ := time.Parse(time.DateOnly, string(s.r.SessionDay))
	*dest[0].(*time.Time) = t
	*dest[1].(*string) = s.r.RunID
	*dest[2].(*string) = s.r.Strategy
	*dest[3].(*string) = s.r.RuleVersion
	*dest[4].(*string) = s.r.ParamHash
	*dest[5].(*string) = s.r.Key
	*dest[6].(*int) = s.r.UnderlyingID
	*dest[7].(*string) = s.r.Status
	*dest[8].(**int) = s.r.Rank
	*dest[9].(**int) = s.r.RankInUnderlying
	*dest[10].(*string) = s.r.Legs
	*dest[11].(*string) = s.r.Checks
	*dest[12].(*string) = s.r.Facts
	*dest[13].(*time.Time) = s.r.ComputedAt
	*dest[14].(**int64) = s.r.SettingsVersion
	return nil
}

func ip(n int) *int { return &n }

func newFake() (*resultStore, *fakeDB) {
	db := &fakeDB{}
	return &resultStore{db: db}, db
}

var t0 = time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)

func mkRow(run, key string, rank *int, at time.Time) strategyresult.Row {
	return strategyresult.Row{SessionDay: "2026-11-13", RunID: run, Strategy: "short_put", RuleVersion: "put_v1",
		Key: key, Status: "ok", Rank: rank, Legs: "[]", Checks: "[]", Facts: "{}", ComputedAt: at}
}

func TestInsertResultsNoRowsNoCall(t *testing.T) {
	t.Parallel()
	st, db := newFake()
	n, err := st.InsertResults(context.Background(), nil)
	if err != nil || n != 0 || db.calls != 0 {
		t.Fatalf("n=%d err=%v calls=%d, want 0, nil, 0", n, err, db.calls)
	}
}

func TestInsertResultsOneTransactionAllOrNothing(t *testing.T) {
	t.Parallel()
	st, db := newFake()
	db.failKey = "2"
	rows := []strategyresult.Row{mkRow("r1", "1", nil, t0), mkRow("r1", "2", nil, t0), mkRow("r1", "3", nil, t0)}
	n, err := st.InsertResults(context.Background(), rows)
	if err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, want 0 and an error", n, err)
	}
	var te contracts.TypedError
	if !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Errorf("error %v is not Retryable", err)
	}
	if db.rollback != 1 || db.commits != 0 || len(db.rows) != 0 {
		t.Errorf("rollback=%d commits=%d kept=%d, want 1, 0, 0", db.rollback, db.commits, len(db.rows))
	}
	db.failKey = ""
	if n, err := st.InsertResults(context.Background(), rows); err != nil || n != 3 || db.commits != 1 {
		t.Fatalf("clean insert n=%d err=%v commits=%d", n, err, db.commits)
	}
	if n, _ := st.InsertResults(context.Background(), rows); n != 0 {
		t.Errorf("repeat inserted %d, want 0", n)
	}
}

func TestResultsNewestRunWins(t *testing.T) {
	t.Parallel()
	st, _ := newFake()
	ctx := context.Background()
	_, _ = st.InsertResults(ctx, []strategyresult.Row{mkRow("old", "1", nil, t0), mkRow("old", "2", nil, t0)})
	_, _ = st.InsertResults(ctx, []strategyresult.Row{mkRow("new", "3", nil, t0.Add(time.Hour))})
	got, err := st.Results(ctx, "2026-11-13", "short_put")
	if err != nil || len(got) != 1 || got[0].RunID != "new" {
		t.Fatalf("got %+v, %v, want the one row of run new", got, err)
	}
	none, err := st.Results(ctx, "2026-11-12", "short_put")
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("empty day = %#v, %v, want empty non-nil", none, err)
	}
}

func TestResultsRankedFirst(t *testing.T) {
	t.Parallel()
	st, _ := newFake()
	ctx := context.Background()
	_, _ = st.InsertResults(ctx, []strategyresult.Row{
		mkRow("r", "9", nil, t0), mkRow("r", "10", ip(2), t0), mkRow("r", "20", ip(1), t0), mkRow("r", "2", nil, t0),
		mkRow("r", "5", ip(2), t0)})
	got, err := st.Results(ctx, "2026-11-13", "short_put")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"20", "5", "10", "2", "9"} // rank 1, rank 2 by numeric id (5 before 10), unranked by id
	if len(got) != len(want) {
		t.Fatalf("got %d rows", len(got))
	}
	for i, k := range want {
		if got[i].Key != k {
			t.Errorf("row %d key %s, want %s", i, got[i].Key, k)
		}
	}
}

func TestResultsRefuseZeroDay(t *testing.T) {
	t.Parallel()
	st, db := newFake()
	for _, c := range []struct {
		day      contracts.Day
		strategy string
	}{{"", "short_put"}, {"2026-11-13", ""}} {
		_, err := st.Results(context.Background(), c.day, c.strategy)
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("Results(%q, %q) err = %v, want Permanent", c.day, c.strategy, err)
		}
	}
	if db.calls != 0 {
		t.Errorf("database called %d times", db.calls)
	}
}
