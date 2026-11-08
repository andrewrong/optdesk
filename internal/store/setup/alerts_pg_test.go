//go:build pg

package setup

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestAlertMigration: the alerts migration applies on a clean database, and ot_w_alerts can insert and select on
// alerts.sent and do nothing more.
func TestAlertMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// ot_w_alerts is shared with TestSetupPG; lock 54 orders the two.
	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	_, err := s.admin.Exec(ctx, "ALTER ROLE ot_w_alerts PASSWORD "+literal(testPassword))
	var w *pgx.Conn
	if err == nil {
		w = connectAs(ctx, t, adminDSN(t, s.db), roleAlerts, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := w.Exec(ctx, `INSERT INTO alerts.sent (rule_id, day, sent_at) VALUES ('r1', '2026-01-02', now())`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := w.Exec(ctx, `INSERT INTO alerts.sent (rule_id, day, sent_at) VALUES ('r1', '2026-01-02', now())`); err == nil {
		t.Error("a second row of the same (rule_id, day) was accepted")
	}
	if _, err := w.Exec(ctx, `INSERT INTO alerts.sent (rule_id, day, sent_at) VALUES ('r1', '2026-01-02', now()) ON CONFLICT DO NOTHING`); err != nil {
		t.Errorf("insert on conflict do nothing: %v", err)
	}
	var n int
	if err := w.QueryRow(ctx, `SELECT count(*) FROM alerts.sent`).Scan(&n); err != nil || n != 1 {
		t.Errorf("select count = %d, %v; want 1", n, err)
	}

	for _, q := range []string{
		`UPDATE alerts.sent SET sent_at = now()`,
		`DELETE FROM alerts.sent`,
		`TRUNCATE alerts.sent`,
		`CREATE TABLE alerts.t (a int)`,
		`SELECT 1 FROM readings.reading`,
		`SELECT 1 FROM master.session_day`,
	} {
		if _, err := w.Exec(ctx, q); !isPermissionError(err) {
			t.Errorf("%s: want a permission error, got %v", q, err)
		}
	}

	inDB, err := pgx.Connect(ctx, adminDSN(t, s.db)) // s.admin is connected to the server's default database
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inDB.Close(ctx) })
	for _, role := range []string{roleRunner, roleRoPages, roleReadings} {
		var ok bool
		if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, 'alerts.sent', 'SELECT')`, role).Scan(&ok); err != nil || ok {
			t.Errorf("%s SELECT on alerts.sent: %v, %v", role, ok, err)
		}
	}
}
