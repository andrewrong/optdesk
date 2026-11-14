//go:build pg

package macroeventpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

const rolePassword = "pg-test-only"

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	cfg, err := pgx.ParseConfig("")
	if err != nil {
		t.Fatalf("read libpq environment: %v", err)
	}
	if cfg.Host == "" || cfg.User == "" {
		t.Fatal("PGHOST and PGUSER name no server")
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(int(cfg.Port))),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	mode := "disable"
	if cfg.TLSConfig != nil {
		mode = "prefer"
	}
	u.RawQuery = url.Values{"sslmode": {mode}}.Encode()
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles, the schema obs and migration 0048.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	cfg, err := pgx.ParseConfig("")
	if err != nil || cfg.Database == "" {
		t.Fatalf("PGDATABASE names no database: %v", err)
	}
	admin, err := pgx.Connect(ctx, connURL(t, cfg.Database, ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(962)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_runner", "ot_w_series_fred"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(962)`)
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
	raw, err := os.ReadFile("../../../migrations/0048_macro_event.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS obs`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0048: %v", err)
	}
	if _, err := c.Exec(ctx, `GRANT USAGE ON SCHEMA obs TO ot_w_series_fred, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_series_fred, ot_ro_pages, ot_runner`); err != nil {
		t.Fatal(err)
	}
	return name
}

func pev(kind, date string) macroevent.Event {
	return macroevent.Event{Kind: kind, Source: "fred", SourceRef: "10", Date: contracts.Day(date), RunDay: "2026-11-14"}
}

func TestMacroEventRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	w, wc, err := OpenWriter(ctx, connURL(t, name, "ot_w_series_fred"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wc.Close() }()
	r, rc, err := OpenReader(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()

	first := []macroevent.Event{pev("cpi", "2026-11-20"), pev("cpi", "2026-12-10")}
	if o, err := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-14", first); err != nil || o.Written != 2 || o.Held {
		t.Fatalf("first: %+v %v", o, err)
	}
	if o, err := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-14", first); err != nil || o.Written != 0 || o.Deleted != 0 {
		t.Fatalf("rerun writes 0: %+v %v", o, err)
	}
	// A middle date that moved is deleted; the last date is kept.
	if o, err := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-15", []macroevent.Event{pev("cpi", "2026-11-25"), pev("cpi", "2026-12-10")}); err != nil || o.Deleted != 1 || o.Held {
		t.Fatalf("moved date: %+v %v", o, err)
	}
	// A shorter answer is held and deletes nothing.
	if o, err := w.ReplaceFuture(ctx, "cpi", "fred", "2026-11-16", []macroevent.Event{pev("cpi", "2026-11-25")}); err != nil || !o.Held || o.Deleted != 0 {
		t.Fatalf("shorter: %+v %v", o, err)
	}
	got, err := r.Between(ctx, "2026-11-01", "2026-12-31")
	if err != nil || len(got) != 2 || got[0].Date != "2026-11-25" || got[1].Date != "2026-12-10" {
		t.Fatalf("between: %v %v", got, err)
	}
	ks, err := r.Kinds(ctx)
	if err != nil || len(ks) != 1 || ks[0].Rows != 2 || ks[0].LastDate != "2026-12-10" || ks[0].HeldDay != "2026-11-16" || ks[0].LastRunDay != "2026-11-16" {
		t.Fatalf("kinds: %+v %v", ks, err)
	}
}

func TestMacroEventGrants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, tc := range []struct {
		role string
		want map[string]bool
	}{
		{"ot_w_series_fred", map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "DELETE": true}},
		{"ot_ro_pages", map[string]bool{"SELECT": true, "INSERT": false, "UPDATE": false, "DELETE": false}},
		{"ot_runner", map[string]bool{"SELECT": false, "INSERT": false, "UPDATE": false, "DELETE": false}},
	} {
		for priv, want := range tc.want {
			var have bool
			if err := c.QueryRow(ctx, `SELECT has_table_privilege($1, 'obs.macro_event', $2)`, tc.role, priv).Scan(&have); err != nil {
				t.Fatal(err)
			}
			if have != want {
				t.Errorf("%s %s: have %v want %v", tc.role, priv, have, want)
			}
		}
	}
}
