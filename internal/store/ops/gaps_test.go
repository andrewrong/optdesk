package ops

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/gaphistory"
)

var gapSince = time.Date(2026, 11, 8, 8, 0, 0, 0, time.UTC)

// gapRow is one row of the seven view columns as the database returns them.
func gapRow(job, dataset, day string, first time.Time, cleared *time.Time) []any {
	d, _ := time.Parse(time.DateOnly, day)
	var reported *time.Time
	return []any{job, dataset, d, "expired", first, reported, cleared}
}

func gapsSince(t *testing.T, db *recDB) []gaphistory.Gap {
	t.Helper()
	got, err := (&gapReader{db: db}).GapsSince(context.Background(), gapSince)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestGapsSinceEmptyIsNotNil(t *testing.T) {
	t.Parallel()
	got := gapsSince(t, &recDB{})
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want empty non-nil slice", got)
	}
}

func TestGapsSinceClearedAfterSince(t *testing.T) {
	t.Parallel()
	cleared := gapSince.Add(time.Hour)
	first := gapSince.Add(-48 * time.Hour)
	got := gapsSince(t, &recDB{fakeDB: fakeDB{rows: [][]any{gapRow("", "obs.series_point", "2026-11-05", first, &cleared)}}})
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	g := got[0]
	if !g.ClearedAt.Equal(cleared) || !g.FirstSeenAt.Equal(first) || !g.ReportedAt.IsZero() ||
		g.DatasetID != "obs.series_point" || g.Day != "2026-11-05" || g.Reason != "expired" {
		t.Errorf("gap = %+v", g)
	}
}

func TestGapsSinceOpenFirstSeenAfterSince(t *testing.T) {
	t.Parallel()
	first := gapSince.Add(time.Minute)
	db := &recDB{fakeDB: fakeDB{rows: [][]any{gapRow("fred", "", "2026-11-08", first, nil)}}}
	got := gapsSince(t, db)
	if len(got) != 1 || got[0].JobID != "fred" || !got[0].ClearedAt.IsZero() || !got[0].FirstSeenAt.Equal(first) {
		t.Fatalf("got %+v", got)
	}
	if len(db.selArgs[0]) != 1 || !db.selArgs[0][0].(time.Time).Equal(gapSince) {
		t.Errorf("args = %v, want the since instant", db.selArgs[0])
	}
}

func TestGapsSinceOrder(t *testing.T) {
	t.Parallel()
	t1 := gapSince.Add(time.Hour)
	t2 := gapSince.Add(2 * time.Hour)
	db := &recDB{fakeDB: fakeDB{rows: [][]any{
		gapRow("", "b", "2026-11-05", t2, nil),
		gapRow("", "b", "2026-11-04", t1, nil),
		gapRow("", "a", "2026-11-04", t1, nil),
		gapRow("", "a", "2026-11-03", t1, nil),
	}}}
	got := gapsSince(t, db)
	want := []struct{ ds, day string }{{"a", "2026-11-03"}, {"a", "2026-11-04"}, {"b", "2026-11-04"}, {"b", "2026-11-05"}}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, w := range want {
		if got[i].DatasetID != w.ds || string(got[i].Day) != w.day {
			t.Errorf("row %d = %s %s, want %s %s", i, got[i].DatasetID, got[i].Day, w.ds, w.day)
		}
	}
}
