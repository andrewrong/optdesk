//go:build pg

package series

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

// upserterAs opens the series store as the writer role and inserts the series row DGS10 (a fresh test database
// has none of the 12 the migration fills on a real one).
func upserterAs(t *testing.T) (string, store.SeriesStore) {
	t.Helper()
	ctx := context.Background()
	name := newDatabase(t)
	if _, err := connectAs(t, name, "ot_w_series_fred").Exec(ctx, `INSERT INTO obs.series_definition (series_id, unit) VALUES ('DGS10', 'percent')`); err != nil {
		t.Fatal(err)
	}
	w, err := Open(ctx, connURL(t, name, "ot_w_series_fred"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return name, w
}

func upserter(t *testing.T, w store.SeriesStore) seriesdef.SeriesUpserter {
	t.Helper()
	u, ok := w.(seriesdef.SeriesUpserter)
	if !ok {
		t.Fatal("store has no UpsertSeriesPoints")
	}
	return u
}

func filler(t *testing.T, w store.SeriesStore) seriesdef.SeriesFiller {
	t.Helper()
	u, ok := w.(seriesdef.SeriesFiller)
	if !ok {
		t.Fatal("store has no FillSeriesPoints")
	}
	return u
}

// storedValue reads the value and status of the DGS10 row of 2026-11-04 as the admin.
func storedValue(t *testing.T, name string) (*float64, string) {
	t.Helper()
	var v *float64
	var status string
	err := connectAs(t, name, "").QueryRow(context.Background(),
		`SELECT value, status FROM obs.series_point WHERE series_id = 'DGS10' AND observation_date = '2026-11-04'`).Scan(&v, &status)
	if err != nil {
		t.Fatal(err)
	}
	return v, status
}

func TestUpsertReplacesDifferingAndKeepsEqualPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, w := upserterAs(t)
	u := upserter(t, w)
	v1, v2 := 4.25, 4.30
	got, err := u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v1, "present")}, "run1")
	if err != nil || got.Written != 1 || got.Unchanged != 0 {
		t.Fatalf("first: %+v, %v", got, err)
	}
	got, err = u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v1, "present")}, "run2")
	if err != nil || got.Written != 0 || got.Unchanged != 1 {
		t.Fatalf("equal: %+v, %v", got, err)
	}
	var by string
	if err := connectAs(t, name, "").QueryRow(ctx, `SELECT written_by FROM obs.series_point`).Scan(&by); err != nil || by != "run1" {
		t.Errorf("written_by after an equal row = %q, %v; want run1", by, err)
	}
	got, err = u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v2, "present")}, "run3")
	if err != nil || got.Written != 1 || got.Unchanged != 0 {
		t.Fatalf("differing: %+v, %v", got, err)
	}
	if v, _ := storedValue(t, name); v == nil || *v != 4.30 {
		t.Errorf("value = %v, want 4.30", v)
	}
	if n := count(t, name); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
}

func TestInsertSeriesPointsAddsNoRowForExistingDatePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, w := upserterAs(t)
	v1, v2 := 4.25, 4.30
	if err := w.InsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v1, "present")}); err != nil {
		t.Fatal(err)
	}
	later := point(t, &v2, "present")
	later.RealtimeStart = date(t, "2026-11-10")
	if err := w.InsertSeriesPoints(ctx, []contracts.SeriesPoint{later}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, name); n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
	if v, _ := storedValue(t, name); v == nil || *v != 4.25 {
		t.Errorf("value = %v, want the first 4.25", v)
	}
}

func TestMigrationKeepsNewestVintagePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabaseWith(t, "0002_series.sql", "0003_ops_ledger.sql", "0014_series_unit_index_points.sql")
	admin := connectAs(t, name, "")
	for _, q := range []string{
		`INSERT INTO obs.series_definition VALUES ('DGS10', 'percent', '')`,
		`INSERT INTO obs.series_point VALUES ('DGS10','2026-11-04','2026-11-05','2026-11-09',4.1,'present','percent','',now(),'fred','r1')`,
		`INSERT INTO obs.series_point VALUES ('DGS10','2026-11-04','2026-11-10','9999-12-31',4.3,'present','percent','',now(),'fred','r2')`,
		`INSERT INTO obs.series_point VALUES ('DGS10','2026-11-03','2026-11-05','9999-12-31',4.0,'present','percent','',now(),'fred','r1')`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	applyMigration(t, connURL(t, name, ""), "0043_series_registry.sql")
	var n int
	var v float64
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM obs.series_point`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("rows = %d, %v; want 2", n, err)
	}
	if err := admin.QueryRow(ctx, `SELECT value FROM obs.series_point WHERE observation_date = '2026-11-04'`).Scan(&v); err != nil || v != 4.3 {
		t.Errorf("kept value = %v, %v; want the newest vintage 4.3", v, err)
	}
}

func TestViewHidesVoidShowsNoValuePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, _ := upserterAs(t)
	admin := connectAs(t, name, "")
	for _, q := range []string{
		`INSERT INTO obs.series_point VALUES ('DGS10','2026-11-03','2026-11-05','9999-12-31',4.0,'void','percent','',now(),'fred','r1')`,
		`INSERT INTO obs.series_point VALUES ('DGS10','2026-11-04','2026-11-05','9999-12-31',NULL,'no_value','percent','',now(),'fred','r1')`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	rows, err := connectAs(t, name, "ot_ro_pages").Query(ctx, `SELECT status FROM rd.series_point_r1`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(got) != 1 {
		t.Fatalf("view rows = %v, %v; want one", got, err)
	}
	if status = got[0]; status != "no_value" {
		t.Errorf("status = %q, want no_value", status)
	}
}

func TestNoValueDoesNotReplacePresentPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, w := upserterAs(t)
	u := upserter(t, w)
	v := 4.25
	if _, err := u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v, "present")}, "r"); err != nil {
		t.Fatal(err)
	}
	got, err := u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, nil, "no_value")}, "r")
	if err != nil || got.Written != 0 || got.Unchanged != 1 {
		t.Fatalf("no_value over present: %+v, %v", got, err)
	}
	if sv, status := storedValue(t, name); sv == nil || *sv != 4.25 || status != "present" {
		t.Errorf("stored %v %q, want 4.25 present", sv, status)
	}
}

func TestFillReplacesNoValueNotPresentPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, w := upserterAs(t)
	u, f := upserter(t, w), filler(t, w)
	if _, err := u.UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, nil, "no_value")}, "r"); err != nil {
		t.Fatal(err)
	}
	v := 5.5
	got, err := f.FillSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v, "present")}, "nyfed")
	if err != nil || got.Written != 1 {
		t.Fatalf("fill over no_value: %+v, %v", got, err)
	}
	other := 6.6
	got, err = f.FillSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &other, "present")}, "nyfed")
	if err != nil || got.Written != 0 || got.Unchanged != 1 {
		t.Fatalf("fill over present: %+v, %v", got, err)
	}
	if sv, _ := storedValue(t, name); sv == nil || *sv != 5.5 {
		t.Errorf("value = %v, want 5.5", sv)
	}
}

func TestWriterRoleDeniedDeleteAndKeyUpdatePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, w := upserterAs(t)
	v := 4.25
	if _, err := upserter(t, w).UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v, "present")}, "r"); err != nil {
		t.Fatal(err)
	}
	fred := connectAs(t, name, "ot_w_series_fred")
	mustFail(t, fred, `DELETE FROM obs.series_point`)                // denied: no DELETE
	mustFail(t, fred, `UPDATE obs.series_point SET series_id = 'Z'`) // denied: no UPDATE of the key
	mustFail(t, fred, `UPDATE obs.series_point SET observation_date = '2026-01-01'`)
	mustFail(t, fred, `TRUNCATE obs.series_point`)
}

func TestSeriesViewGrantsAfter0043(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, w := upserterAs(t)
	v := 4.25
	if _, err := upserter(t, w).UpsertSeriesPoints(ctx, []contracts.SeriesPoint{point(t, &v, "present")}, "r"); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"ot_ro_pages", "ot_runner"} {
		var n int
		if err := connectAs(t, name, role).QueryRow(ctx, `SELECT count(*) FROM rd.series_point_r1`).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s: view rows = %d, %v; want 1", role, n, err)
		}
	}
}
