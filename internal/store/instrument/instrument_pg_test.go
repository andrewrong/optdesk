//go:build pg

package instrument

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/store"
)

const rolePassword = "pg-test-only"

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

func pgEnv(key string) string { v, _ := os.LookupEnv(key); return v }

func connURL(t *testing.T, dbname, user string) string {
	t.Helper()
	for _, v := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		if pgEnv(v) == "" {
			t.Fatalf("missing environment variable %s", v)
		}
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(pgEnv("PGUSER"), pgEnv("PGPASSWORD")),
		Host:   pgEnv("PGHOST") + ":" + pgEnv("PGPORT"),
		Path:   "/" + dbname,
	}
	if user != "" {
		u.User = url.UserPassword(user, rolePassword)
	}
	if m := pgEnv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

func applyMigration(t *testing.T, dsn, file string) {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/" + file)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	up = strings.Replace(up, "-- +goose Up", "", 1)
	ctx := context.Background()
	c, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, up); err != nil {
		t.Fatalf("apply %s: %v", file, err)
	}
}

// newDatabase is newDatabaseBefore0044 with 0044_instrument_company.sql applied.
func newDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabaseBefore0044(t)
	applyMigration(t, connURL(t, name, ""), "0044_instrument_company.sql")
	return name
}

// newDatabaseBefore0044 creates a database of a random name with the Timescale extension and the roles, applies 0001
// to 0008 there (0004 and 0005 need ot_runner and ot_loader) and 0032 and 0033, which 0044 reads (option_contract and
// settings.underlying; they need nothing beyond 0008 and their own roles), and drops it in t.Cleanup. The roles are
// shared by the server; advisory lock 54 is held until the admin connection closes, as the other pg tests do.
func newDatabaseBefore0044(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, pgEnv("PGDATABASE"), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	roles := []string{"ot_owner", "ot_runner", "ot_w_series_fred", "ot_ro_pages", "ot_loader", "ot_w_option_eod", "ot_w_quote_snapshot", "ot_w_master",
		"ot_w_option_open", "ot_w_settings", "ot_w_underlying_resolve"}
	for _, r := range roles {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		verb := "ALTER"
		if !have {
			verb = "CREATE"
		}
		if _, err := admin.Exec(ctx, verb+" ROLE "+r+" LOGIN PASSWORD '"+rolePassword+"'"); err != nil {
			t.Fatalf("%s role %s: %v", strings.ToLower(verb), r, err)
		}
	}
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
		t.Fatalf("connect new database: %v", err)
	}
	_, err = ext.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb`)
	_ = ext.Close(ctx)
	if err != nil {
		t.Fatalf("create extension: %v", err)
	}
	for _, f := range []string{"0001_master_session_day.sql", "0002_series.sql", "0003_ops_ledger.sql", "0004_session_day_grant.sql",
		"0005_session_day_loader.sql", "0006_option_eod.sql", "0007_quote_snapshot.sql", "0008_master_instrument.sql"} {
		applyMigration(t, connURL(t, name, ""), f)
		if f == "0001_master_session_day.sql" {
			// goose runs as ot_owner, which owns schema master; the foreign-key checks on master.instrument run as the
			// table owner, so it needs the schema. Applying as the administrator needs the same ownership here.
			own, err := pgx.Connect(ctx, connURL(t, name, ""))
			if err != nil {
				t.Fatal(err)
			}
			_, err = own.Exec(ctx, `ALTER SCHEMA master OWNER TO ot_owner`)
			_ = own.Close(ctx)
			if err != nil {
				t.Fatalf("alter schema owner: %v", err)
			}
		}
	}
	for _, f := range []string{"0032_option_contract.sql", "0033_underlying.sql"} {
		applyMigration(t, connURL(t, name, ""), f)
	}
	for _, r := range []string{"ot_runner", "ot_ro_pages", "ot_w_master", "ot_w_option_eod", "ot_w_quote_snapshot"} {
		if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO `+r); err != nil {
			t.Fatal(err)
		}
	}
	return name
}

func connectAs(t *testing.T, name, user string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatalf("connect as %s: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

func mustFail(t *testing.T, c *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql, args...); err == nil {
		t.Errorf("statement was allowed: %s", sql)
	}
}

func openMaster(t *testing.T, name string) store.InstrumentStore {
	t.Helper()
	s, err := Open(context.Background(), connURL(t, name, "ot_w_master"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestMigration0008PG applies 0001 to 0008 (newDatabase fails when any step, the compressed-hypertable keys included,
// fails) and checks that both observation tables still have compression on, and that the owner can read the tables.
func TestMigration0008PG(t *testing.T) {
	name := newDatabase(t)
	c := connectAs(t, name, "")
	ctx := context.Background()
	var n int
	err := c.QueryRow(ctx, `SELECT count(*) FROM timescaledb_information.hypertables
WHERE hypertable_schema = 'obs' AND compression_enabled`).Scan(&n)
	if err != nil || n != 2 {
		t.Fatalf("compressed hypertables = %d, %v; want 2", n, err)
	}
	err = c.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND confrelid = 'master.instrument'::regclass
AND conrelid IN ('obs.option_eod'::regclass, 'obs.quote_snapshot'::regclass)`).Scan(&n)
	if err != nil || n != 3 {
		t.Fatalf("foreign keys to master.instrument = %d, %v; want 3", n, err)
	}
}

func TestForeignKeysPG(t *testing.T) {
	name := newDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	var id int64
	err := owner.QueryRow(ctx, `INSERT INTO master.instrument (instrument_type, name, currency, mic, calendar_id, status, price_increment, lot_size, recorded_at)
VALUES ('stock', 'SPY', 'USD', 'XARC', 'XNYS', 'active', 0.01, 1, now()) RETURNING instrument_id`).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	const eod = `INSERT INTO obs.option_eod (session_day, capture_id, instrument_id, underlying_id, generation, market_data_type,
  received_at, status, currency, snapshot_kind, underlying_ccy)
VALUES ('2026-11-06', '00000000-0000-4000-8000-000000000001', $1, $2, 1, 1, now(), 4, 'USD', 'eod', 'USD')`
	mustFail(t, owner, eod, int64(999999), id)
	mustFail(t, owner, eod, id, int64(999999))
	mustFail(t, owner, `INSERT INTO obs.quote_snapshot (session_day, capture_id, slot, instrument_id, received_at, generation, market_data_type, status, currency)
VALUES ('2026-11-06', '00000000-0000-4000-8000-000000000001', '1000', 999999, now(), 1, 1, 4, 'USD')`)
	if _, err := owner.Exec(ctx, eod, id, id); err != nil {
		t.Fatalf("insert with known ids: %v", err)
	}
}

func TestConcurrentResolvePG(t *testing.T) {
	name := newDatabase(t)
	s := openMaster(t, name)
	key := store.Identifier{Scheme: "ibkr_conid", Value: "756733"}
	spec := store.InstrumentSpec{Type: "stock", Name: "SPY", Currency: "USD", MIC: "XARC", CalendarID: "XNYS", PriceIncrement: "0.01", LotSize: "1"}
	const n = 8
	ids := make([]int64, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids[i], errs[i] = s.ResolveInstrument(context.Background(), key, spec, []store.Identifier{{Scheme: "ticker_exchange", Value: "SPY:ARCA"}})
		}()
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("resolve %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("ids differ: %v", ids)
		}
	}
	got, err := s.Instruments(context.Background(), []int64{ids[0], 424242})
	if err != nil || len(got) != 1 || len(got[0].Identifiers) != 2 || got[0].Spec.PriceIncrement != "0.01" || got[0].Spec.Status != "active" {
		t.Fatalf("Instruments = %+v, %v", got, err)
	}
}

func TestMasterGrantsPG(t *testing.T) {
	name := newDatabase(t)
	s := openMaster(t, name)
	id, err := s.ResolveInstrument(context.Background(), store.Identifier{Scheme: "isin", Value: "US78462F1030"},
		store.InstrumentSpec{Type: "stock", Name: "SPY", Currency: "USD", MIC: "XARC", CalendarID: "XNYS", PriceIncrement: "0.01", LotSize: "1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := connectAs(t, name, "ot_w_master")
	mustFail(t, w, `UPDATE master.instrument SET name = 'x' WHERE instrument_id = $1`, id)
	mustFail(t, w, `DELETE FROM master.instrument WHERE instrument_id = $1`, id)
	mustFail(t, w, `UPDATE master.instrument_identifier SET valid_to = now()`)
	mustFail(t, w, `DELETE FROM master.instrument_identifier`)
	mustFail(t, w, `TRUNCATE master.instrument_identifier`)
	for _, role := range []string{"ot_runner"} {
		c := connectAs(t, name, role)
		var n int
		if err := c.QueryRow(context.Background(), `SELECT count(*) FROM master.instrument`).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s reads %d, %v", role, n, err)
		}
		mustFail(t, c, `INSERT INTO master.instrument_identifier (instrument_id, scheme, value, valid_from) VALUES (1, 'isin', 'x', now())`)
	}
}
