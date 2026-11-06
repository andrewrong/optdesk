//go:build pg

package ops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
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
// migrations of this issue there (0002 and 0003; neither refers to master.session_day). It drops the database in
// t.Cleanup and returns its name.
func newDatabase(t *testing.T) string {
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
	for _, f := range []string{"0002_series.sql", "0003_ops_ledger.sql"} {
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

func openRunner(t *testing.T) (store.OpsLedger, string) {
	t.Helper()
	name := newDatabase(t)
	l, err := Open(context.Background(), connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l, name
}

func TestRunRoundTripPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l, _ := openRunner(t)
	lease := time.Date(2026, 11, 5, 21, 0, 0, 0, time.UTC)
	r := store.Run{ID: "run-1", Day: "2026-11-05", Key: contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: "DGS10"},
		Status: store.RunRunning, LeaseUntil: lease}
	if err := l.RecordRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err := l.Runs(ctx, "2026-11-05")
	if err != nil || len(got) != 1 || !sameRun(got[0], r) {
		t.Fatalf("got %+v, %v; want %+v", got, err, r)
	}
	fin := lease.Add(time.Minute)
	r.Status, r.LeaseUntil, r.HeartbeatAt, r.FinishedAt = store.RunDone, time.Time{}, lease, fin
	r.ReturnedN, r.UsableN, r.ErrorCode, r.ReasonCode = 3, 2, "retryable", "late"
	if err := l.RecordRun(ctx, r); err != nil {
		t.Fatal(err)
	}
	got, err = l.Runs(ctx, "2026-11-05")
	if err != nil || len(got) != 1 || !sameRun(got[0], r) {
		t.Fatalf("after update got %+v, %v; want %+v", got, err, r)
	}
	bad := store.Run{ID: "run-2", Day: "2026-11-05", Status: store.RunRunning}
	if err := l.RecordRun(ctx, bad); err == nil {
		t.Error("a running run without lease was stored")
	}
}

func TestLedgerTablesPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l, _ := openRunner(t)

	if _, err := l.Window(ctx, "obs.series_point", "2026-11-26"); err == nil {
		t.Error("window without row")
	} else if te, ok := err.(contracts.TypedError); !ok || te.Kind() != contracts.NoValue {
		t.Errorf("err = %v", err)
	}
	w := store.Window{DatasetID: "obs.series_point", Day: "2026-11-05", OpenAt: time.Date(2026, 11, 5, 14, 30, 0, 0, time.UTC)}
	for range 2 {
		if err := l.PutWindow(ctx, w); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := l.Window(ctx, w.DatasetID, w.Day); err != nil || !sameWindow(got, w) {
		t.Errorf("window = %+v, %v", got, err)
	}
}

func TestExpectedPresentCursorPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l, _ := openRunner(t)
	k := contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: "DGS10"}
	if keys, h, err := l.Expected(ctx, "2026-11-05"); err != nil || len(keys) != 0 || h != "" {
		t.Errorf("empty plan = %v %q %v", keys, h, err)
	}
	if err := l.PutExpected(ctx, "2026-11-05", "h1", []contracts.ExpectedKey{k, {DatasetID: "obs.series_point", SubjectKey: "DGS2"}}); err != nil {
		t.Fatal(err)
	}
	if err := l.PutExpected(ctx, "2026-11-05", "h2", []contracts.ExpectedKey{k}); err != nil {
		t.Fatal(err)
	}
	if keys, h, err := l.Expected(ctx, "2026-11-05"); err != nil || h != "h2" || !reflect.DeepEqual(keys, []contracts.ExpectedKey{k}) {
		t.Errorf("plan = %v %q %v", keys, h, err)
	}
	if err := l.MarkPresent(ctx, "2026-11-05", k, store.StatusNoValue); err != nil {
		t.Fatal(err)
	}
	if err := l.MarkPresent(ctx, "2026-11-05", k, store.StatusPresent); err != nil {
		t.Fatal(err)
	}
	if got, err := l.Present(ctx, "2026-11-05"); err != nil || !reflect.DeepEqual(got, map[contracts.ExpectedKey]store.PresentStatus{k: store.StatusPresent}) {
		t.Errorf("present = %v, %v", got, err)
	}

	if c, err := l.Cursor(ctx); err != nil || c.Day != "" {
		t.Errorf("empty cursor = %+v, %v", c, err)
	}
	for _, d := range []contracts.Day{"2026-11-04", "2026-11-05"} {
		if err := l.SetCursor(ctx, store.Cursor{Day: d}); err != nil {
			t.Fatal(err)
		}
	}
	if c, err := l.Cursor(ctx); err != nil || c.Day != "2026-11-05" {
		t.Errorf("cursor = %+v, %v", c, err)
	}
}

func TestLeasePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l, _ := openRunner(t)
	a, err := l.Acquire(ctx, "a", time.Hour)
	if err != nil || a.Owner != "a" || a.ExpiresAt.IsZero() {
		t.Fatalf("acquire a = %+v, %v", a, err)
	}
	if _, err := l.Acquire(ctx, "a", time.Hour); err != nil {
		t.Errorf("same owner renew: %v", err)
	}
	_, err = l.Acquire(ctx, "b", time.Hour)
	if te, ok := err.(contracts.TypedError); !ok || te.Kind() != contracts.Retryable {
		t.Errorf("b while held: %v", err)
	}
	if err := l.Release(ctx, a); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire(ctx, "b", time.Hour); err != nil {
		t.Errorf("b after release: %v", err)
	}
}

func TestRolesPG(t *testing.T) {
	t.Parallel()
	_, name := openRunner(t)
	runner := connectAs(t, name, "ot_runner")
	mustFail(t, runner, `UPDATE ops.capture_run SET session_day = '2026-01-01'`)
	mustFail(t, runner, `UPDATE ops.capture_run SET run_id = 'x'`)
	mustFail(t, runner, `DELETE FROM ops.capture_run`)
	mustFail(t, runner, `INSERT INTO obs.series_definition VALUES ('Z', 'percent', '')`)
	if _, err := runner.Exec(context.Background(), `UPDATE ops.capture_run SET status = 'done'`); err != nil {
		t.Errorf("limited update: %v", err)
	}
	mustFail(t, connectAs(t, name, "ot_ro_pages"), `SELECT 1 FROM ops.run_lease`)
	mustFail(t, connectAs(t, name, "ot_w_series_fred"), `INSERT INTO ops.run_lease (owner, expires_at) VALUES ('x', now())`)
}

// sameRun compares two runs, instants by Equal.
func sameRun(a, b store.Run) bool {
	return a.ID == b.ID && a.Day == b.Day && a.Key == b.Key && a.Status == b.Status &&
		a.LeaseUntil.Equal(b.LeaseUntil) && a.HeartbeatAt.Equal(b.HeartbeatAt) && a.FinishedAt.Equal(b.FinishedAt) &&
		a.ReturnedN == b.ReturnedN && a.UsableN == b.UsableN && a.ErrorCode == b.ErrorCode && a.ReasonCode == b.ReasonCode
}

func sameWindow(a, b store.Window) bool {
	return a.DatasetID == b.DatasetID && a.Day == b.Day && a.OpenAt.Equal(b.OpenAt) && a.CloseAt.Equal(b.CloseAt)
}
