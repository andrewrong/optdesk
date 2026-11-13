//go:build pg

package series

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// libpqVars are the connection variables the pg tests read; none has a default.
var libpqVars = []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"}

// rolePassword is the password of the roles in the throwaway database of the tests.
const rolePassword = "pg-test-only"

// connURL builds the connection URL of database dbname for user from the libpq environment; user "" is the admin.
func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	var missing []string
	for _, v := range libpqVars {
		if os.Getenv(v) == "" {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing environment variables: %s", strings.Join(missing, ", "))
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(os.Getenv("PGUSER"), os.Getenv("PGPASSWORD")),
		Host:   os.Getenv("PGHOST") + ":" + os.Getenv("PGPORT"),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	if m := os.Getenv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// ensureRoles creates the four roles of week 1 when missing, under an advisory lock because roles are shared
// by every database of the server and the tests run in parallel.
func ensureRoles(ctx context.Context, t *testing.T, admin *pgx.Conn) {
	t.Helper()
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`) }()
	for _, r := range []string{"ot_owner", "ot_runner", "ot_w_series_fred", "ot_ro_pages"} {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		if !have {
			if _, err := admin.Exec(ctx, "CREATE ROLE "+r+" LOGIN PASSWORD '"+rolePassword+"'"); err != nil {
				t.Fatalf("create role %s: %v", r, err)
			}
		}
	}
}

// newDatabase creates a database named from the test name and a random suffix, the roles, and applies the
// migrations of this issue there (0002, 0003, 0014 and 0043; none refers to master.session_day). It drops the
// database in t.Cleanup and returns its name.
func newDatabase(t *testing.T) string {
	t.Helper()
	return newDatabaseWith(t, "0002_series.sql", "0003_ops_ledger.sql", "0014_series_unit_index_points.sql", "0043_series_registry.sql")
}

// newDatabaseWith is newDatabase with the named migration files applied in the order given.
func newDatabaseWith(t *testing.T, files ...string) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, os.Getenv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	ensureRoles(ctx, t, admin)
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatal(err)
	}
	name := nonIdent.ReplaceAllString(strings.ToLower(t.Name()), "_")
	if len(name) > 40 {
		name = name[:40]
	}
	name += "_" + hex.EncodeToString(suffix)
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		defer func() { _ = admin.Close(ctx) }()
		if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+name+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})
	for _, f := range files {
		applyMigration(t, connURL(t, name, ""), f)
	}
	for _, r := range []string{"ot_runner", "ot_w_series_fred", "ot_ro_pages"} {
		if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO `+r); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

// applyMigration runs the Up section of migrations/<file> on the database at dsn.
func applyMigration(t *testing.T, dsn, file string) {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/" + file)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	up = strings.Replace(up, "-- +goose Up", "", 1)
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, up); err != nil {
		t.Fatalf("apply %s: %v", file, err)
	}
}

// connectAs opens a plain connection to database name as user.
func connectAs(t *testing.T, name, user string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatalf("connect as %s: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// mustFail asserts that the statement is refused for the connection's role.
func mustFail(t *testing.T, c *pgx.Conn, sql string) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql); err == nil {
		t.Errorf("statement was allowed: %s", sql)
	}
}

func count(t *testing.T, name string) int {
	t.Helper()
	var n int
	if err := connectAs(t, name, "").QueryRow(context.Background(), `SELECT count(*) FROM obs.series_point`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestReplayIdempotentPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_series_fred"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	v := 4.25
	capture := []contracts.SeriesPoint{point(t, &v, "present")}
	for range 2 {
		if err := w.InsertSeriesPoints(ctx, capture); err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, name); n != 1 {
		t.Errorf("rows after replay = %d, want 1", n)
	}
}

func TestSentinelStoredAsNullPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_series_fred"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	for i, f := range []float64{-1, math.NaN(), math.Inf(1)} {
		p := point(t, &f, "present")
		p.ObservationDate = date(t, time.Date(2026, 11, 5+i, 0, 0, 0, 0, time.UTC).Format(time.DateOnly))
		if err := w.InsertSeriesPoints(ctx, []contracts.SeriesPoint{p}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := connectAs(t, name, "").Query(ctx, `SELECT value IS NULL, status FROM obs.series_point`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var isNull bool
		var status string
		if err := rows.Scan(&isNull, &status); err != nil {
			t.Fatal(err)
		}
		if !isNull || status != StatusSentinelRejected {
			t.Errorf("row = %v, %q", isNull, status)
		}
		n++
	}
	if n != 3 {
		t.Errorf("rows = %d, want 3", n)
	}
}

func TestChecksAndRolesPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)

	fred := connectAs(t, name, "ot_w_series_fred")
	// The CHECKs mirror the Writer even for a raw insert.
	mustFail(t, fred, `INSERT INTO obs.series_definition VALUES ('X', 'ccy_amount', '')`)
	if _, err := fred.Exec(ctx, `INSERT INTO obs.series_definition VALUES ('Y', 'percent', '')`); err != nil {
		t.Fatal(err)
	}
	mustFail(t, fred, `INSERT INTO obs.series_point VALUES ('Y', '2026-11-04', '2026-11-05', '9999-12-31', -1, 'present', 'percent', '', now(), 'fred', 'r1')`)
	mustFail(t, fred, `INSERT INTO obs.series_point VALUES ('Y', '2026-11-04', '2026-11-05', '9999-12-31', 'NaN', 'present', 'percent', '', now(), 'fred', 'r1')`)
	mustFail(t, fred, `INSERT INTO obs.series_point VALUES ('Y', '2026-11-04', '2026-11-05', '9999-12-31', NULL, 'present', 'percent', '', now(), 'fred', 'r1')`)
	const good = `INSERT INTO obs.series_point VALUES ` +
		`('Y', '2026-11-04', '2026-11-05', '9999-12-31', 4.1, 'present', 'percent', '', now(), 'fred', 'r1')`
	if _, err := fred.Exec(ctx, good); err != nil {
		t.Fatalf("insert as ot_w_series_fred: %v", err)
	}
	// ot_w_series_fred: INSERT and column UPDATE on series_point (never the key), and REFERENCES and SELECT on ops.capture_run only.
	mustFail(t, fred, `UPDATE obs.series_point SET series_id = 'x'`)
	mustFail(t, fred, `DELETE FROM obs.series_point`)
	mustFail(t, fred, `INSERT INTO ops.capture_run (run_id, session_day, dataset_id, subject_key, status) VALUES ('r', '2026-11-05', 'd', 's', 'done')`)
	mustFail(t, fred, `UPDATE ops.capture_run SET status = 'done'`)
	mustFail(t, fred, `DELETE FROM ops.capture_run`)
	if _, err := fred.Exec(ctx, `SELECT 1 FROM ops.capture_run`); err != nil {
		t.Errorf("select on ops.capture_run: %v", err)
	}

	// ot_ro_pages reads the view and nothing else.
	pages := connectAs(t, name, "ot_ro_pages")
	var n int
	if err := pages.QueryRow(ctx, `SELECT count(*) FROM rd.series_point_r1`).Scan(&n); err != nil || n != 1 {
		t.Errorf("view count = %d, %v", n, err)
	}
	mustFail(t, pages, `SELECT 1 FROM obs.series_point`)
	mustFail(t, pages, `INSERT INTO rd.series_point_r1 SELECT * FROM rd.series_point_r1`)
	mustFail(t, pages, `SELECT 1 FROM ops.capture_run`)

	// ot_runner reads the series tables through the Reader.
	r, err := Open(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.SeriesPoints(ctx, "Y", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 || got[0].Value == nil || *got[0].Value != 4.1 {
		t.Errorf("got %+v, %v", got, err)
	}
	mustFail(t, connectAs(t, name, "ot_runner"), `INSERT INTO obs.series_point SELECT * FROM obs.series_point`)
}

func TestPagesRoleCanUseReaderPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	fred := connectAs(t, name, "ot_w_series_fred")
	if _, err := fred.Exec(ctx, `INSERT INTO obs.series_definition VALUES ('Y','percent','')`); err != nil {
		t.Fatal(err)
	}
	const good = `INSERT INTO obs.series_point VALUES ` +
		`('Y','2026-11-04','2026-11-05','9999-12-31',4.1,'present','percent','',now(),'fred','r1')`
	if _, err := fred.Exec(ctx, good); err != nil {
		t.Fatal(err)
	}
	r, err := Open(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.SeriesPoints(ctx, "Y", "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 {
		t.Fatalf("pages reader: %v, %v", got, err)
	}
}
