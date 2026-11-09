//go:build pg

package setup

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestCandidateMigration: the candidates migration applies on a clean database; ot_w_candidate can insert into and
// select from the table, read the view and do nothing more; ot_ro_pages reads the view and not the table; no
// other role of the list reads the table.
func TestCandidateMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	_, err := s.admin.Exec(ctx, "ALTER ROLE ot_w_candidate PASSWORD "+literal(testPassword))
	var w *pgx.Conn
	if err == nil {
		w = connectAs(ctx, t, adminDSN(t, s.db), roleCandidate, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
	if err != nil {
		t.Fatal(err)
	}

	ins := `INSERT INTO candidates.option_candidate (session_day, run_id, kind, instrument_id, alias, rule_version, issuer,
		underlying_id, expiry, dte, option_right, strike, multiplier, bid_eod, break_even, status, gates, marks, fits,
		earnings_state, computed_at)
		VALUES ('2026-11-06', 'r1', 'short_put', 1, '', 'v1', 'AAPL', 2, '2026-12-18', 42, 'put', '180', 100, '2.15',
		'177.85', 'ok', '[]', '[]', '{}', 'listed', now())`
	if _, err := w.Exec(ctx, ins); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var n int
	if err := w.QueryRow(ctx, `SELECT count(*) FROM candidates.option_candidate`).Scan(&n); err != nil || n != 1 {
		t.Errorf("table count = %d, %v; want 1", n, err)
	}
	if err := w.QueryRow(ctx, `SELECT count(*) FROM rd.option_candidate_r1`).Scan(&n); err != nil || n != 1 {
		t.Errorf("view count = %d, %v; want 1", n, err)
	}
	for _, q := range []string{
		`UPDATE candidates.option_candidate SET run_id = 'x'`,
		`DELETE FROM candidates.option_candidate`,
		`TRUNCATE candidates.option_candidate`,
		`CREATE TABLE candidates.t (a int)`,
		`SELECT 1 FROM alerts.sent`,
	} {
		if _, err := w.Exec(ctx, q); !isPermissionError(err) {
			t.Errorf("%s: want a permission error, got %v", q, err)
		}
	}

	inDB, err := pgx.Connect(ctx, adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inDB.Close(ctx) })
	for _, c := range []struct {
		role, object, priv string
		want               bool
	}{
		{roleRoPages, "rd.option_candidate_r1", "SELECT", true},
		{roleRoPages, "candidates.option_candidate", "SELECT", false},
		{roleRunner, "rd.option_candidate_r1", "SELECT", false},
		{roleRunner, "candidates.option_candidate", "SELECT", false},
		{roleAlerts, "candidates.option_candidate", "SELECT", false},
		{roleCandidate, "rd.option_candidate_r1", "SELECT", true},
	} {
		var ok bool
		if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`, c.role, c.object, c.priv).Scan(&ok); err != nil || ok != c.want {
			t.Errorf("%s %s on %s = %v, %v; want %v", c.role, c.priv, c.object, ok, err, c.want)
		}
	}
}
