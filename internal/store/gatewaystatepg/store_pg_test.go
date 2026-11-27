//go:build pg

package gatewaystatepg

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
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	rolePassword  = "pg-test-only"
	migrationFile = "../../../migrations/0074_gateway_state.sql"
)

// connURL builds the URL of database dbname on the server the libpq variables name; user "" is the administrator.
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

// newDatabase makes a throwaway database with the roles and the migration, dropped in t.Cleanup.
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
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(864)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_runner"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(864)`)
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
	raw, err := os.ReadFile(migrationFile)
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_runner, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

// openStores returns a writer on the role ot_runner and a reader on the role ot_ro_pages of one new database.
func openStores(t *testing.T) (w, r *Store) {
	t.Helper()
	name := newDatabase(t)
	w, err := Open(context.Background(), connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	r, err = Open(context.Background(), connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return w, r
}

func TestObserveSequence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, r := openStores(t)
	t0 := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	steps := []struct {
		name      string
		down      bool
		at        time.Time
		wantDown  bool
		wantSince time.Time
		wantObs   time.Time
	}{
		{"insert", true, t0, true, t0, t0},
		{"same flag keeps since", true, t0.Add(time.Minute), true, t0, t0.Add(time.Minute)},
		{"flipped flag moves since", false, t0.Add(2 * time.Minute), false, t0.Add(2 * time.Minute), t0.Add(2 * time.Minute)},
		{"earlier at changes nothing", true, t0.Add(time.Minute), false, t0.Add(2 * time.Minute), t0.Add(2 * time.Minute)},
		{"earlier at, same flag, changes nothing", false, t0, false, t0.Add(2 * time.Minute), t0.Add(2 * time.Minute)},
	}
	for _, s := range steps {
		if err := w.Observe(ctx, "main", s.down, s.at); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		got, err := r.States(ctx)
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: rows = %+v, %v", s.name, got, err)
		}
		g := got[0]
		if g.Alias != "main" || g.Down != s.wantDown || !g.Since.Equal(s.wantSince) || !g.ObservedAt.Equal(s.wantObs) {
			t.Errorf("%s: got %+v, want down=%v since=%v observed=%v", s.name, g, s.wantDown, s.wantSince, s.wantObs)
		}
	}
}

func TestStatesOrderedByAlias(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	w, r := openStores(t)
	at := time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)
	for _, a := range []string{"wife", "main", "alt"} {
		if err := w.Observe(ctx, a, false, at); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.States(ctx)
	if err != nil || len(got) != 3 || got[0].Alias != "alt" || got[1].Alias != "main" || got[2].Alias != "wife" {
		t.Fatalf("rows = %+v, %v", got, err)
	}
}
