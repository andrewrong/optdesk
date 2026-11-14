//go:build pg

package setup

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

const migrationsDir = "../../../migrations"

// migrationLockKey is the advisory lock, on the admin database, that every test-side application of migration
// files holds while it runs: the migrations write the roles, which every database of the server shares.
// Lock order: key 54 (role passwords), where a test takes it, is taken before 8008; 8008 is held only while a
// migration runs and nothing requests 54 or any other key while it is held, so the two cannot form a cycle.
const migrationLockKey = 8008

// migrateLocked is migrateWith with gooseUp, run while holding migrationLockKey on a connection of its own.
func migrateLocked(
	ctx context.Context,
	t *testing.T,
	open func(context.Context, func(context.Context, execer) error) (*sql.DB, error),
	fsys fs.FS,
	log *slog.Logger,
) error {
	t.Helper()
	c, err := pgx.Connect(ctx, adminDSN(t, os.Getenv("PGDATABASE")))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		t.Fatalf("migration lock: %v", err)
	}
	defer func() { _, _ = c.Exec(ctx, `SELECT pg_advisory_unlock($1)`, migrationLockKey) }()
	return migrateWith(ctx, open, gooseUp, fsys, log)
}

// ensureOwnerRoles creates the roles of `roles` when missing, under an advisory lock because roles are shared
// by every database of the server and the tests run in parallel.
func ensureOwnerRoles(ctx context.Context, t *testing.T, admin *pgx.Conn) {
	t.Helper()
	if _, err := admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`) }()
	for _, spec := range roles {
		r := spec.name
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

// createTimescale creates the Timescale extension in database db through an admin connection to it, as the
// migrations of the writer tables need it and ot_owner is not a superuser.
func createTimescale(ctx context.Context, t *testing.T, db string) {
	t.Helper()
	c, err := pgx.Connect(ctx, adminDSN(t, db))
	if err != nil {
		t.Fatalf("connect admin to %s: %v", db, err)
	}
	defer func() { _ = c.Close(ctx) }()
	if _, err := c.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS timescaledb`); err != nil {
		t.Fatalf("create extension timescaledb in %s: %v", db, err)
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
	createTimescale(ctx, t, s.db)
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
		if err := migrateLocked(ctx, t, open, os.DirFS(migrationsDir), quietLog()); err != nil {
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
	err := migrateLocked(context.Background(), t, s.openAs(t, testPassword), os.DirFS(migrationsDir), quietLog())
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
	// migrateWith, not migrateLocked: the connection fails (wrong password) before any migration runs, so no migration lock is needed.
	err := migrateWith(context.Background(), s.openAs(t, wrong), gooseUp, os.DirFS(migrationsDir), slog.New(slog.NewTextHandler(&logs, nil)))
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), wrong) || strings.Contains(logs.String(), wrong) {
		t.Fatalf("password leaks into error %q or log %q", err, logs.String())
	}
}

// sentinelOpen returns an open function for migrateWith whose connection under test is the DSN dsn, as MigrateUp
// builds it: a parsed pgx config and a ping that maps every failure to errConnect.
func sentinelOpen(t *testing.T, dsn string) func(context.Context, func(context.Context, execer) error) (*sql.DB, error) {
	t.Helper()
	cfg, err := pgx.ParseConfig(dsn)
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

// TestMigrateLogsNoSecret: with a sentinel DSN and password in the connection under test, the error and the log
// hold neither; a log line carries only the message and a role or migration name, never connection data.
func TestMigrateLogsNoSecret(t *testing.T) {
	t.Parallel()
	const dsn = "postgres://sentinel-user:sentinel-dsn-secret@127.0.0.1:1/x?connect_timeout=1"
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	// migrateWith, not migrateLocked: the connection fails (sentinel DSN) before any migration runs, so no migration lock is needed.
	err := migrateWith(context.Background(), sentinelOpen(t, dsn), gooseUp, os.DirFS(migrationsDir), log)
	if err == nil {
		t.Fatal("want an error")
	}
	s := newScratch(t, true)
	const wrong = "wrong-S3NTINEL-pw"
	// migrateWith, not migrateLocked: the connection fails (wrong password) before any migration runs, so no migration lock is needed.
	errWrong := migrateWith(context.Background(), s.openAs(t, wrong), gooseUp, os.DirFS(migrationsDir), log)
	if errWrong == nil {
		t.Fatal("want an error for the wrong password")
	}
	for _, secret := range []string{wrong, dsn, "sentinel-dsn-secret", "sentinel-user", "127.0.0.1"} {
		if strings.Contains(err.Error(), secret) || strings.Contains(errWrong.Error(), secret) || strings.Contains(logs.String(), secret) {
			t.Fatalf("%q leaks into an error or the log %q", secret, logs.String())
		}
	}
	// A run that succeeds logs names only.
	logs.Reset()
	ok := newScratch(t, true)
	if err := migrateLocked(context.Background(), t, ok.openAs(t, testPassword), os.DirFS(migrationsDir), log); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		for _, field := range strings.Fields(line) {
			k, _, _ := strings.Cut(field, "=")
			switch k {
			case "time", "level", "msg", "role", "migration", "version":
			default:
				if strings.Contains(field, "=") {
					t.Errorf("log line %q carries the field %q", line, field)
				}
			}
		}
		if strings.Contains(line, testPassword) || strings.Contains(line, "postgres://") || strings.Contains(line, ok.login) {
			t.Errorf("log line %q holds connection data", line)
		}
	}
}

// TestMigrateLogsNoSentinelDSN: MigrateUp with an unreachable sentinel DSN returns an error without the secret.
func TestMigrateLogsNoSentinelDSN(t *testing.T) {
	t.Parallel()
	const dsn = "postgres://sentinel-user:sentinel-dsn-secret@127.0.0.1:1/x?connect_timeout=1"
	err := MigrateUp(context.Background(), dsn, os.DirFS(migrationsDir))
	if err == nil || strings.Contains(err.Error(), "sentinel-dsn-secret") {
		t.Fatalf("err = %v", err)
	}
}
