//go:build pg

package setup

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const loginPassword = "pg-test-only"

func pgURL(t *testing.T, dbname, user, password string) string {
	t.Helper()
	for _, v := range []string{"PGHOST", "PGPORT", "PGUSER", "PGPASSWORD", "PGDATABASE"} {
		if os.Getenv(v) == "" {
			t.Fatalf("missing environment variable %s", v)
		}
	}
	if user == "" {
		user, password = os.Getenv("PGUSER"), os.Getenv("PGPASSWORD")
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   os.Getenv("PGHOST") + ":" + os.Getenv("PGPORT"),
		Path:   "/" + dbname,
	}
	if m := os.Getenv("PGSSLMODE"); m != "" {
		u.RawQuery = url.Values{"sslmode": {m}}.Encode()
	}
	return u.String()
}

// ensureOwnerRole creates ot_owner when missing, under an advisory lock because roles are shared by every
// database of the server and the tests run in parallel.
func ensureOwnerRole(ctx context.Context, t *testing.T, admin *pgx.Conn) {
	t.Helper()
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`) }()
	for _, r := range []string{"ot_owner", "ot_runner", "ot_w_series_fred", "ot_ro_pages"} {
		var have bool
		if err := admin.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, r).Scan(&have); err != nil {
			t.Fatal(err)
		}
		if !have {
			if _, err := admin.Exec(ctx, "CREATE ROLE "+r+" LOGIN PASSWORD '"+loginPassword+"'"); err != nil {
				t.Fatalf("create role %s: %v", r, err)
			}
		}
	}
}

func randomName(t *testing.T, prefix string) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return prefix + hex.EncodeToString(b)
}

// scratch makes a database owned by ot_owner and a login role (a member of ot_owner when member is set, never a
// superuser). It removes both in t.Cleanup and returns the database and the role.
func scratch(t *testing.T, member bool) (dbname, login string) {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, pgURL(t, os.Getenv("PGDATABASE"), "", ""))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	ensureOwnerRole(ctx, t, admin)
	dbname, login = randomName(t, "ot_mig_"), randomName(t, "ot_login_")
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+dbname+` OWNER ot_owner`); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE `+login+` LOGIN PASSWORD '`+loginPassword+`' NOSUPERUSER`); err != nil {
		t.Fatalf("create login: %v", err)
	}
	if member {
		if _, err := admin.Exec(ctx, `GRANT ot_owner TO `+login); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		defer func() { _ = admin.Close(ctx) }()
		if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS `+dbname+` WITH (FORCE)`); err != nil {
			t.Errorf("drop database: %v", err)
		}
		if _, err := admin.Exec(ctx, `DROP ROLE IF EXISTS `+login); err != nil {
			t.Errorf("drop role: %v", err)
		}
	})
	return dbname, login
}

// openAs returns the open function of migrateWith that connects to dbname as login with password.
func openAs(t *testing.T, dbname, login, password string) func(context.Context, afterConnect) (*sql.DB, error) {
	t.Helper()
	cfg, err := pgx.ParseConfig(pgURL(t, dbname, login, password))
	if err != nil {
		t.Fatal(err)
	}
	return func(_ context.Context, after afterConnect) (*sql.DB, error) {
		db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, c *pgx.Conn) error { return after(ctx, c) }))
		db.SetMaxOpenConns(1)
		return db, nil
	}
}

func tableCount(ctx context.Context, t *testing.T, dbname, where string) int {
	t.Helper()
	c, err := pgx.Connect(ctx, pgURL(t, dbname, "", ""))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close(ctx) }()
	var n int
	if err := c.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE `+where).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrateUpAsOwnerPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbname, login := scratch(t, true)
	fsys := os.DirFS("../../../migrations")
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	for i := 0; i < 2; i++ {
		if err := migrateWith(ctx, openAs(t, dbname, login, loginPassword), gooseUp, fsys, log); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
	if n := tableCount(ctx, t, dbname, `schemaname IN ('master','obs','ops') AND tableowner <> 'ot_owner'`); n != 0 {
		t.Errorf("%d tables not owned by ot_owner", n)
	}
	if n := tableCount(ctx, t, dbname, `schemaname IN ('master','obs','ops')`); n == 0 {
		t.Error("no tables made")
	}
	if n := tableCount(ctx, t, dbname, `schemaname = 'meta' AND tablename = 'goose_db_version' AND tableowner = 'ot_owner'`); n != 1 {
		t.Errorf("meta.goose_db_version owned by ot_owner: %d", n)
	}
	if n := tableCount(ctx, t, dbname, `schemaname = 'public' AND tablename = 'goose_db_version'`); n != 0 {
		t.Error("public.goose_db_version exists")
	}
}

func TestMigrateUpRejectedRolePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dbname, login := scratch(t, false)
	err := migrateWith(ctx, openAs(t, dbname, login, loginPassword), gooseUp, os.DirFS("../../../migrations"), slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	if err == nil || !strings.Contains(err.Error(), "set role") {
		t.Fatalf("err = %v, want stage set role", err)
	}
	if n := tableCount(ctx, t, dbname, `schemaname NOT IN ('pg_catalog','information_schema')`); n != 0 {
		t.Errorf("%d tables exist after a refused role", n)
	}
}

func TestMigrateUpWrongPasswordPG(t *testing.T) {
	t.Parallel()
	const wrong = "wrong-pw-sentinel"
	dbname, login := scratch(t, true)
	var logs bytes.Buffer
	err := migrateWith(context.Background(), openAs(t, dbname, login, wrong), gooseUp, os.DirFS("../../../migrations"), slog.New(slog.NewTextHandler(&logs, nil)))
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), wrong) || strings.Contains(logs.String(), wrong) {
		t.Errorf("password in error %q or log %q", err, &logs)
	}
}
