//go:build pg

package strategyresultpg

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

	"github.com/andrewrong/optdesk/internal/store/strategyresult"
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

// newDatabase makes a throwaway database with the roles, the schemas rd and the migration, dropped in
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
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(2238)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_candidate", "ot_runner"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(2238)`)
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
	raw, err := os.ReadFile("../../../migrations/0042_strategy_result.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS rd`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_candidate, ot_ro_pages, ot_runner`); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestStrategyResultRolesAndView(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_candidate"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	rank := 1
	in := strategyresult.Row{SessionDay: "2026-11-13", RunID: "r1", Strategy: "short_put", RuleVersion: "put_v1",
		ParamHash: "h", Key: "42", UnderlyingID: 7, Status: "ok", Rank: &rank, Legs: "[]", Checks: "[]", Facts: "{}",
		ComputedAt: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)}
	if n, err := st.InsertResults(ctx, []strategyresult.Row{in}); err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	if n, err := st.InsertResults(ctx, []strategyresult.Row{in}); err != nil || n != 0 {
		t.Fatalf("repeat insert = %d, %v", n, err)
	}
	got, err := st.Results(ctx, "2026-11-13", "short_put")
	if err != nil || len(got) != 1 || got[0].Key != "42" || got[0].Rank == nil || *got[0].Rank != 1 || got[0].RankInUnderlying != nil {
		t.Fatalf("results = %+v, %v", got, err)
	}

	pages, err := pgx.Connect(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pages.Close(ctx) })
	var n int
	if err := pages.QueryRow(ctx, `SELECT count(*) FROM rd.strategy_result_r1`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("ot_ro_pages view read = %d, %v", n, err)
	}
	if _, err := pages.Exec(ctx, `INSERT INTO strategy.result SELECT * FROM rd.strategy_result_r1`); err == nil {
		t.Error("ot_ro_pages inserted into strategy.result")
	}
	if err := pages.QueryRow(ctx, `SELECT count(*) FROM strategy.result`).Scan(&n); err == nil {
		t.Error("ot_ro_pages read strategy.result")
	}

	runner, err := pgx.Connect(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close(ctx) })
	if err := runner.QueryRow(ctx, `SELECT count(*) FROM rd.strategy_result_r1`).Scan(&n); err == nil {
		t.Error("ot_runner read the view")
	}
}
