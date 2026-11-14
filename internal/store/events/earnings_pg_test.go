//go:build pg

package events

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const rolePassword = "pg-test-only"

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	for _, v := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		if os.Getenv(v) == "" {
			t.Fatalf("missing environment variable %s", v)
		}
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

// newDatabase makes a throwaway database with the roles and the migrations of keysMigrations, dropped in t.Cleanup.
func newDatabase(t *testing.T) string { return newDatabaseWith(t, keysMigrations) }

// newDatabaseWith makes a throwaway database with the roles and the Timescale extension and applies the files in
// order. 0001 to 0008 need ot_runner and ot_loader; the roles are shared by the server, created when missing and never
// altered.
func newDatabaseWith(t *testing.T, files []string) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, os.Getenv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(682)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range keysRoles {
		const create = `DO $$ BEGIN CREATE ROLE %s LOGIN PASSWORD '%s'; EXCEPTION WHEN duplicate_object OR unique_violation THEN NULL; END $$`
		if _, err := admin.Exec(ctx, fmt.Sprintf(create, r, rolePassword)); err != nil {
			t.Fatalf("create role %s: %v", r, err)
		}
	}
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(682)`)
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
	ext, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	_, err = ext.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb`)
	_ = ext.Close(ctx)
	if err != nil {
		t.Fatalf("create extension: %v", err)
	}
	for _, f := range files {
		applyMigration(t, name, f)
		if f == "0001_master_session_day.sql" {
			// The foreign-key checks on master.instrument run as its owner ot_owner, which needs the schema.
			execAdmin(t, name, `ALTER SCHEMA master OWNER TO ot_owner`)
		}
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_events, ot_ro_pages, ot_w_fundamental`); err != nil {
		t.Fatal(err)
	}
	return name
}

func pgxAs(t *testing.T, name string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, ""))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func count(t *testing.T, name string) int {
	t.Helper()
	var n int
	if err := pgxAs(t, name).QueryRow(context.Background(), `SELECT count(*) FROM obs.earnings_check`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInsertEarningsDateUpsertsOneRowPerDayPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := OpenEarningsDate(ctx, connURL(t, name, "ot_w_events"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.InsertEarningsDate(ctx, row("2026-11-09", 1, "absent")); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertEarningsDate(ctx, row("2026-11-09", 1, "listed")); err != nil {
		t.Fatalf("second answer of one day: %v", err)
	}
	if n := count(t, name); n != 1 {
		t.Errorf("%d answers after two inserts of one day, want 1", n)
	}
	got, err := w.EarningsDates(ctx, issuer, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].Status != "listed" || got[0].Attempt != 1 {
		t.Errorf("rows = %+v, %v; want the listed answer", got, err)
	}
}

func TestReaderReadsTheViewOnBothRolesPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := OpenEarningsDate(ctx, connURL(t, name, "ot_w_events"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	for _, r := range []contracts.EarningsDate{
		row("2026-11-09", 1, "error"), row("2026-11-09", 2, "listed"), row("2026-11-10", 1, "absent"),
	} {
		if err := w.InsertEarningsDate(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	p, err := OpenEarningsDate(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	for role, r := range map[string]store.EarningsDateReader{"ot_w_events": w, "ot_ro_pages": p} {
		got, err := r.EarningsDates(ctx, issuer, "2026-11-09", "2026-11-10")
		if err != nil || len(got) != 2 || got[0].Attempt != 1 || got[0].ReportDate == nil || got[0].Estimate == nil {
			t.Fatalf("%s: %+v, %v; want day 9 listed (attempt 1), then day 10", role, got, err)
		}
	}
}

func TestRolesCannotReadTableOrPageWriteOrChangeRowsPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	for _, role := range []string{"ot_w_events", "ot_ro_pages"} {
		c, err := pgx.Connect(ctx, connURL(t, name, role))
		if err != nil {
			t.Fatal(err)
		}
		for _, q := range []string{
			`SELECT received_at FROM obs.earnings_date`, `UPDATE obs.earnings_date SET period_key = DATE '2026-01-01'`,
			`DELETE FROM obs.earnings_date`, `TRUNCATE obs.earnings_date`,
		} {
			if _, err := c.Exec(ctx, q); err == nil {
				t.Errorf("%s ran %q", role, q)
			}
		}
		_ = c.Close(ctx)
	}
	p, err := OpenEarningsDate(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if _, err := connectAs(t, name, "ot_ro_pages").Exec(ctx, `UPDATE obs.earnings_date SET symbol = 'X'`); err == nil {
		t.Error("the page role updated a symbol")
	}
	if err := p.InsertEarningsDate(ctx, row("2026-11-09", 1, "absent")); err == nil {
		t.Error("the page role wrote a row")
	}
}
