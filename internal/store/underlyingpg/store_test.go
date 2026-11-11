package underlyingpg

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// fakeDB is the query layer in memory. It knows the statements of the package by their text and answers each as
// the database would, over one table whose set_at is the fake's now.
type fakeDB struct {
	now      time.Time
	rows     []underlying.Row
	execs    int
	failExec int // the n-th insert fails (0: never)
}

func newFake() *fakeDB { return &fakeDB{now: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)} }

func (f *fakeDB) seed(r underlying.Row) underlying.Row {
	r.ID = int64(len(f.rows) + 1)
	f.now = f.now.Add(time.Minute)
	r.SetAt = f.now
	f.rows = append(f.rows, r)
	return r
}

type argScanner []any

func (a argScanner) Scan(dest ...any) error {
	for i, d := range dest {
		reflect.ValueOf(d).Elem().Set(reflect.ValueOf(a[i]))
	}
	return nil
}

func rowArgs(r underlying.Row) argScanner {
	return argScanner{r.ID, r.Key, r.Symbol, r.Market, r.Kind, r.Exchange, r.Currency, r.Name, r.Issuer, r.ConID,
		r.Status, r.Reason, r.Options, r.Fundamentals, r.BuyPrice, r.SalePrice, r.ReportDateOverride, r.Note, r.SetAt}
}

func (f *fakeDB) newest() map[string]underlying.Row {
	best := map[string]underlying.Row{}
	for _, r := range f.rows {
		if b, ok := best[r.Key]; !ok || r.ID > b.ID {
			best[r.Key] = r
		}
	}
	return best
}

func (f *fakeDB) Select(_ context.Context, sql string, args []any, scan func(scanner) error) error {
	var out []underlying.Row
	switch sql {
	case currentSQL:
		for _, r := range f.newest() {
			out = append(out, r)
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	case newestSQL:
		if r, ok := f.newest()[args[0].(string)]; ok {
			out = append(out, r)
		}
	default:
		return errors.New("unknown query " + sql)
	}
	for _, r := range out {
		if err := scan(rowArgs(r)); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeDB) Exec(_ context.Context, sql string, args ...any) error {
	switch sql {
	case lockSQL:
		return nil
	case insertSQL:
		f.execs++
		if f.failExec == f.execs {
			return errors.New("insert failed")
		}
		var conID int64
		if args[8] != nil {
			conID = args[8].(int64)
		}
		f.seed(underlying.Row{Key: args[0].(string), Symbol: args[1].(string), Market: args[2].(string),
			Kind: args[3].(string), Exchange: args[4].(string), Currency: args[5].(string), Name: args[6].(string),
			Issuer: args[7].(string), ConID: conID, Status: args[9].(string), Reason: args[10].(string),
			Options: args[11].(bool), Fundamentals: args[12].(bool), BuyPrice: args[13].(string),
			SalePrice: args[14].(string), ReportDateOverride: args[15].(string), Note: args[16].(string)})
		return nil
	}
	return errors.New("unknown statement " + sql)
}

// InTx rolls the rows back when fn fails.
func (f *fakeDB) InTx(_ context.Context, fn func(querier) error) error {
	saved := append([]underlying.Row(nil), f.rows...)
	if err := fn(f); err != nil {
		f.rows = saved
		return err
	}
	return nil
}

func (f *fakeDB) Close() error { return nil }

func newStore(f *fakeDB) *Store { return &Store{db: f} }

func seeded() (*fakeDB, *Store) {
	f := newFake()
	f.seed(underlying.Row{Key: "US:AAPL", Symbol: "AAPL", Market: "US", Kind: "stock", Exchange: "SMART", Currency: "USD",
		ConID: 265598, Status: underlying.StatusActive, Options: true})
	return f, newStore(f)
}

func TestApplyUnchangedEditWritesNothing(t *testing.T) {
	t.Parallel()
	f, s := seeded()
	n, err := s.Apply(context.Background(), underlying.Change{Edits: []underlying.Edit{
		{Key: "US:AAPL", Status: sp("active"), Note: sp(""), Exchange: sp("SMART")},
	}})
	if err != nil || n != 0 || len(f.rows) != 1 {
		t.Fatalf("Apply = %d, %v with %d rows; want 0 written", n, err, len(f.rows))
	}
}

func TestApplyAddWritesOnePending(t *testing.T) {
	t.Parallel()
	f, s := seeded()
	n, err := s.Apply(context.Background(), underlying.Change{Adds: []underlying.Add{
		{Symbol: "MSFT", Market: "US", Kind: underlying.KindStock, Options: true},
	}})
	if err != nil || n != 1 || len(f.rows) != 2 {
		t.Fatalf("Apply = %d, %v with %d rows; want 1", n, err, len(f.rows))
	}
	if r := f.rows[1]; r.Key != "US:MSFT" || r.Status != underlying.StatusPending || r.ConID != 0 || r.Exchange != "SMART" {
		t.Errorf("row = %+v; want pending US:MSFT with the default exchange", r)
	}
	if _, err := s.Apply(context.Background(), underlying.Change{Adds: []underlying.Add{
		{Symbol: "MSFT", Market: "US", Kind: underlying.KindStock},
	}}); err == nil {
		t.Error("the same name_key added twice must fail")
	}
}

func TestApplyActiveWithoutConIDRefused(t *testing.T) {
	t.Parallel()
	f, s := seeded()
	f.seed(underlying.Row{Key: "US:MSFT", Symbol: "MSFT", Market: "US", Kind: "stock", Exchange: "SMART",
		Currency: "USD", Status: underlying.StatusPending})
	n, err := s.Apply(context.Background(), underlying.Change{Edits: []underlying.Edit{{Key: "US:MSFT", Status: sp("active")}}})
	var ie underlying.InvalidError
	if !errors.As(err, &ie) || ie.Field() != "status" || n != 0 || len(f.rows) != 2 {
		t.Fatalf("Apply = %d, %v; want InvalidError on status and no row", n, err)
	}
}

func TestApplyCorrectingExchangeOfUnresolvedRowResetsIt(t *testing.T) {
	t.Parallel()
	f, s := seeded()
	f.seed(underlying.Row{Key: "US:ZZZ", Symbol: "ZZZ", Market: "US", Kind: "stock", Exchange: "SMART",
		Currency: "USD", Status: underlying.StatusUnresolved, Reason: "not listed"})
	n, err := s.Apply(context.Background(), underlying.Change{Edits: []underlying.Edit{{Key: "US:ZZZ", Exchange: sp("ARCA")}}})
	if err != nil || n != 1 {
		t.Fatalf("Apply = %d, %v", n, err)
	}
	got := f.rows[len(f.rows)-1]
	if got.Status != underlying.StatusPending || got.ConID != 0 || got.Reason != "" || got.Exchange != "ARCA" {
		t.Errorf("row = %+v; want pending, ConID cleared, exchange ARCA", got)
	}
}

func TestApplyRefusedChangeWritesNothingOfIt(t *testing.T) {
	t.Parallel()
	f, s := seeded()
	_, err := s.Apply(context.Background(), underlying.Change{
		Adds:  []underlying.Add{{Symbol: "MSFT", Market: "US", Kind: underlying.KindStock}},
		Edits: []underlying.Edit{{Key: "US:AAPL", Exchange: sp("ARCA")}},
	})
	if err == nil || len(f.rows) != 1 {
		t.Fatalf("Apply = %v with %d rows; the valid add must not be written when an edit is refused", err, len(f.rows))
	}
}

func TestApplyInsertFailureIsRolledBackAndTyped(t *testing.T) {
	t.Parallel()
	f, s := seeded()
	f.failExec = 2
	_, err := s.Apply(context.Background(), underlying.Change{Adds: []underlying.Add{
		{Symbol: "MSFT", Market: "US", Kind: underlying.KindStock},
		{Symbol: "NVDA", Market: "US", Kind: underlying.KindStock},
	}})
	if err == nil || len(f.rows) != 1 {
		t.Fatalf("Apply = %v with %d rows; want an error and the first insert rolled back", err, len(f.rows))
	}
}

func TestSettle(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		status  string
		afterID int64 // 0: the row's own id
		conID   int64
		written bool
		want    string
	}{
		{"pending listed", underlying.StatusPending, 0, 99, true, underlying.StatusActive},
		{"unresolved not listed", underlying.StatusUnresolved, 0, 0, true, underlying.StatusUnresolved},
		{"paused meanwhile", underlying.StatusPaused, 0, 99, false, underlying.StatusPaused},
		{"older id", underlying.StatusPending, 1000, 99, false, underlying.StatusPending},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFake()
			r := f.seed(underlying.Row{Key: "US:ZZZ", Symbol: "ZZZ", Market: "US", Kind: "stock", Exchange: "SMART",
				Currency: "USD", Status: tc.status})
			after := tc.afterID
			if after == 0 {
				after = r.ID
			}
			ok, err := newStore(f).Settle(context.Background(), underlying.Outcome{Key: "US:ZZZ", AfterID: after, ConID: tc.conID, Reason: "not listed"})
			if err != nil || ok != tc.written {
				t.Fatalf("Settle = %v, %v; want written %v", ok, err, tc.written)
			}
			got := f.newest()["US:ZZZ"]
			if got.Status != tc.want || (tc.written && len(f.rows) != 2) || (!tc.written && len(f.rows) != 1) {
				t.Errorf("newest = %+v with %d rows", got, len(f.rows))
			}
			if tc.written && tc.conID == 0 && got.Reason != "not listed" {
				t.Errorf("reason = %q", got.Reason)
			}
		})
	}
}
