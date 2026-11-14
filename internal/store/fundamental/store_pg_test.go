//go:build pg

package fundamental

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

// stubInstrument stands for migration 0008 in the list of newDatabase.
const stubInstrument = `CREATE TABLE master.instrument (instrument_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY)`

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

// newDatabase makes a throwaway database with the schemas, the roles and, in one list, 0017, the files
// 0046 needs and 0046, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, os.Getenv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(654)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_w_fundamental", "ot_w_events", "ot_ro_pages"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(654)`)
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
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, `CREATE SCHEMA obs; CREATE SCHEMA rd; CREATE SCHEMA master`); err != nil {
		t.Fatal(err)
	}
	// 0046 reaches the event tables, the views and master.instrument. 0008 makes master.instrument but also
	// needs the timescale hypertables of 0006 and 0007, which no fact test has, so the stub below is the one
	// column 0046 refers to.
	for _, f := range []string{
		"0017_fundamental_fact.sql", "0020_events.sql", "0023_dividends.sql", stubInstrument, "0046_fundamentals_keys.sql",
	} {
		sql := f
		if f != stubInstrument {
			raw, err := os.ReadFile("../../../migrations/" + f)
			if err != nil {
				t.Fatal(err)
			}
			up, _, _ := strings.Cut(string(raw), "-- +goose Down")
			sql = strings.Replace(up, "-- +goose Up", "", 1)
		}
		if _, err := c.Exec(ctx, sql); err != nil {
			t.Fatalf("apply %.40s: %v", f, err)
		}
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_fundamental, ot_ro_pages`); err != nil {
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
	if err := pgxAs(t, name).QueryRow(context.Background(), `SELECT count(*) FROM obs.fundamental_fact`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestInsertTwiceAddsZeroRowsPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_fundamental"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	rows := []contracts.FundamentalFact{
		fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01"),
		fact("a1", "Assets", "", "2024-12-31", "2025-02-01"),
	}
	if err := w.InsertFacts(ctx, rows); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertFacts(ctx, rows); err != nil {
		t.Fatalf("second insert: %v", err)
	}
	if n := count(t, name); n != 2 {
		t.Errorf("%d rows after two inserts, want 2 (a duration and an instant fact)", n)
	}
}

func TestFactsFiledByOnBothRolesPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, err := Open(ctx, connURL(t, name, "ot_w_fundamental"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	if err := w.InsertFacts(ctx, []contracts.FundamentalFact{
		fact("a1", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2025-02-01"),
		fact("a1", "Assets", "", "2024-12-31", "2025-02-01"),
		fact("a2", "NetIncomeLoss", "2024-01-01", "2024-12-31", "2026-02-01"),
	}); err != nil {
		t.Fatal(err)
	}
	p, err := Open(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	for role, r := range map[string]store.FactReader{"ot_w_fundamental": w, "ot_ro_pages": p} {
		got, err := r.FactsFiledBy(ctx, "sec", "cik:0000320193", "2025-06-30")
		if err != nil || len(got) != 2 {
			t.Fatalf("%s: %d rows, %v; want 2", role, len(got), err)
		}
		for _, g := range got {
			if g.Concept == "Assets" && g.PeriodStart != nil {
				t.Errorf("%s: instant fact has period_start %v", role, g.PeriodStart)
			}
		}
		if all, _ := r.FactsFiledBy(ctx, "sec", "cik:0000320193", "2026-02-01"); len(all) != 3 {
			t.Errorf("%s: %d rows by the late day, want 3", role, len(all))
		}
	}
}

func TestRolesCannotReadTableOrPageWritePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	for _, role := range []string{"ot_w_fundamental", "ot_ro_pages"} {
		c, err := pgx.Connect(ctx, connURL(t, name, role))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := c.Exec(ctx, `SELECT 1 FROM obs.fundamental_fact`); err == nil {
			t.Errorf("%s read the base table", role)
		}
		_ = c.Close(ctx)
	}
	p, err := Open(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if err := p.InsertFacts(ctx, []contracts.FundamentalFact{fact("a1", "Assets", "", "2024-12-31", "2025-02-01")}); err == nil {
		t.Error("the page role wrote a fact")
	}
}
