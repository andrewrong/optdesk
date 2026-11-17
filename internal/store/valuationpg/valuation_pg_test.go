//go:build pg

package valuationpg

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

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const rolePassword = "pg-test-only"

// roleLockKey is the advisory lock, on the admin database, that every pg test holds while it creates the roles all
// databases of the server share (the same key as the other store packages).
const roleLockKey = 962

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

// migrationParts returns the Up and the Down text of migration 0052.
func migrationParts(t *testing.T) (up, down string) {
	t.Helper()
	raw, err := os.ReadFile("../../../migrations/0052_valuation.sql")
	if err != nil {
		t.Fatal(err)
	}
	u, d, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("0052 has no Down part")
	}
	return strings.Replace(u, "-- +goose Up", "", 1), d
}

// newDatabase makes a throwaway database with the roles, the schema rd and migration 0052 applied, dropped in
// t.Cleanup. It returns the name and an administrator connection to it.
func newDatabase(t *testing.T) (string, *pgx.Conn) {
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
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock($1)`, roleLockKey); err != nil {
		t.Fatal(err)
	}
	for _, r := range []string{"ot_owner", "ot_ro_pages", "ot_runner", "ot_w_settings"} {
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
	_, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock($1)`, roleLockKey)
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
	c := connectAs(t, name, "")
	up, _ := migrationParts(t)
	if _, err := c.Exec(ctx, up); err != nil {
		t.Fatalf("apply 0052: %v", err)
	}
	return name, c
}

func connectAs(t *testing.T, name, user string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), connURL(t, name, user))
	if err != nil {
		t.Fatalf("connect as %q: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(context.Background()) })
	return c
}

// sqlState is the SQLSTATE of err, "" when it is not a database error.
func sqlState(err error) string {
	var pe *pgconn.PgError
	if errors.As(err, &pe) {
		return pe.Code
	}
	return ""
}

// inserts is one valid INSERT per table.
var inserts = map[string]string{
	"company_profile": `INSERT INTO valuation.company_profile (company_key, name, currency, r_market, pool) VALUES ('US:AAA', 'Aaa', 'USD', 'US', 'in_pool')`,
	"review":          `INSERT INTO valuation.review (review_id, company_key, status, source) VALUES ('r1', 'US:AAA', 'draft', 'owner')`,
	"r_version":       `INSERT INTO valuation.r_version (effective_date, r_us, r_jp, r_hkcn) VALUES ('2026-01-02', '1.1', '1.2', '1.3')`,
	"r_table":         `INSERT INTO valuation.r_table (effective_date, yield_up_to, us, jp, hkcn) VALUES ('2026-01-02', '3.0', '1.1', '1.2', '1.3')`,
	"margin_table":    `INSERT INTO valuation.margin_table (effective_date, p_from, m, w, h, s) VALUES ('2026-01-02', '1', '0.3', '0.1', '0.2', '0.4')`,
}

var viewOf = map[string]string{
	"company_profile": "rd.valuation_company_profile_r1",
	"review":          "rd.valuation_review_r1",
	"r_version":       "rd.valuation_r_version_r1",
	"r_table":         "rd.valuation_r_table_r1",
	"margin_table":    "rd.valuation_margin_table_r1",
}

func TestValuationTablesAsRoles(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, _ := newDatabase(t)
	w := connectAs(t, name, "ot_w_settings")
	pages := connectAs(t, name, "ot_ro_pages")
	for table, ins := range inserts {
		if _, err := w.Exec(ctx, ins); err != nil {
			t.Errorf("INSERT into %s as ot_w_settings: %v", table, err)
		}
		if _, err := w.Exec(ctx, `UPDATE valuation.`+table+` SET id = id`); sqlState(err) != "42501" {
			t.Errorf("UPDATE %s as ot_w_settings: err = %v, want permission denied", table, err)
		}
		if _, err := w.Exec(ctx, `DELETE FROM valuation.`+table); sqlState(err) != "42501" {
			t.Errorf("DELETE %s as ot_w_settings: err = %v, want permission denied", table, err)
		}
		if _, err := w.Exec(ctx, `SELECT count(*) FROM `+viewOf[table]); err != nil {
			t.Errorf("SELECT %s as ot_w_settings: %v", viewOf[table], err)
		}
		var n int
		if err := pages.QueryRow(ctx, `SELECT count(*) FROM `+viewOf[table]).Scan(&n); err != nil || n != 1 {
			t.Errorf("SELECT %s as ot_ro_pages = %d, %v; want 1 row", viewOf[table], n, err)
		}
		if _, err := pages.Exec(ctx, `SELECT count(*) FROM valuation.`+table); sqlState(err) != "42501" {
			t.Errorf("SELECT valuation.%s as ot_ro_pages: err = %v, want permission denied", table, err)
		}
		if _, err := pages.Exec(ctx, ins); sqlState(err) != "42501" {
			t.Errorf("INSERT into %s as ot_ro_pages: err = %v, want permission denied", table, err)
		}
	}
}

func TestRequiredColumnsUnlessDraft(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, _ := newDatabase(t)
	w := connectAs(t, name, "ot_w_settings")
	if _, err := w.Exec(ctx, `INSERT INTO valuation.review (review_id, company_key, status, source) VALUES ('d', 'US:AAA', 'draft', 'owner')`); err != nil {
		t.Fatalf("draft with null type, version_date, v and p: %v", err)
	}
	full := map[string]string{"type": "'first'", "version_date": "'2026-01-02'", "v": "'10'", "p": "'3'"}
	for _, status := range []string{"effective", "void"} {
		for _, missing := range []string{"type", "version_date", "v", "p"} {
			cols, vals := []string{"review_id", "company_key", "status", "source"}, []string{"'x_" + status + missing + "'", "'US:AAA'", "'" + status + "'", "'owner'"}
			for c, v := range full {
				if c != missing {
					cols, vals = append(cols, c), append(vals, v)
				}
			}
			q := `INSERT INTO valuation.review (` + strings.Join(cols, ", ") + `) VALUES (` + strings.Join(vals, ", ") + `)`
			if _, err := w.Exec(ctx, q); sqlState(err) != "23514" {
				t.Errorf("%s row without %s: err = %v, want a check violation", status, missing, err)
			}
		}
	}
	// With all four, an effective row is accepted and its effective_at is set by the database.
	if _, err := w.Exec(ctx, `INSERT INTO valuation.review (review_id, company_key, status, source, type, version_date, v, p, effective_at) `+
		`VALUES ('ok', 'US:AAA', 'effective', 'owner', 'first', '2026-01-02', '10', '3', NULL)`); err != nil {
		t.Fatalf("effective row with the four values: %v", err)
	}
	var set, draftSet bool
	if err := w.QueryRow(ctx, `SELECT (SELECT effective_at IS NOT NULL FROM valuation.review WHERE review_id = 'ok'), `+
		`(SELECT effective_at IS NOT NULL FROM valuation.review WHERE review_id = 'd')`).Scan(&set, &draftSet); err != nil || !set || draftSet {
		t.Errorf("effective_at set on effective = %v, on draft = %v, err = %v; want true, false", set, draftSet, err)
	}
	for _, q := range []string{
		`INSERT INTO valuation.review (review_id, company_key, status, source) VALUES ('s', 'US:AAA', 'closed', 'owner')`,
		`INSERT INTO valuation.review (review_id, company_key, status, source) VALUES ('s', 'US:AAA', 'draft', 'robot')`,
	} {
		if _, err := w.Exec(ctx, q); sqlState(err) != "23514" {
			t.Errorf("bad status or source: err = %v, want a check violation", err)
		}
	}
}

func TestEffectiveAndVoidPartialIndexes(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name, _ := newDatabase(t)
	w := connectAs(t, name, "ot_w_settings")
	row := func(id, status string) string {
		return `INSERT INTO valuation.review (review_id, company_key, status, source, type, version_date, v, p) ` +
			`VALUES ('` + id + `', 'US:AAA', '` + status + `', 'owner', 'first', '2026-01-02', '10', '3')`
	}
	for _, q := range []string{row("r1", "draft"), row("r1", "draft"), row("r1", "effective"), row("r1", "void")} {
		if _, err := w.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for _, status := range []string{"effective", "void"} {
		if _, err := w.Exec(ctx, row("r1", status)); sqlState(err) != "23505" {
			t.Errorf("second %s row of one review_id: err = %v, want a unique violation", status, err)
		}
		if _, err := w.Exec(ctx, row("r2", status)); err != nil {
			t.Errorf("%s row of another review_id: %v", status, err)
		}
	}
}

func TestValuationDown(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	_, down := migrationParts(t)
	down = strings.Replace(down, "-- +goose StatementBegin", "", 1)
	down = strings.Replace(down, "-- +goose StatementEnd", "", 1)
	exists := func(c *pgx.Conn) (schema bool, views int) {
		t.Helper()
		if err := c.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'valuation'), `+
			`(SELECT count(*) FROM pg_views WHERE schemaname = 'rd' AND viewname LIKE 'valuation\_%')`).Scan(&schema, &views); err != nil {
			t.Fatal(err)
		}
		return schema, views
	}

	_, empty := newDatabase(t)
	if schema, views := exists(empty); !schema || views != 5 {
		t.Fatalf("after Up: schema %v, views %d; want true, 5", schema, views)
	}
	if _, err := empty.Exec(ctx, down); err != nil {
		t.Fatalf("Down on empty tables: %v", err)
	}
	if schema, views := exists(empty); schema || views != 0 {
		t.Errorf("after Down: schema %v, views %d; want everything removed", schema, views)
	}

	_, full := newDatabase(t)
	if _, err := full.Exec(ctx, inserts["company_profile"]); err != nil {
		t.Fatal(err)
	}
	_, err := full.Exec(ctx, down)
	var pe *pgconn.PgError
	if !errors.As(err, &pe) || pe.Code != "P0001" || !strings.Contains(pe.Message, "0052 down") {
		t.Fatalf("Down with a profile row: err = %v, want the exception of 0052", err)
	}
	if schema, views := exists(full); !schema || views != 5 {
		t.Errorf("after the refused Down: schema %v, views %d; want the tables and views left", schema, views)
	}
	var n int
	if err := full.QueryRow(ctx, `SELECT count(*) FROM valuation.company_profile`).Scan(&n); err != nil || n != 1 {
		t.Errorf("profile rows after the refused Down = %d, %v; want 1", n, err)
	}
}
