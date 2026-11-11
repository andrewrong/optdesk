//go:build pg

package underlyingpg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

const rolePassword = "pg-test-only"

var testRoles = []string{"ot_owner", "ot_ro_pages", "ot_runner", "ot_w_settings", "ot_w_underlying_resolve"}

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

// newDatabase makes a throwaway database with the roles and migration 0033, dropped in t.Cleanup.
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
	for _, r := range testRoles {
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
	raw, err := os.ReadFile("../../../migrations/0033_underlying.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c := pgxAs(t, name)
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0033: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_settings, ot_ro_pages, ot_runner, ot_w_underlying_resolve`); err != nil {
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

func openAs(t *testing.T, name, role string) underlying.Store {
	t.Helper()
	st, err := Open(context.Background(), connURL(t, name, role))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// seedRow inserts one row as the administrator with an explicit set_at.
func seedRow(t *testing.T, c *pgx.Conn, key, status string, conID any, at time.Time) {
	t.Helper()
	sym := key[strings.Index(key, ":")+1:]
	_, err := c.Exec(context.Background(), `INSERT INTO settings.underlying `+
		`(name_key, symbol, market, kind, exchange, currency, con_id, status, set_at) `+
		`VALUES ($1, $2, 'US', 'stock', 'SMART', 'USD', $3, $4, $5)`, key, sym, conID, status, at)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewestRowWinsTiesByID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	admin := pgxAs(t, name)
	st := openAs(t, name, "ot_w_settings")
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	seedRow(t, admin, "US:AAPL", "pending", nil, at)
	seedRow(t, admin, "US:AAPL", "active", 265598, at) // same set_at: the larger id wins
	seedRow(t, admin, "US:MSFT", "pending", nil, at.Add(time.Hour))
	seedRow(t, admin, "US:MSFT", "paused", nil, at) // older set_at, larger id: set_at decides
	cur, err := st.Current(ctx)
	if err != nil || len(cur) != 2 {
		t.Fatalf("Current = %+v, %v", cur, err)
	}
	if cur[0].Key != "US:AAPL" || cur[0].Status != "active" || cur[0].ConID != 265598 {
		t.Errorf("AAPL = %+v; want the active row (tie by id)", cur[0])
	}
	if cur[1].Status != "pending" || cur[1].ConID != 0 {
		t.Errorf("MSFT = %+v; want the pending row (newest set_at)", cur[1])
	}
	h, err := st.History(ctx, 3)
	if err != nil || len(h) != 3 || h[0].Key != "US:MSFT" || h[0].Status != "pending" {
		t.Errorf("History = %+v, %v; want newest set_at first", h, err)
	}
}

func TestPendingReturnsPendingAndOldUnresolved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	admin := pgxAs(t, name)
	st := openAs(t, name, "ot_w_underlying_resolve")
	cut := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	seedRow(t, admin, "US:PEND", "pending", nil, cut.Add(time.Hour)) // pending is read whatever its age
	seedRow(t, admin, "US:OLD", "unresolved", nil, cut.Add(-time.Hour))
	seedRow(t, admin, "US:NEW", "unresolved", nil, cut.Add(time.Hour))
	seedRow(t, admin, "US:ACT", "active", 1, cut.Add(-time.Hour))
	seedRow(t, admin, "US:WAS", "pending", nil, cut.Add(-2*time.Hour))
	seedRow(t, admin, "US:WAS", "paused", nil, cut.Add(-time.Hour)) // no longer pending
	rows, err := st.Pending(ctx, cut)
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, r := range rows {
		keys = append(keys, r.Key)
	}
	if got := strings.Join(keys, ","); got != "US:OLD,US:PEND" {
		t.Errorf("Pending = %s; want US:OLD,US:PEND", got)
	}
}

func TestSettleWritesOnlyWhileNewestIsTheRowRead(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	admin := pgxAs(t, name)
	resolve := openAs(t, name, "ot_w_underlying_resolve")
	owner := openAs(t, name, "ot_w_settings")
	at := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	seedRow(t, admin, "US:AAA", "pending", nil, at)
	seedRow(t, admin, "US:BBB", "pending", nil, at)
	rows, err := resolve.Pending(ctx, at)
	if err != nil || len(rows) != 2 {
		t.Fatalf("Pending = %+v, %v", rows, err)
	}
	if n, err := owner.Apply(ctx, underlying.Change{Edits: []underlying.Edit{{Key: "US:BBB", Status: sp("paused")}}}); err != nil || n != 1 {
		t.Fatalf("pause = %d, %v", n, err)
	}
	for _, r := range rows {
		ok, err := resolve.Settle(ctx, underlying.Outcome{Key: r.Key, AfterID: r.ID, ConID: 4242})
		if err != nil || ok != (r.Key == "US:AAA") {
			t.Errorf("Settle %s = %v, %v; want written only for AAA", r.Key, ok, err)
		}
	}
	cur, err := resolve.Current(ctx)
	if err != nil || len(cur) != 2 || cur[0].Status != "active" || cur[0].ConID != 4242 || cur[1].Status != "paused" {
		t.Errorf("Current = %+v, %v; want AAA active 4242 and BBB still paused", cur, err)
	}
}

func TestApplyThroughPostgres(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	owner := openAs(t, name, "ot_w_settings")
	n, err := owner.Apply(ctx, underlying.Change{Adds: []underlying.Add{{Symbol: "AAPL", Market: "US", Kind: "stock", Options: true}}})
	if err != nil || n != 1 {
		t.Fatalf("add = %d, %v", n, err)
	}
	n, err = owner.Apply(ctx, underlying.Change{Edits: []underlying.Edit{{Key: "US:AAPL", Exchange: sp("ARCA")}}})
	cur, _ := owner.Current(ctx)
	if err != nil || n != 1 || len(cur) != 1 || cur[0].Exchange != "ARCA" || cur[0].Status != "pending" {
		t.Errorf("edit = %d, %v, %+v", n, err, cur)
	}
}

func TestPageRoleCannotSelectTheBaseTable(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	admin := pgxAs(t, name)
	seedRow(t, admin, "US:AAPL", "pending", nil, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	ro, err := pgx.Connect(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close(ctx) }()
	_, err = ro.Exec(ctx, `SELECT 1 FROM settings.underlying`)
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "42501" {
		t.Errorf("SELECT on the table as ot_ro_pages: %v; want SQLSTATE 42501", err)
	}
	st := openAs(t, name, "ot_ro_pages")
	if cur, err := st.Current(ctx); err != nil || len(cur) != 1 {
		t.Errorf("view read as ot_ro_pages = %+v, %v", cur, err)
	}
}
