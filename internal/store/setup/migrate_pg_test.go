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

const migrationsDir = "../../../migrations"

// ensureOwnerRoles creates the four roles of week 1 when missing, under an advisory lock because roles are shared
// by every database of the server and the tests run in parallel.
func ensureOwnerRoles(ctx context.Context, t *testing.T, admin *pgx.Conn) {
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
			if _, err := admin.Exec(ctx, "CREATE ROLE "+r+" LOGIN PASSWORD '"+testPassword+"'"); err != nil {
				t.Fatalf("create role %s: %v", r, err)
			}
		}
	}
}

func randHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// scratch holds a database owned by ot_owner and a login role that is not a superuser; both are dropped in t.Cleanup.
type scratch struct {
	db    string
	login string
	admin *pgx.Conn
}

// newScratch creates the roles when missing, a scratch database owned by ot_owner and a login role, a member of
// ot_owner when member is true.
func newScratch(t *testing.T, member bool) scratch {
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, adminDSN(t, os.Getenv("PGDATABASE")))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	ensureOwnerRoles(ctx, t, admin)
	s := scratch{db: "mig_" + randHex(t), login: "mig_login_" + randHex(t), admin: admin}
	for _, q := range []string{
		`CREATE DATABASE "` + s.db + `" OWNER ot_owner`,
		`CREATE ROLE "` + s.login + `" LOGIN PASSWORD '` + testPassword + `' NOSUPERUSER`,
	} {
		if _, err := admin.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	if member {
		if _, err := admin.Exec(ctx, `GRANT ot_owner TO "`+s.login+`"`); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		defer func() { _ = admin.Close(ctx) }()
		if _, err := admin.Exec(ctx, `DROP DATABASE IF EXISTS "`+s.db+`" WITH (FORCE)`); err != nil {
			t.Errorf("drop database: %v", err)
		}
		if _, err := admin.Exec(ctx, `DROP ROLE IF EXISTS "`+s.login+`"`); err != nil {
			t.Errorf("drop role: %v", err)
		}
	})
	return s
}

// openAs returns the open function of migrateWith: a connection to the scratch database as login with password,
// with the given after-connect step and one open connection, as MigrateUp does.
func (s scratch) openAs(t *testing.T, password string) func(context.Context, func(context.Context, execer) error) (*sql.DB, error) {
	t.Helper()
	u, err := url.Parse(adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(s.login, password)
	cfg, err := pgx.ParseConfig(u.String())
	if err != nil {
		t.Fatal("parse config")
	}
	return func(ctx context.Context, after func(context.Context, execer) error) (*sql.DB, error) {
		db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(ctx context.Context, c *pgx.Conn) error { return after(ctx, c) }))
		db.SetMaxOpenConns(1)
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, errConnect
		}
		return db, nil
	}
}

func (s scratch) count(t *testing.T, q string, args ...any) int {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var n int
	if err := conn.QueryRow(ctx, q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)) }

func TestMigrateUpAsOwnerPG(t *testing.T) {
	ctx := context.Background()
	s := newScratch(t, true)
	open := s.openAs(t, testPassword)
	for i := 0; i < 2; i++ {
		if err := migrateWith(ctx, open, gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if n := s.count(t, `SELECT count(*) FROM pg_tables WHERE schemaname IN ('master','obs','ops')`); n == 0 {
		t.Fatal("no table was created")
	}
	if n := s.count(t, `SELECT count(*) FROM pg_tables WHERE schemaname IN ('master','obs','ops') AND tableowner <> 'ot_owner'`); n != 0 {
		t.Fatalf("%d tables are not owned by ot_owner", n)
	}
	if n := s.count(t, `SELECT count(*) FROM pg_tables WHERE schemaname = 'meta' AND tablename = 'goose_db_version'`); n != 1 {
		t.Fatal("meta.goose_db_version missing")
	}
	if n := s.count(t, `SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename = 'goose_db_version'`); n != 0 {
		t.Fatal("public.goose_db_version exists")
	}
	if n := s.count(t, `SELECT count(*) FROM pg_namespace n JOIN pg_roles r ON r.oid = n.nspowner WHERE n.nspname = 'meta' AND r.rolname = 'ot_owner'`); n != 1 {
		t.Fatal("schema meta is not owned by ot_owner")
	}
}

func TestMigrateUpRejectedRolePG(t *testing.T) {
	s := newScratch(t, false)
	err := migrateWith(context.Background(), s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog())
	if err == nil || !strings.Contains(err.Error(), "set role") {
		t.Fatalf("want error of stage set role, got %v", err)
	}
	if n := s.count(t, `SELECT count(*) FROM pg_tables WHERE schemaname IN ('master','obs','ops','meta','public')`); n != 0 {
		t.Fatalf("%d tables exist", n)
	}
}

func TestMigrateUpWrongPasswordPG(t *testing.T) {
	s := newScratch(t, true)
	const wrong = "wrong-S3NTINEL"
	var logs bytes.Buffer
	err := migrateWith(context.Background(), s.openAs(t, wrong), gooseUp, os.DirFS(migrationsDir), slog.New(slog.NewTextHandler(&logs, nil)))
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), wrong) || strings.Contains(logs.String(), wrong) {
		t.Fatalf("password leaks into error %q or log %q", err, logs.String())
	}
}
