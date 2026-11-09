//go:build pg

package optionledger

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

// env reads one process environment variable (the pg tests take their connection from PG*).
func env(k string) string {
	v, _ := os.LookupEnv(k)
	return v
}

const rolePassword = "pg-test-only"

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	for _, v := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		if env(v) == "" {
			t.Fatalf("missing environment variable %s", v)
		}
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(env("PGUSER"), env("PGPASSWORD")),
		Host:   env("PGHOST") + ":" + env("PGPORT"),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	if m := env("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles and migration 0022, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, env("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_w_optionledger", "ot_ro_pages"} {
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
	raw, err := os.ReadFile("../../../migrations/0022_optionledger.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c, err := pgx.Connect(ctx, connURL(t, name, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0022: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_optionledger, ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestPGRevisionsAndReader(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	at := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	rec := store.FlexFetchRecord{Login: "main", QueryID: "1", FirstDay: "2026-10-01", LastDay: "2026-11-08",
		StartedAt: at, FinishedAt: at.Add(time.Minute), Status: "ok", ParserVersion: "1"}
	r := func(tx, hash string) store.FlexTradeRow {
		return store.FlexTradeRow{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: tx,
			Attributes: `{"quantity":"1"}`, AttributesSHA256: hash}
	}
	if got, err := st.RecordFlexFetch(ctx, rec, []store.FlexTradeRow{r("t1", "h1"), r("t2", "h2")}); err != nil || got.New != 2 {
		t.Fatalf("first = %+v, %v", got, err)
	}
	rec.StartedAt = at.Add(time.Hour)
	rec.FinishedAt = at.Add(time.Hour + time.Minute)
	if got, err := st.RecordFlexFetch(ctx, rec, []store.FlexTradeRow{r("t1", "h1"), r("t2", "h3")}); err != nil || got.Unchanged != 1 || got.Revised != 1 {
		t.Fatalf("second = %+v, %v", got, err)
	}
	rows, err := st.FlexTradeRows(ctx, "a")
	if err != nil || len(rows) != 2 || rows[0].Revision != 1 || rows[1].Revision != 2 || rows[1].Source != "flex" {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	pgImportAndLatest(ctx, t, st, at)
}

func pgImportAndLatest(ctx context.Context, t *testing.T, st *Store, at time.Time) {
	t.Helper()
	if got, err := st.ImportFlexTradeRows(ctx, []store.FlexTradeRow{
		{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: "t1", Revision: 1, Attributes: `{"quantity":"2"}`, AttributesSHA256: "z"},
		{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: "t3", Revision: 1, Attributes: `{"quantity":"2"}`, AttributesSHA256: "h3", RecordedAt: at},
		{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: "t2", Revision: 7, Attributes: `{"quantity":"1"}`, AttributesSHA256: "h3"},
	}); err != nil || got != (store.FlexImportResult{Inserted: 1, Conflict: 1, Same: 1}) {
		t.Fatalf("import = %+v, %v", got, err)
	}
	last, ok, err := st.LatestOkFetch(ctx, "main", "1")
	if err != nil || !ok || !last.StartedAt.Equal(at.Add(time.Hour)) || last.FirstDay != "2026-10-01" || last.RowsRevised != 1 {
		t.Fatalf("latest = %+v, %v, %v", last, ok, err)
	}
	if al, err := st.StoredAliases(ctx); err != nil || len(al) != 1 || al[0] != "a" {
		t.Errorf("aliases = %v, %v", al, err)
	}
}

// TestPGRoleCannotAlter shows the grants: the role writes and reads, and cannot update, delete or truncate.
func TestPGRoleCannotAlter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	row := store.FlexTradeRow{Alias: "a", LevelOfDetail: "EXECUTION", TransactionID: "t1", Attributes: `{}`, AttributesSHA256: "h"}
	rec := store.FlexFetchRecord{Login: "m", QueryID: "1", StartedAt: time.Unix(1, 0), FinishedAt: time.Unix(2, 0), Status: "ok"}
	if _, err := st.RecordFlexFetch(ctx, rec, []store.FlexTradeRow{row}); err != nil {
		t.Fatal(err)
	}
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_w_optionledger"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, sql := range []string{
		`UPDATE optionledger.flex_trade_row SET source = 'flex'`,
		`DELETE FROM optionledger.flex_trade_row`,
		`TRUNCATE optionledger.flex_trade_row`,
		`UPDATE optionledger.flex_fetch SET status = 'ok'`,
		`DELETE FROM optionledger.flex_fetch`,
	} {
		if _, err := c.Exec(ctx, sql); err == nil {
			t.Errorf("the role could run %q", sql)
		}
	}
}

// TestPGPagesReadViewsOnly shows ot_ro_pages reads the views and no base table.
func TestPGPagesReadViewsOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	for _, v := range []string{"rd.flex_trade_row_r1", "rd.flex_fetch_r1"} {
		if _, err := c.Exec(ctx, `SELECT count(*) FROM `+v); err != nil {
			t.Errorf("read %s: %v", v, err)
		}
	}
	for _, tb := range []string{"optionledger.flex_trade_row", "optionledger.flex_fetch"} {
		if _, err := c.Exec(ctx, `SELECT count(*) FROM `+tb); err == nil {
			t.Errorf("ot_ro_pages read the base table %s", tb)
		}
	}
}
