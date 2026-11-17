//go:build pg

package ops

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

const covDayPG = "2026-11-12"

// newCoverageDatabase is newDatabase plus the Up section of 0051.
func newCoverageDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	applyMigration(t, connURL(t, name, ""), "0051_coverage_view.sql")
	return name
}

func covExec(t *testing.T, c *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func covExpect(t *testing.T, c *pgx.Conn, ds string, keys ...string) {
	t.Helper()
	for _, k := range keys {
		covExec(t, c, `INSERT INTO ops.capture_expected_key VALUES ($1, 'plan', $2, $3)`, covDayPG, ds, k)
	}
}

func covPresent(t *testing.T, c *pgx.Conn, ds, key, status string) {
	t.Helper()
	covExec(t, c, `INSERT INTO ops.capture_present VALUES ($1, $2, $3, $4)`, covDayPG, ds, key, status)
}

// covRun inserts a finished run row of the key; at orders the rows of one key (the later the newer).
func covRun(t *testing.T, c *pgx.Conn, id, ds, key, status, errCode, reason string, minute int) {
	t.Helper()
	covExec(t, c, `INSERT INTO ops.capture_run (run_id, session_day, dataset_id, subject_key, status, finished_at, error_code, reason_code)
		VALUES ($1, $2, $3, $4, $5, timestamptz '2026-11-12 12:00:00+00' + make_interval(mins => $6), $7, $8)`,
		id, covDayPG, ds, key, status, minute, errCode, reason)
}

type covRow struct {
	keys, ok, noValue, expired, missing int
	missingKeys                         []string
}

func covRead(t *testing.T, name, ds string) covRow {
	t.Helper()
	r, err := OpenCoverage(context.Background(), connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	cells, err := r.Cells(context.Background(), covDayPG, covDayPG)
	if err != nil {
		t.Fatal(err)
	}
	var rows []covRow
	for _, c := range cells {
		if c.Dataset == ds {
			rows = append(rows, covRow{c.Keys, c.OK, c.NoValue, c.Expired, c.Missing, c.MissingKeys})
		}
	}
	if len(rows) != 1 {
		t.Fatalf("rows of %s = %+v, want one", ds, rows)
	}
	return rows[0]
}

func TestCoverageViewStates(t *testing.T) {
	t.Parallel()
	name := newCoverageDatabase(t)
	c := connectAs(t, name, "")
	covExpect(t, c, "obs.series_point", "A", "B", "C")
	covPresent(t, c, "obs.series_point", "A", "present")
	covPresent(t, c, "obs.series_point", "B", "no_value")
	covRun(t, c, "r-b", "obs.series_point", "B", "done", "", "", 0)
	got := covRead(t, name, "obs.series_point")
	want := covRow{keys: 3, ok: 1, noValue: 1, missing: 1, missingKeys: []string{"C"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestCoverageViewExpired(t *testing.T) {
	t.Parallel()
	name := newCoverageDatabase(t)
	c := connectAs(t, name, "")
	ds := "obs.bar"
	covExpect(t, c, ds, "A", "B", "C")
	for _, k := range []string{"A", "B", "C"} {
		covPresent(t, c, ds, k, "no_value")
	}
	covRun(t, c, "r-a", ds, "A", "done", "", "expired", 0)
	covRun(t, c, "r-b", ds, "B", "failed", "retryable", "", 0)
	covRun(t, c, "r-c", ds, "C", "done", "", "", 0)
	got := covRead(t, name, ds)
	want := covRow{keys: 3, noValue: 1, expired: 2, missingKeys: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	covRun(t, c, "r-a2", ds, "A", "done", "", "", 5)
	got = covRead(t, name, ds)
	want = covRow{keys: 3, noValue: 2, expired: 1, missingKeys: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("after a clean run: got %+v, want %+v", got, want)
	}
}

func TestCoverageViewAsPagesRole(t *testing.T) {
	t.Parallel()
	name := newCoverageDatabase(t)
	pages := connectAs(t, name, "ot_ro_pages")
	rows, err := pages.Query(context.Background(), `SELECT session_day, dataset_id, keys, ok, no_value, expired, missing, missing_keys FROM rd.coverage_day_r1`)
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	mustFail(t, pages, `SELECT 1 FROM ops.capture_present`)
}

func TestCoverageDown(t *testing.T) {
	t.Parallel()
	name := newCoverageDatabase(t)
	raw, err := os.ReadFile("../../../migrations/0051_coverage_view.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("0051 has no Down section")
	}
	c := connectAs(t, name, "")
	covExec(t, c, down)
	var n int
	if err := c.QueryRow(context.Background(), `SELECT count(*) FROM pg_views WHERE schemaname = 'rd' AND viewname = 'coverage_day_r1'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal(fmt.Sprintf("view still present after Down: %d", n))
	}
}
