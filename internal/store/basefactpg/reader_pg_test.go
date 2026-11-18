//go:build pg

package basefactpg

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

// The pg tests of the four views run on a database of their own per test. The tables the views read (the rd views
// of the observation tables and the three tables outside rd) are stand-ins with the columns and the types the views
// use, as the other pg tests of the store do for tables they do not own; the four view migrations 0056 to 0059 are
// the real files. A fixture reads as the page role, through the view and nothing else.

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

// standIns creates the schemas and the tables the four views read.
const standIns = `
CREATE SCHEMA IF NOT EXISTS rd;
CREATE SCHEMA IF NOT EXISTS obs;
CREATE SCHEMA IF NOT EXISTS master;
CREATE TABLE master.instrument (instrument_id bigint PRIMARY KEY, symbol text NOT NULL, currency char(3) NOT NULL);
CREATE TABLE rd.price_bar_r1 (instrument_id bigint NOT NULL, session_day date NOT NULL, basis text NOT NULL,
    open text NOT NULL, high text NOT NULL, low text NOT NULL, close text NOT NULL);
CREATE TABLE rd.quote_snapshot_r1 (session_day date NOT NULL, instrument_id bigint NOT NULL, currency text NOT NULL,
    status smallint NOT NULL, received_at timestamptz NOT NULL, market_data_type smallint NOT NULL,
    bid numeric, ask numeric);
CREATE TABLE rd.option_eod_r1 (session_day date NOT NULL, instrument_id bigint NOT NULL, currency text NOT NULL,
    status smallint NOT NULL, received_at timestamptz NOT NULL, market_data_type smallint NOT NULL,
    bid numeric, ask numeric, iv_decimal numeric);
CREATE TABLE rd.option_open_r1 (session_day date NOT NULL, instrument_id bigint NOT NULL, currency text NOT NULL,
    status smallint NOT NULL, received_at timestamptz NOT NULL, market_data_type smallint NOT NULL,
    bid numeric, ask numeric, iv_decimal numeric);
CREATE TABLE rd.option_terms_r1 (valid_from timestamptz NOT NULL);
CREATE TABLE obs.series_definition (series_id text PRIMARY KEY, frequency text NOT NULL, active boolean NOT NULL);
CREATE TABLE rd.series_point_r1 (series_id text NOT NULL, observation_date date NOT NULL, value double precision NOT NULL);
CREATE TABLE rd.fx_rate_r1 (pair text NOT NULL, rate_day date NOT NULL, rate text NOT NULL);
CREATE TABLE rd.earnings_date_r1 (issuer text NOT NULL, status text NOT NULL, report_date date,
    observed_on date NOT NULL, attempt integer NOT NULL);
CREATE TABLE rd.dividend_expectation_r1 (instrument_id integer NOT NULL, status text NOT NULL, next_ex_date date,
    session_day date NOT NULL, attempt integer NOT NULL);
CREATE TABLE obs.macro_event (kind text NOT NULL, event_date date NOT NULL);
CREATE TABLE rd.session_day_r1 (calendar_id text NOT NULL, session_day date NOT NULL, is_session boolean NOT NULL);
CREATE TABLE rd.fundamental_fact_r2 (subject text NOT NULL, filed date NOT NULL, period_start date, period_end date NOT NULL);
CREATE TABLE rd.account_capture_r1 (alias text NOT NULL, capture_id text NOT NULL, captured_at timestamptz NOT NULL,
    session_day date NOT NULL);
CREATE TABLE rd.account_position_r1 (capture_id text NOT NULL, sec_type text NOT NULL, expiry text NOT NULL);
CREATE TABLE rd.account_value_r1 (capture_id text NOT NULL, tag text NOT NULL);
CREATE TABLE rd.flex_section_status_r1 (fetch_id text NOT NULL, alias text NOT NULL, section text NOT NULL,
    state text NOT NULL, recorded_at timestamptz NOT NULL);
CREATE TABLE rd.flex_trade_row_r1 (recorded_at timestamptz NOT NULL);
GRANT USAGE ON SCHEMA rd, obs, master TO ot_owner;
GRANT SELECT ON ALL TABLES IN SCHEMA rd, obs, master TO ot_owner;
GRANT USAGE ON SCHEMA rd TO ot_ro_pages;
`

var viewMigrations = []string{
	"0056_base_fact_bars.sql", "0057_base_fact_option.sql", "0058_base_fact_other.sql", "0059_base_fact_account.sql",
}

// newViewsDatabase makes a throwaway database with the roles, the stand-in tables and the four view migrations,
// dropped in t.Cleanup, and returns its name.
func newViewsDatabase(t *testing.T) string {
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
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(2954)`); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(2954)`)
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
	c := adminConn(t, name)
	pgExec(t, c, standIns)
	for _, f := range viewMigrations {
		raw, err := os.ReadFile("../../../migrations/" + f)
		if err != nil {
			t.Fatal(err)
		}
		up, _, _ := strings.Cut(string(raw), "-- +goose Down")
		pgExec(t, c, strings.Replace(up, "-- +goose Up", "", 1))
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_ro_pages`); err != nil {
		t.Fatal(err)
	}
	return name
}

// adminConn opens the administrator's connection to database name, closed in t.Cleanup.
func adminConn(t *testing.T, name string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, ""))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// pgExec runs one statement (or, without arguments, a script) and stops the test on an error.
func pgExec(t *testing.T, c *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := c.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%.80s: %v", sql, err)
	}
}

// nyDate is the New York date off days after today, as the views compute their Day, as YYYY-MM-DD.
func nyDate(t *testing.T, c *pgx.Conn, off int) string {
	t.Helper()
	var d time.Time
	if err := c.QueryRow(context.Background(),
		`SELECT (now() AT TIME ZONE 'America/New_York')::date + $1::int`, off).Scan(&d); err != nil {
		t.Fatal(err)
	}
	return d.Format("2006-01-02")
}

// factRow is one row of a view as the page role reads it.
type factRow struct {
	N, Bad int64
	Day    string
	Value  string
}

// viewFacts reads every row of the view as ot_ro_pages, keyed by "dataset/rule/subject".
func viewFacts(t *testing.T, name, view string) map[string]factRow {
	t.Helper()
	ctx := context.Background()
	c, err := pgx.Connect(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatalf("connect as ot_ro_pages: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()
	rows, err := c.Query(ctx, `SELECT dataset, rule, subject, n, bad, day, value FROM `+view)
	if err != nil {
		t.Fatalf("read %s: %v", view, err)
	}
	defer rows.Close()
	out := map[string]factRow{}
	for rows.Next() {
		var ds, rule, subject string
		var f factRow
		var day *time.Time
		var value *string
		if err := rows.Scan(&ds, &rule, &subject, &f.N, &f.Bad, &day, &value); err != nil {
			t.Fatal(err)
		}
		if day != nil {
			f.Day = day.Format("2006-01-02")
		}
		if value != nil {
			f.Value = *value
		}
		key := ds + "/" + rule + "/" + subject
		if _, dup := out[key]; dup {
			t.Fatalf("view %s has two rows for %s", view, key)
		}
		out[key] = f
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// wantCounts fails unless the fact of key has n examined rows and bad bad ones.
func wantCounts(t *testing.T, facts map[string]factRow, key string, n, bad int64) {
	t.Helper()
	f, ok := facts[key]
	if !ok {
		t.Errorf("%s: no row", key)
		return
	}
	if f.N != n || f.Bad != bad {
		t.Errorf("%s: n=%d bad=%d, want n=%d bad=%d", key, f.N, f.Bad, n, bad)
	}
}

func TestReaderReadsTheFourRealViews(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	pgExec(t, c, `INSERT INTO master.instrument VALUES (1, 'AAA', 'USD')`)
	pgExec(t, c, `INSERT INTO rd.price_bar_r1 VALUES (1, (now() AT TIME ZONE 'America/New_York')::date - 2, 'trades',
		'10', '12', '9', '11')`)

	r, err := Open(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	res, err := r.Facts(ctx)
	if err != nil {
		t.Fatalf("Facts: %v", err)
	}
	if res.Failed == nil || len(res.Failed) != 0 {
		t.Fatalf("Failed = %#v, want empty and non-nil", res.Failed)
	}
	seen := map[string]bool{}
	for _, f := range res.Facts {
		seen[f.Dataset] = true
		if f.Dataset == "bars" && f.Rule == "newest" && f.Subject == "USD" {
			if want := nyDate(t, c, -2); string(f.Day) != want || f.Value != "AAA close 11" {
				t.Errorf("bars newest USD = day %q value %q, want %q and %q", f.Day, f.Value, want, "AAA close 11")
			}
		}
	}
	for _, ds := range []string{"bars", "quotes", "chains", "open", "contracts", "rates", "fx", "events", "calendar",
		"facts", "account", "flex"} {
		if !seen[ds] {
			t.Errorf("no fact of dataset %s", ds)
		}
	}
}
