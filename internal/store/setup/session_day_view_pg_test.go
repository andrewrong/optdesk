//go:build pg

package setup

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// sessionDayDB migrates a scratch database to the head, seeds one session day and returns a connection to it.
// The test acts as a role with SET ROLE, so no password of a shared role is changed.
func sessionDayDB(t *testing.T) (context.Context, *pgx.Conn) {
	t.Helper()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	c, err := pgx.Connect(ctx, adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close(ctx) })
	if _, err := c.Exec(ctx, `INSERT INTO master.session_day (calendar_id, session_day, is_session, open_at, close_at, early_close)
		VALUES ('XNYS', '2026-11-02', true, now(), now(), false)`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return ctx, c
}

// asRole runs query as role in a transaction that is rolled back, so the next role starts clean.
func asRole(ctx context.Context, t *testing.T, c *pgx.Conn, role, query string) error {
	t.Helper()
	tx, err := c.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SET LOCAL ROLE `+role); err != nil {
		t.Fatalf("set role %s: %v", role, err)
	}
	_, err = tx.Exec(ctx, query)
	return err
}

// TestPagesReadSessionDayView: ot_ro_pages reads rd.session_day_r1 and cannot write it.
func TestPagesReadSessionDayView(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	var n int
	var ok bool
	if err := c.QueryRow(ctx, `SELECT has_table_privilege('ot_ro_pages', 'rd.session_day_r1', 'SELECT')`).Scan(&ok); err != nil || !ok {
		t.Fatalf("ot_ro_pages SELECT on rd.session_day_r1 = %v, %v; want true", ok, err)
	}
	if err := asRole(ctx, t, c, roleRoPages, `SELECT calendar_id FROM rd.session_day_r1`); err != nil {
		t.Errorf("select as ot_ro_pages: %v", err)
	}
	for _, q := range []string{
		`INSERT INTO rd.session_day_r1 (calendar_id, session_day, is_session, open_at, close_at, early_close) VALUES ('x', '2026-01-01', true, now(), now(), false)`,
		`UPDATE rd.session_day_r1 SET is_session = false`,
		`DELETE FROM rd.session_day_r1`,
	} {
		if err := asRole(ctx, t, c, roleRoPages, q); !isPermissionError(err) {
			t.Errorf("%s as ot_ro_pages: want a permission error, got %v", q, err)
		}
	}
	if err := c.QueryRow(ctx, `SELECT count(*) FROM rd.session_day_r1`).Scan(&n); err != nil || n != 1 {
		t.Errorf("view count = %d, %v; want 1", n, err)
	}
}

// TestPagesNoMasterUsage: ot_ro_pages has no USAGE on schema master and cannot read master.session_day.
func TestPagesNoMasterUsage(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	var ok bool
	if err := c.QueryRow(ctx, `SELECT has_schema_privilege('ot_ro_pages', 'master', 'USAGE')`).Scan(&ok); err != nil || ok {
		t.Errorf("ot_ro_pages USAGE on master = %v, %v; want false", ok, err)
	}
	if err := asRole(ctx, t, c, roleRoPages, `SELECT 1 FROM master.session_day`); !isPermissionError(err) {
		t.Errorf("select master.session_day as ot_ro_pages: want a permission error, got %v", err)
	}
}

// TestRunnerLoaderReadView: ot_runner and ot_loader read rd.session_day_r1.
func TestRunnerLoaderReadView(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	for _, role := range []string{roleRunner, roleLoader} {
		if err := asRole(ctx, t, c, role, `SELECT calendar_id FROM rd.session_day_r1`); err != nil {
			t.Errorf("select as %s: %v", role, err)
		}
	}
}
