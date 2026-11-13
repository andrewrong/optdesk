//go:build pg

package hoursnoticepg

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

	"github.com/andrewrong/optdesk/internal/store/hoursnotice"
)

const rolePassword = "pg-test-only"

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

// newDatabase makes a throwaway database with the roles, the schemas master and rd and the migration, dropped in
// t.Cleanup.
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
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_master"} {
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
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS master; CREATE SCHEMA IF NOT EXISTS rd`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_master, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

func openStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(context.Background(), connURL(t, newDatabase(t), "ot_w_master"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func nt(market, day, ours, ibkr string) hoursnotice.Notice {
	return hoursnotice.Notice{Market: market, Day: day, Ours: ours, IBKR: ibkr}
}

func TestReplaceInsertsAndReadsInOrder(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openStore(t)
	rows := []hoursnotice.Notice{nt("XNYS", "2026-11-27", "0930-1300", "CLOSED"), nt("XNYS", "2026-11-26", "0930-1600", "CLOSED")}
	if err := st.Replace(ctx, "XNYS", rows); err != nil {
		t.Fatal(err)
	}
	got, err := st.Notices(ctx)
	if err != nil || len(got) != 2 || got[0] != rows[1] || got[1] != rows[0] {
		t.Fatalf("rows = %+v, %v", got, err)
	}
}

func TestReplaceTwiceWithSameRowsChangesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openStore(t)
	rows := []hoursnotice.Notice{nt("XNYS", "2026-11-27", "0930-1300", "CLOSED")}
	for i := 0; i < 2; i++ {
		if err := st.Replace(ctx, "XNYS", rows); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := st.Notices(ctx); len(got) != 1 || got[0] != rows[0] {
		t.Fatalf("rows = %+v", got)
	}
}

func TestReplaceWithFewerRowsDropsTheMissingDays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openStore(t)
	rows := []hoursnotice.Notice{nt("XNYS", "2026-11-26", "a", "b"), nt("XNYS", "2026-11-27", "c", "d")}
	if err := st.Replace(ctx, "XNYS", rows); err != nil {
		t.Fatal(err)
	}
	if err := st.Replace(ctx, "XNYS", rows[1:]); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Notices(ctx); len(got) != 1 || got[0] != rows[1] {
		t.Fatalf("rows = %+v", got)
	}
}

func TestReplaceOfOneMarketLeavesTheOtherAlone(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openStore(t)
	jp := nt("XTKS", "2026-11-24", "0900-1530", "0900-1500")
	if err := st.Replace(ctx, "XTKS", []hoursnotice.Notice{jp}); err != nil {
		t.Fatal(err)
	}
	if err := st.Replace(ctx, "XNYS", []hoursnotice.Notice{nt("XNYS", "2026-11-27", "a", "b")}); err != nil {
		t.Fatal(err)
	}
	if err := st.Replace(ctx, "XNYS", nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Notices(ctx); len(got) != 1 || got[0] != jp {
		t.Fatalf("rows = %+v", got)
	}
}
