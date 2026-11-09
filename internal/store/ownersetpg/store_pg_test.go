//go:build pg

package ownersetpg

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

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

const rolePassword = "pg-test-only"

// connURL builds the URL of database dbname on the server the libpq variables (PGHOST, PGPORT, PGUSER, PGPASSWORD,
// PGSSLMODE) name; pgx reads them itself, so no variable is read here. user "" is the administrator.
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

// adminDatabase is the database the administrator connects to (PGDATABASE).
func adminDatabase(t *testing.T) string {
	t.Helper()
	cfg, err := pgx.ParseConfig("")
	if err != nil || cfg.Database == "" {
		t.Fatalf("PGDATABASE names no database: %v", err)
	}
	return cfg.Database
}

var nonIdent = regexp.MustCompile(`[^a-z0-9]+`)

// newDatabase makes a throwaway database with the roles, the schema rd and migration 0024, dropped in t.Cleanup.
func newDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, connURL(t, adminDatabase(t), ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(962)`); err != nil {
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
	raw, err := os.ReadFile("../../../migrations/0024_settings.sql")
	if err != nil {
		t.Fatal(err)
	}
	up, _, _ := strings.Cut(string(raw), "-- +goose Down")
	c := pgxAs(t, name)
	if _, err := c.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS rd`); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, strings.Replace(up, "-- +goose Up", "", 1)); err != nil {
		t.Fatalf("apply 0024: %v", err)
	}
	if _, err := admin.Exec(ctx, `GRANT CONNECT ON DATABASE "`+name+`" TO ot_w_settings, ot_ro_pages, ot_runner`); err != nil {
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

// dbNow is the database's clock: set_at comes from it, so the test asks it and holds no clock.
func dbNow(t *testing.T, c *pgx.Conn) time.Time {
	t.Helper()
	var at time.Time
	if err := c.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at
}

func TestSettingsRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, err := Open(ctx, connURL(t, name, "ot_w_settings"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	admin := pgxAs(t, name)

	n, err := st.Apply(ctx, ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "21"}, {Key: "max_dte", Value: "45"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 7, Excluded: true}},
	})
	if err != nil || n != 3 {
		t.Fatalf("apply = %d, %v; want 3", n, err)
	}
	first := dbNow(t, admin)
	n, err = st.Apply(ctx, ownerset.Change{
		Params:     []ownerset.ParamValue{{Key: "min_dte", Value: "21"}, {Key: "max_dte", Value: "50"}},
		Exclusions: []ownerset.ExclusionValue{{InstrumentID: 7, Excluded: true}},
	})
	if err != nil || n != 1 {
		t.Fatalf("second apply = %d, %v; want 1 (only max_dte differs)", n, err)
	}
	later := dbNow(t, admin)
	checkReads(t, st, first, later)
	if got := count(t, admin, "settings.rule_param"); got != 3 {
		t.Errorf("rule_param rows = %d, want 3", got)
	}
	checkPageRole(t, connURL(t, name, "ot_ro_pages"), later)
}

func checkReads(t *testing.T, st ownerset.Store, first, later time.Time) {
	t.Helper()
	ctx := context.Background()
	params, err := st.Params(ctx, later)
	if err != nil || len(params) != 2 || params[0].Key != "max_dte" || params[0].Value != "50" || params[1].Value != "21" {
		t.Fatalf("params = %+v, %v", params, err)
	}
	old, err := st.Params(ctx, first)
	if err != nil || len(old) != 2 || old[0].Value != "45" {
		t.Errorf("params as of the first apply = %+v, %v; max_dte must be 45", old, err)
	}
	ex, err := st.Exclusions(ctx, later)
	if err != nil || len(ex) != 1 || ex[0].InstrumentID != 7 || !ex[0].Excluded {
		t.Errorf("exclusions = %+v, %v", ex, err)
	}
	if v, err := st.Version(ctx, later); err != nil || v < 1 {
		t.Errorf("version = %d, %v", v, err)
	}
	h, err := st.History(ctx, 2)
	if err != nil || len(h) != 2 || h[0].Kind != "param" || h[0].Key != "max_dte" || h[0].Value != "50" {
		t.Errorf("history = %+v, %v", h, err)
	}
}

// checkPageRole: a page role reads and cannot write; Apply of a difference is a Permanent error.
func checkPageRole(t *testing.T, dsn string, at time.Time) {
	t.Helper()
	ctx := context.Background()
	ro, err := Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ro.Close() }()
	if p, err := ro.Params(ctx, at); err != nil || len(p) != 2 {
		t.Errorf("ot_ro_pages params = %+v, %v", p, err)
	}
	_, err = ro.Apply(ctx, ownerset.Change{Params: []ownerset.ParamValue{{Key: "min_dte", Value: "30"}}})
	var te contracts.TypedError
	var pe *pgconn.PgError
	if !errors.As(err, &te) || te.Kind() != contracts.Permanent || !errors.As(err, &pe) || pe.Code != "42501" {
		t.Errorf("ot_ro_pages apply error = %v; want Permanent, SQLSTATE 42501", err)
	}
}

func count(t *testing.T, c *pgx.Conn, table string) int {
	t.Helper()
	var n int
	if err := c.QueryRow(context.Background(), `SELECT count(*) FROM `+table).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
