package underlying_test

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// invalidErr is the fake's underlying.InvalidError.
type invalidErr struct{ field, why string }

func (e invalidErr) Error() string { return "underlying: " + e.field + ": " + e.why }
func (e invalidErr) Field() string { return e.field }
func (e invalidErr) Why() string   { return e.why }

var _ underlying.InvalidError = invalidErr{}

// fakeStore is an in-memory underlying.Store that keeps the promises of the interface.
type fakeStore struct {
	rows   []underlying.Row
	nextID int64
	now    time.Time
}

var _ underlying.Store = (*fakeStore)(nil)

func (f *fakeStore) insert(r underlying.Row) {
	f.nextID++
	r.ID = f.nextID
	r.SetAt = f.now
	f.now = f.now.Add(time.Minute)
	f.rows = append(f.rows, r)
}

func (f *fakeStore) newest(key string) (underlying.Row, bool) {
	var best underlying.Row
	ok := false
	for _, r := range f.rows {
		if r.Key == key && (!ok || r.ID > best.ID) {
			best, ok = r, true
		}
	}
	return best, ok
}

func (f *fakeStore) Current(context.Context) ([]underlying.Row, error) {
	keys := map[string]bool{}
	for _, r := range f.rows {
		keys[r.Key] = true
	}
	out := []underlying.Row{}
	for k := range keys {
		r, _ := f.newest(k)
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (f *fakeStore) History(_ context.Context, limit int) ([]underlying.Row, error) {
	out := append([]underlying.Row{}, f.rows...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (f *fakeStore) Apply(_ context.Context, c underlying.Change) (int, error) {
	n := 0
	for _, a := range c.Adds {
		key := a.Market + ":" + a.Symbol
		if _, ok := f.newest(key); ok {
			return 0, invalidErr{field: "symbol", why: "name exists"}
		}
		st := underlying.StatusPending
		if a.ConID > 0 {
			st = underlying.StatusActive
		}
		f.insert(underlying.Row{Key: key, Symbol: a.Symbol, Market: a.Market, Kind: a.Kind, Exchange: a.Exchange,
			Currency: a.Currency, ConID: a.ConID, Status: st, Options: a.Options, Fundamentals: a.Fundamentals})
		n++
	}
	for _, e := range c.Edits {
		r, ok := f.newest(e.Key)
		if !ok {
			return 0, invalidErr{field: "key", why: "unknown name"}
		}
		if e.Status != nil && *e.Status != r.Status {
			r.Status = *e.Status
			f.insert(r)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) Pending(ctx context.Context, retryBefore time.Time) ([]underlying.Row, error) {
	cur, _ := f.Current(ctx)
	out := []underlying.Row{}
	for _, r := range cur {
		if r.Status == underlying.StatusPending || (r.Status == underlying.StatusUnresolved && r.SetAt.Before(retryBefore)) {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeStore) Settle(_ context.Context, o underlying.Outcome) (bool, error) {
	r, ok := f.newest(o.Key)
	if !ok || r.ID != o.AfterID || (r.Status != underlying.StatusPending && r.Status != underlying.StatusUnresolved) {
		return false, nil
	}
	if o.ConID > 0 {
		r.ConID, r.Status, r.Reason = o.ConID, underlying.StatusActive, ""
	} else {
		r.Status, r.Reason = underlying.StatusUnresolved, o.Reason
	}
	f.insert(r)
	return true, nil
}

func (f *fakeStore) Close() error { return nil }

// resolveAll is a caller that holds only the Settler: it settles every pending row and counts the rows written.
func resolveAll(ctx context.Context, s underlying.Settler, conID int64) (int, error) {
	rows, err := s.Pending(ctx, time.Time{})
	if err != nil {
		return 0, err
	}
	n := 0
	for _, r := range rows {
		ok, err := s.Settle(ctx, underlying.Outcome{Key: r.Key, AfterID: r.ID, ConID: conID})
		if err != nil {
			return n, err
		}
		if ok {
			n++
		}
	}
	return n, nil
}

func newFake() *fakeStore { return &fakeStore{now: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)} }

func TestStoreContractAddResolveRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var s underlying.Store = newFake()
	n, err := s.Apply(ctx, underlying.Change{Adds: []underlying.Add{
		{Symbol: "MSFT", Market: "US", Kind: underlying.KindStock, Exchange: "SMART", Currency: "USD", Options: true},
		{Symbol: "AAPL", Market: "US", Kind: underlying.KindStock, Exchange: "SMART", Currency: "USD", Options: true},
	}})
	if err != nil || n != 2 {
		t.Fatalf("Apply = %d, %v", n, err)
	}
	if n, err := resolveAll(ctx, s, 265598); err != nil || n != 2 {
		t.Fatalf("resolveAll = %d, %v", n, err)
	}
	cur, err := s.Current(ctx)
	if err != nil || len(cur) != 2 || cur[0].Key != "US:AAPL" || cur[1].Key != "US:MSFT" {
		t.Fatalf("Current = %+v, %v; want two keys sorted", cur, err)
	}
	for _, r := range cur {
		if r.Status != underlying.StatusActive || r.ConID != 265598 {
			t.Errorf("%s = %s con_id %d; want active 265598", r.Key, r.Status, r.ConID)
		}
	}
	hist, _ := s.History(ctx, 3)
	if len(hist) != 3 || hist[0].ID <= hist[1].ID {
		t.Errorf("History = %+v; want 3 newest first", hist)
	}
	if _, err := s.Apply(ctx, underlying.Change{Adds: []underlying.Add{{Symbol: "AAPL", Market: "US"}}}); err == nil {
		t.Error("a name added twice must fail")
	} else {
		var ie underlying.InvalidError
		if !errors.As(err, &ie) || ie.Field() == "" {
			t.Errorf("error = %v; want an InvalidError naming a field", err)
		}
	}
	unknown := "paused"
	if _, err := s.Apply(ctx, underlying.Change{Edits: []underlying.Edit{{Key: "US:NOPE", Status: &unknown}}}); err == nil {
		t.Error("an unknown key must fail")
	} else {
		var ie underlying.InvalidError
		if !errors.As(err, &ie) || ie.Field() == "" {
			t.Errorf("error = %v; want an InvalidError naming a field", err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSettleOnlyPending(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	paused := underlying.StatusPaused
	for _, tc := range []struct {
		name  string
		setup func(f *fakeStore, read underlying.Row)
	}{
		{"owner paused meanwhile", func(f *fakeStore, _ underlying.Row) {
			if _, err := f.Apply(ctx, underlying.Change{Edits: []underlying.Edit{{Key: "US:AAPL", Status: &paused}}}); err != nil {
				t.Fatal(err)
			}
		}},
		{"another id", func(f *fakeStore, r underlying.Row) {
			f.insert(r) // a newer pending row of the same key
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake()
			if _, err := f.Apply(ctx, underlying.Change{Adds: []underlying.Add{{Symbol: "AAPL", Market: "US"}}}); err != nil {
				t.Fatal(err)
			}
			rows, _ := f.Pending(ctx, time.Time{})
			if len(rows) != 1 {
				t.Fatalf("pending = %d", len(rows))
			}
			tc.setup(f, rows[0])
			before := len(f.rows)
			var s underlying.Settler = f
			ok, err := s.Settle(ctx, underlying.Outcome{Key: "US:AAPL", AfterID: rows[0].ID, ConID: 1})
			if err != nil || ok {
				t.Fatalf("Settle = %v, %v; want false, nil", ok, err)
			}
			if len(f.rows) != before {
				t.Errorf("rows %d -> %d; nothing may be written", before, len(f.rows))
			}
			// the caller treats written=false as no error
			if _, err := resolveAll(ctx, f, 1); err != nil {
				t.Errorf("caller error: %v", err)
			}
		})
	}
}

func TestSettleUnresolvedRetry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFake()
	_, _ = f.Apply(ctx, underlying.Change{Adds: []underlying.Add{{Symbol: "ZZZZ", Market: "US"}}})
	rows, _ := f.Pending(ctx, time.Time{})
	if ok, err := f.Settle(ctx, underlying.Outcome{Key: "US:ZZZZ", AfterID: rows[0].ID, Reason: "not listed"}); !ok || err != nil {
		t.Fatalf("Settle = %v, %v", ok, err)
	}
	cur, _ := f.Current(ctx)
	if cur[0].Status != underlying.StatusUnresolved || cur[0].Reason != "not listed" || cur[0].ConID != 0 {
		t.Fatalf("row = %+v", cur[0])
	}
	if p, _ := f.Pending(ctx, cur[0].SetAt); len(p) != 0 {
		t.Errorf("unresolved row younger than the retry bound is pending: %+v", p)
	}
	if p, _ := f.Pending(ctx, cur[0].SetAt.Add(time.Second)); len(p) != 1 {
		t.Errorf("unresolved row older than the retry bound is not pending")
	}
}

func TestDSNEnvResolveName(t *testing.T) {
	t.Parallel()
	if underlying.DSNEnvResolve != "OT_PG_DSN_UNDERLYING_RESOLVE" {
		t.Errorf("DSNEnvResolve = %q", underlying.DSNEnvResolve)
	}
}
