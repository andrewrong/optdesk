package seriesdef_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

type key struct{ id, date string }

// fakeStore keeps the promises of the interfaces: upsert replaces a differing
// row, fill never replaces a present value.
type fakeStore struct {
	rows map[key]contracts.SeriesPoint
	defs []seriesdef.SeriesDefinition
}

var (
	_ seriesdef.SeriesUpserter = (*fakeStore)(nil)
	_ seriesdef.SeriesFiller   = (*fakeStore)(nil)
	_ seriesdef.SeriesRegistry = (*fakeStore)(nil)
)

func same(a, b contracts.SeriesPoint) bool {
	if a.Status != b.Status || (a.Value == nil) != (b.Value == nil) {
		return false
	}
	return a.Value == nil || *a.Value == *b.Value
}

func (f *fakeStore) UpsertSeriesPoints(_ context.Context, rows []contracts.SeriesPoint, _ string) (seriesdef.WriteCount, error) {
	var c seriesdef.WriteCount
	if f.rows == nil {
		f.rows = map[key]contracts.SeriesPoint{}
	}
	for _, r := range rows {
		k := key{r.SeriesID, r.ObservationDate.String()}
		if old, ok := f.rows[k]; ok && same(old, r) {
			c.Unchanged++
			continue
		}
		f.rows[k] = r
		c.Written++
	}
	return c, nil
}

func (f *fakeStore) FillSeriesPoints(_ context.Context, rows []contracts.SeriesPoint, _ string) (seriesdef.WriteCount, error) {
	var c seriesdef.WriteCount
	if f.rows == nil {
		f.rows = map[key]contracts.SeriesPoint{}
	}
	for _, r := range rows {
		k := key{r.SeriesID, r.ObservationDate.String()}
		if old, ok := f.rows[k]; ok && old.Status == "present" {
			c.Unchanged++
			continue
		}
		f.rows[k] = r
		c.Written++
	}
	return c, nil
}

func (f *fakeStore) ActiveSeries(context.Context) ([]seriesdef.SeriesDefinition, error) {
	var out []seriesdef.SeriesDefinition
	for _, d := range f.defs {
		if d.Active {
			out = append(out, d)
		}
	}
	return out, nil
}

func point(t *testing.T, v float64) contracts.SeriesPoint {
	t.Helper()
	var p contracts.SeriesPoint
	p.SeriesID = "DGS10"
	p.Status = "present"
	p.Value = &v
	return p
}

func TestSeriesUpserterContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var u seriesdef.SeriesUpserter = &fakeStore{}

	got, err := u.UpsertSeriesPoints(ctx, nil, "fred_series:2026-11-13")
	if err != nil || got != (seriesdef.WriteCount{}) {
		t.Fatalf("empty list: got %+v, %v; want zero counts", got, err)
	}

	p := point(t, 4.1)
	got, err = u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{p}, "w")
	if err != nil || got.Written != 1 || got.Unchanged != 0 {
		t.Fatalf("first write: got %+v, %v", got, err)
	}
	// One equal row and one differing row of the same key family.
	q := point(t, 4.2)
	got, err = u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{p, q}, "w")
	if err != nil || got.Written != 1 || got.Unchanged != 1 {
		t.Fatalf("equal+differing: got %+v, %v; want Written 1 Unchanged 1", got, err)
	}
}

func TestSeriesFillerContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := &fakeStore{}
	if _, err := f.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, 1)}, "w"); err != nil {
		t.Fatal(err)
	}
	got, err := f.FillSeriesPoints(ctx, []contracts.SeriesPoint{point(t, 2)}, "w")
	if err != nil || got.Written != 0 || got.Unchanged != 1 {
		t.Fatalf("fill over present: got %+v, %v", got, err)
	}
}

func TestSeriesRegistryContract(t *testing.T) {
	t.Parallel()
	var r seriesdef.SeriesRegistry = &fakeStore{defs: []seriesdef.SeriesDefinition{
		{SeriesID: "SOFR", Category: "rates", Frequency: "daily", Source: "fred", SecondarySource: "nyfed", Unit: "percent", Active: true},
		{SeriesID: "OLD", Active: false},
	}}
	got, err := r.ActiveSeries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].SeriesID != "SOFR" || got[0].SecondarySource != "nyfed" {
		t.Fatalf("got %+v; want only the active SOFR row", got)
	}
}
