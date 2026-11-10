//go:build pg

package setup

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func connectAs(ctx context.Context, t *testing.T, dsn, user, password string) *pgx.Conn {
	t.Helper()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("parse config")
	}
	cfg.User, cfg.Password = user, password
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect as %s: %v", user, err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	return c
}

func isPermissionError(err error) bool {
	var pe *pgconn.PgError
	return err != nil && errors.As(err, &pe) && pe.Code == "42501"
}

func TestLoaderGrantsPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	db := "ldr_" + randHex(t)
	if _, err := s.admin.Exec(ctx, `CREATE DATABASE "`+db+`" OWNER ot_owner`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(ctx, adminDSN(t, os.Getenv("PGDATABASE")))
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = c.Close(ctx) }()
		_, _ = c.Exec(ctx, `DROP DATABASE IF EXISTS "`+db+`" WITH (FORCE)`)
	})
	s.db = db
	createTimescale(ctx, t, db) // CREATE EXTENSION timescaledb in the database made above, before the migration
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// ot_loader's password is changed by TestSetupPG; lock 54 orders the two.
	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	_, err := s.admin.Exec(ctx, "ALTER ROLE ot_loader PASSWORD "+literal(testPassword))
	var loader, runner *pgx.Conn
	if err == nil {
		loader = connectAs(ctx, t, adminDSN(t, db), roleLoader, testPassword)
		runner = connectAs(ctx, t, adminDSN(t, db), roleRunner, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
	if err != nil {
		t.Fatal(err)
	}

	const upsert = `INSERT INTO master.session_day (calendar_id, session_day, is_session, open_at, close_at, early_close)
VALUES ('c', '2026-01-02', true, now(), now(), false), ('c', '2026-01-03', true, now(), now(), false), ('c', '2026-01-04', false, now(), now(), false)
ON CONFLICT (calendar_id, session_day) DO UPDATE
SET is_session = $1, open_at = EXCLUDED.open_at, close_at = EXCLUDED.close_at, early_close = EXCLUDED.early_close`
	if _, err := loader.Exec(ctx, upsert, true); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if _, err := loader.Exec(ctx, upsert, false); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if n := s.countIn(t, db, `SELECT count(*) FROM master.session_day WHERE is_session`); n != 0 {
		t.Errorf("%d rows still have is_session after the conflicting upsert", n)
	}

	for _, q := range []string{
		`DELETE FROM master.session_day`,
		`TRUNCATE master.session_day`,
		`UPDATE master.session_day SET calendar_id = 'x'`,
		`UPDATE master.session_day SET session_day = session_day`,
		`CREATE TABLE master.t (a int)`,
		`SELECT 1 FROM obs.series_point`,
		`SELECT 1 FROM ops.capture_run`,
	} {
		if _, err := loader.Exec(ctx, q); !isPermissionError(err) {
			t.Errorf("%s: want a permission error, got %v", q, err)
		}
	}
	for _, schema := range []string{"obs", "ops"} {
		var ok bool
		if err := loader.QueryRow(ctx, `SELECT has_schema_privilege('ot_loader', $1, 'USAGE')`, schema).Scan(&ok); err != nil || ok {
			t.Errorf("USAGE on %s: %v, %v", schema, ok, err)
		}
	}
	// 0028 gives the loader USAGE on rd for the view rd.session_day_r1 and nothing else there.
	var rdUsage bool
	if err := loader.QueryRow(ctx, `SELECT has_schema_privilege('ot_loader', 'rd', 'USAGE')`).Scan(&rdUsage); err != nil || !rdUsage {
		t.Errorf("USAGE on rd: %v, %v; want true", rdUsage, err)
	}
	if _, err := loader.Exec(ctx, `SELECT count(*) FROM rd.session_day_r1`); err != nil {
		t.Errorf("ot_loader select rd.session_day_r1: %v", err)
	}
	if _, err := loader.Exec(ctx, `SELECT 1 FROM rd.option_eod_r1`); !isPermissionError(err) {
		t.Errorf("ot_loader select rd.option_eod_r1: want a permission error, got %v", err)
	}
	if _, err := runner.Exec(ctx, `SELECT count(*) FROM master.session_day`); err != nil {
		t.Errorf("ot_runner select: %v", err)
	}
	if _, err := runner.Exec(ctx, `INSERT INTO master.session_day VALUES ('r', '2026-01-02', true, now(), now(), false)`); !isPermissionError(err) {
		t.Errorf("ot_runner insert: want a permission error, got %v", err)
	}
}

func (s scratch) countIn(t *testing.T, db, q string) int {
	t.Helper()
	s.db = db
	return s.count(t, q)
}

func TestScramVerifierRolePG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, false)
	role := "scr_" + randHex(t)
	const pw = "scram-test-pw"
	if _, err := s.admin.Exec(ctx, "CREATE ROLE "+ident(role)+" LOGIN PASSWORD "+literal(scramVerifier(pw, newSalt()))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.admin.Exec(ctx, "DROP ROLE IF EXISTS "+ident(role)) })
	connectAs(ctx, t, adminDSN(t, os.Getenv("PGDATABASE")), role, pw)

	cfg, err := pgx.ParseConfig(adminDSN(t, os.Getenv("PGDATABASE")))
	if err != nil {
		t.Fatal("parse config")
	}
	cfg.User, cfg.Password = role, "another-"+pw
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err == nil {
		_ = c.Close(ctx)
		t.Fatal("a wrong password was accepted")
	}
	if strings.Contains(err.Error(), pw) {
		t.Error("the password is in the error")
	}
}
