//go:build pg

package sessionday

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

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
// migrations 0001 to 0004 there in order. It drops the database in t.Cleanup and returns its name.
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
	for _, f := range []string{"0001_master_session_day.sql", "0002_series.sql", "0003_ops_ledger.sql", "0004_session_day_grant.sql"} {
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
		t.Fatalf("apply migration %s: %v", file, err)
	}
}

// TestRunnerReadsOnlyPG: ot_runner reads a typed row through the real Reader and cannot write.
func TestRunnerReadsOnlyPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)

	owner, err := Open(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatalf("Open admin: %v", err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	row := contracts.SessionDay{CalendarID: "XNYS", SessionDay: day(t, "2026-11-02"), IsSession: true,
		OpenAt: utc(2026, 11, 2, 14, 30), CloseAt: utc(2026, 11, 2, 21, 0)}
	if err := owner.UpsertSessionDays(ctx, []contracts.SessionDay{row}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	runner, err := Open(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatalf("Open runner: %v", err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	got, err := runner.SessionDays(ctx, "XNYS", "2026-11-02", "2026-11-02")
	if err != nil {
		t.Fatalf("read as ot_runner: %v", err)
	}
	assertRows(t, got, []contracts.SessionDay{row})

	if err := runner.UpsertSessionDays(ctx, []contracts.SessionDay{row}); err == nil {
		t.Fatal("write as ot_runner succeeded, want permission denied")
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := Open(ctx, connURL(t, newDatabase(t), ""))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	rows := []contracts.SessionDay{
		{CalendarID: "XNYS", SessionDay: day(t, "2026-10-30"), IsSession: true, OpenAt: utc(2026, 10, 30, 13, 30), CloseAt: utc(2026, 10, 30, 20, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-02"), IsSession: true, OpenAt: utc(2026, 11, 2, 14, 30), CloseAt: utc(2026, 11, 2, 21, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-26"), IsSession: false, OpenAt: utc(2026, 11, 26, 14, 30), CloseAt: utc(2026, 11, 26, 21, 0)},
		{CalendarID: "XNYS", SessionDay: day(t, "2026-11-27"), IsSession: true, EarlyClose: true,
			OpenAt: utc(2026, 11, 27, 14, 30), CloseAt: utc(2026, 11, 27, 18, 0)},
		{CalendarID: "XTKS", SessionDay: day(t, "2026-11-02"), IsSession: true, OpenAt: utc(2026, 11, 2, 0, 0), CloseAt: utc(2026, 11, 2, 6, 0)},
	}
	if err := s.UpsertSessionDays(ctx, rows); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := s.SessionDays(ctx, "XNYS", "2026-10-30", "2026-11-27")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	assertRows(t, got, rows[:4])

	// Both ends are inclusive and other calendars are excluded.
	got, err = s.SessionDays(ctx, "XNYS", "2026-11-02", "2026-11-26")
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, got, rows[1:3])
}

func TestUpsertReplacesRow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := Open(ctx, connURL(t, newDatabase(t), ""))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	first := contracts.SessionDay{CalendarID: "XNYS", SessionDay: day(t, "2026-11-27"), IsSession: true,
		OpenAt: utc(2026, 11, 27, 14, 30), CloseAt: utc(2026, 11, 27, 21, 0)}
	second := first
	second.EarlyClose, second.CloseAt = true, utc(2026, 11, 27, 18, 0)
	if err := s.UpsertSessionDays(ctx, []contracts.SessionDay{first}); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertSessionDays(ctx, []contracts.SessionDay{second}); err != nil {
		t.Fatal(err)
	}
	got, err := s.SessionDays(ctx, "XNYS", "2026-11-27", "2026-11-27")
	if err != nil {
		t.Fatal(err)
	}
	assertRows(t, got, []contracts.SessionDay{second})
}

func assertRows(t *testing.T, got, want []contracts.SessionDay) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.CalendarID != w.CalendarID || !g.SessionDay.Equal(w.SessionDay.Time) || g.IsSession != w.IsSession ||
			g.EarlyClose != w.EarlyClose || !g.OpenAt.Equal(w.OpenAt) || !g.CloseAt.Equal(w.CloseAt) {
			t.Errorf("row %d = %+v, want %+v", i, g, w)
		}
	}
}
