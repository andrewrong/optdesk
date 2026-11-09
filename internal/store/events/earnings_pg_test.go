//go:build pg

package events

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

// newDatabase makes a throwaway database with the schemas, the roles and migration 0020, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, os.Getenv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(682)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_w_events", "ot_ro_pages"} {
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
	raw, err := os.ReadFile("../../../migrations/0020_events.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, `CREATE SCHEMA obs; CREATE SCHEMA rd`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0020: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_events, ot_ro_pages`); err != nil {
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
	if err := pgxAs(t, name).QueryRow(context.Background(), `SELECT count(*) FROM obs.earnings_date`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestWriterKeepsExistingKeyPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := OpenEarningsDate(ctx, connURL(t, name, "ot_w_events"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.InsertEarningsDate(ctx, row("2026-11-09", 1, "error")); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertEarningsDate(ctx, row("2026-11-09", 1, "absent")); err != nil {
		t.Fatalf("second insert of one key: %v", err)
	}
	if n := count(t, name); n != 1 {
		t.Errorf("%d rows after two inserts of one key, want 1", n)
	}
	got, err := w.EarningsDates(ctx, issuer, "2026-11-09", "2026-11-09")
	if err != nil || len(got) != 1 || got[0].Status != "error" {
		t.Errorf("rows = %+v, %v; want the first row", got, err)
	}
}

func TestReaderTakesHighestAttemptOnBothRolesPG(t *testing.T) {
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
		if err != nil || len(got) != 2 || got[0].Attempt != 2 || got[0].ReportDate == nil || got[0].Estimate == nil {
			t.Fatalf("%s: %+v, %v; want day 9 attempt 2 listed, then day 10", role, got, err)
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
			`SELECT 1 FROM obs.earnings_date`, `UPDATE obs.earnings_date SET symbol = 'X'`,
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
	if err := p.InsertEarningsDate(ctx, row("2026-11-09", 1, "absent")); err == nil {
		t.Error("the page role wrote a row")
	}
}
