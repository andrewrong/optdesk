package ownerset_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

// fakeStore is an in-memory ownerset.Store that keeps the promises of the interface.
type fakeStore struct {
	params []paramRow
	excl   []exclRow
	nextID int64
	clock  time.Time
	closed bool
}

type paramRow struct {
	id int64
	ownerset.Param
}

type exclRow struct {
	id int64
	ownerset.Exclusion
}

var _ ownerset.Store = (*fakeStore)(nil)

func (f *fakeStore) Params(_ context.Context, at time.Time) ([]ownerset.Param, error) {
	newest := map[string]paramRow{}
	for _, r := range f.params {
		if r.SetAt.After(at) {
			continue
		}
		if o, ok := newest[r.Key]; !ok || r.id > o.id {
			newest[r.Key] = r
		}
	}
	out := []ownerset.Param{}
	for _, r := range newest {
		out = append(out, r.Param)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (f *fakeStore) Exclusions(_ context.Context, at time.Time) ([]ownerset.Exclusion, error) {
	newest := map[int64]exclRow{}
	for _, r := range f.excl {
		if r.SetAt.After(at) {
			continue
		}
		if o, ok := newest[r.InstrumentID]; !ok || r.id > o.id {
			newest[r.InstrumentID] = r
		}
	}
	out := []ownerset.Exclusion{}
	for _, r := range newest {
		out = append(out, r.Exclusion)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].InstrumentID < out[j].InstrumentID })
	return out, nil
}

func (f *fakeStore) History(_ context.Context, n int) ([]ownerset.HistoryRow, error) {
	type item struct {
		id  int64
		row ownerset.HistoryRow
	}
	var all []item
	for _, r := range f.params {
		all = append(all, item{r.id, ownerset.HistoryRow{Kind: "param", Key: r.Key, Value: r.Value, SetAt: r.SetAt}})
	}
	for _, r := range f.excl {
		all = append(all, item{r.id, ownerset.HistoryRow{Kind: "exclusion", InstrumentID: r.InstrumentID, Excluded: r.Excluded, SetAt: r.SetAt}})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].id > all[j].id })
	out := []ownerset.HistoryRow{}
	for i := 0; i < len(all) && i < n; i++ {
		out = append(out, all[i].row)
	}
	return out, nil
}

func (f *fakeStore) Version(_ context.Context, at time.Time) (int64, error) {
	var v int64
	for _, r := range f.params {
		if !r.SetAt.After(at) && r.id > v {
			v = r.id
		}
	}
	for _, r := range f.excl {
		if !r.SetAt.After(at) && r.id > v {
			v = r.id
		}
	}
	return v, nil
}

func (f *fakeStore) Close() error { f.closed = true; return nil }

func (f *fakeStore) Apply(ctx context.Context, c ownerset.Change) (int, error) {
	f.clock = f.clock.Add(time.Hour)
	curP, _ := f.Params(ctx, f.clock)
	curE, _ := f.Exclusions(ctx, f.clock)
	n := 0
	for _, p := range c.Params {
		same := false
		for _, o := range curP {
			if o.Key == p.Key && o.Value == p.Value {
				same = true
			}
		}
		if same {
			continue
		}
		f.nextID++
		f.params = append(f.params, paramRow{f.nextID, ownerset.Param{Key: p.Key, Value: p.Value, SetAt: f.clock}})
		n++
	}
	for _, e := range c.Exclusions {
		same := false
		for _, o := range curE {
			if o.InstrumentID == e.InstrumentID && o.Excluded == e.Excluded {
				same = true
			}
		}
		if same {
			continue
		}
		f.nextID++
		f.excl = append(f.excl, exclRow{f.nextID, ownerset.Exclusion{InstrumentID: e.InstrumentID, Excluded: e.Excluded, SetAt: f.clock}})
		n++
	}
	return n, nil
}

func TestStoreContractApplyTwiceAndAsOf(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	var open ownerset.OpenFunc = func(context.Context, string) (ownerset.Store, error) {
		return &fakeStore{clock: base}, nil
	}
	s, err := open(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	before := base
	c := ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "20"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 7, Excluded: true}},
	}
	n, err := s.Apply(ctx, c)
	if err != nil || n != 2 {
		t.Fatalf("first apply = %d, %v; want 2", n, err)
	}
	after := base.Add(time.Hour)
	n, err = s.Apply(ctx, c)
	if err != nil || n != 0 {
		t.Fatalf("second apply = %d, %v; want 0 (unchanged fields insert nothing)", n, err)
	}
	if v, _ := s.Version(ctx, before); v != 0 {
		t.Fatalf("version before = %d, want 0", v)
	}
	if v, _ := s.Version(ctx, after.Add(24*time.Hour)); v != 2 {
		t.Fatalf("version after = %d, want 2", v)
	}
	if p, _ := s.Params(ctx, before); len(p) != 0 {
		t.Fatalf("params before = %v, want none", p)
	}
	p, _ := s.Params(ctx, after.Add(24*time.Hour))
	if len(p) != 1 || p[0].Key != "min_dte" || p[0].Value != "20" {
		t.Fatalf("params after = %v", p)
	}
	if e, _ := s.Exclusions(ctx, before); len(e) != 0 {
		t.Fatalf("exclusions before = %v, want none", e)
	}
	e, _ := s.Exclusions(ctx, after.Add(24*time.Hour))
	if len(e) != 1 || e[0].InstrumentID != 7 || !e[0].Excluded {
		t.Fatalf("exclusions after = %v", e)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreContractHistoryNewestFirst(t *testing.T) {
	t.Parallel()
	f := &fakeStore{clock: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	var w ownerset.Writer = f
	var r ownerset.Reader = f
	ctx := context.Background()
	_, _ = w.Apply(ctx, ownerset.Change{Params: []ownerset.ParamValue{{Key: "a", Value: "1"}}})
	_, _ = w.Apply(ctx, ownerset.Change{Exclusions: []ownerset.ExclusionValue{{InstrumentID: 3, Excluded: true}}})
	_, _ = w.Apply(ctx, ownerset.Change{Params: []ownerset.ParamValue{{Key: "a", Value: "2"}}})
	h, err := r.History(ctx, 2)
	if err != nil || len(h) != 2 {
		t.Fatalf("history = %v, %v", h, err)
	}
	if h[0].Kind != "param" || h[0].Value != "2" || h[1].Kind != "exclusion" || h[1].InstrumentID != 3 {
		t.Fatalf("history order wrong: %v", h)
	}
}

func TestDSNEnvSettingsName(t *testing.T) {
	t.Parallel()
	if ownerset.DSNEnvSettings != "OT_PG_DSN_SETTINGS" {
		t.Fatalf("DSNEnvSettings = %q", ownerset.DSNEnvSettings)
	}
}
