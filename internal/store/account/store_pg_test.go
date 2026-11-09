//go:build pg

package account

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

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

// newDatabase makes a throwaway database with the roles, the schema rd and migration 0019, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, os.Getenv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_w_account"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
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
	raw, err := os.ReadFile("../../../migrations/0019_account.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c := pgxAs(t, name)
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS rd`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0019: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_account`); err != nil {
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

func count(t *testing.T, c *pgx.Conn, table string) int {
	t.Helper()
	var n int
	if err := c.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAccountStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_account"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	admin := pgxAs(t, name)

	at := time.Date(2026, 11, 6, 21, 0, 0, 0, time.UTC)
	c := store.Capture{
		ID: "a:2026-11-06:run1", Alias: "a", At: at, SessionDay: "2026-11-06", Status: "ok",
		Values: []store.AccountValue{
			{Tag: "NetLiquidation", Value: "100000.50", Currency: "USD"},
			{Tag: "Cushion", Value: "0.8", Currency: "USD"},
		},
		Positions: []store.Position{
			{Symbol: "XYZ", SecType: "option", Exchange: "SMART", Currency: "USD", Expiry: "2026-12-18", Strike: "50", Multiplier: "100", Right: "put", ConID: 9, Quantity: "-2", AvgCost: "123.45"},
			{Symbol: "XYZ", SecType: "stock", Exchange: "SMART", Currency: "USD", Multiplier: "1", ConID: 3, Quantity: "100", AvgCost: "48.1"},
		},
	}
	if err := st.WriteCapture(ctx, c); err != nil {
		t.Fatal(err)
	}
	// The same capture id again, with other children: nothing is written.
	again := c
	again.Values = []store.AccountValue{{Tag: "Other", Value: "1", Currency: "USD"}}
	again.Positions = nil
	if err := st.WriteCapture(ctx, again); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if n := count(t, admin, "account.capture"); n != 1 {
		t.Errorf("captures = %d", n)
	}
	if n := count(t, admin, "account.value"); n != 2 {
		t.Errorf("values = %d, want 2", n)
	}
	if n := count(t, admin, "account.position"); n != 2 {
		t.Errorf("positions = %d, want 2", n)
	}

	pos, err := st.Positions(ctx, "a", "2026-11-06")
	if err != nil || len(pos) != 2 || pos[0].ConID != 3 || pos[1].ConID != 9 || pos[1].Quantity != "-2" ||
		pos[1].AvgCost != "123.45" || pos[1].Right != "put" || pos[1].SessionDay != "2026-11-06" || !pos[1].CapturedAt.Equal(at) {
		t.Errorf("positions = %+v, %v", pos, err)
	}
	vals, err := st.Values(ctx, "a", "2026-11-06")
	if err != nil || len(vals) != 2 || vals[0].Tag != "Cushion" || vals[1].Value != "100000.50" {
		t.Errorf("values = %+v, %v", vals, err)
	}
	if other, err := st.Positions(ctx, "b", "2026-11-06"); err != nil || other == nil || len(other) != 0 {
		t.Errorf("other alias = %v, %v", other, err)
	}

	execs := []store.Execution{
		{ExecID: "e2", PermID: 11, ExecutedAt: at, SessionDay: "2026-11-06", Symbol: "XYZ", SecType: "option", Expiry: "2026-12-18", Strike: "50", Right: "put", Multiplier: "100", Currency: "USD", Side: "SLD", Quantity: "2", Price: "1.60", Commission: "1.30", CommissionCurrency: "USD"},
		{ExecID: "e1", PermID: 10, ExecutedAt: at, SessionDay: "2026-11-06", Symbol: "XYZ", SecType: "stock", Multiplier: "1", Currency: "USD", Side: "BOT", Quantity: "100", Price: "48.1", Commission: "1", CommissionCurrency: "USD"},
	}
	for i := 0; i < 2; i++ {
		if err := st.WriteExecutions(ctx, "a", execs); err != nil {
			t.Fatalf("write executions %d: %v", i, err)
		}
	}
	if n := count(t, admin, "account.execution"); n != 2 {
		t.Errorf("executions = %d, want 2", n)
	}
	got, err := st.Executions(ctx, "a", "2026-11-01", "2026-11-06")
	if err != nil || len(got) != 2 || got[0].ExecID != "e1" || got[1].Price != "1.60" || got[1].Alias != "a" {
		t.Errorf("executions = %+v, %v", got, err)
	}
	if none, err := st.Executions(ctx, "a", "2026-11-07", "2026-11-08"); err != nil || none == nil || len(none) != 0 {
		t.Errorf("empty range = %v, %v", none, err)
	}
}
