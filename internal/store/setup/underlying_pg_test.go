//go:build pg

package setup

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestUnderlyingResolveRole: the migration applies on a clean database; ot_w_underlying_resolve can insert into and
// select from settings.underlying and read rd.underlying_r1, and cannot insert into settings.rule_param or
// settings.option_exclusion; ot_ro_pages and ot_runner read the view and no table.
func TestUnderlyingResolveRole(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateLocked(ctx, t, s.openAs(t, testPassword), os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(963)`); err != nil {
		t.Fatal(err)
	}
	_, err := s.admin.Exec(ctx, "ALTER ROLE ot_w_underlying_resolve PASSWORD "+literal(testPassword))
	var w *pgx.Conn
	if err == nil {
		w = connectAs(ctx, t, adminDSN(t, s.db), roleUnderlyingResolve, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(963)`)
	if err != nil {
		t.Fatal(err)
	}

	const insert = `INSERT INTO settings.underlying (name_key, symbol, market, kind, exchange, currency, status)
		VALUES ('US:AAPL', 'AAPL', 'US', 'stock', 'SMART', 'USD', 'pending')`
	if _, err := w.Exec(ctx, insert); err != nil {
		t.Fatalf("insert: %v", err)
	}
	var n int
	for _, q := range []string{`SELECT count(*) FROM settings.underlying`, `SELECT count(*) FROM rd.underlying_r1`} {
		if err := w.QueryRow(ctx, q).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s = %d, %v; want 1", q, n, err)
		}
	}
	for _, q := range []string{
		`INSERT INTO settings.rule_param (param_key, value) VALUES ('min_dte', '21')`,
		`INSERT INTO settings.option_exclusion (instrument_id, excluded) VALUES (7, true)`,
		`SELECT 1 FROM settings.rule_param`,
		`SELECT 1 FROM rd.rule_param_r1`,
		`UPDATE settings.underlying SET status = 'active'`,
		`DELETE FROM settings.underlying`,
		`TRUNCATE settings.underlying`,
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
		{roleUnderlyingResolve, "settings.underlying", "INSERT", true},
		{roleUnderlyingResolve, "settings.underlying", "SELECT", true},
		{roleUnderlyingResolve, "rd.underlying_r1", "SELECT", true},
		{roleSettings, "settings.underlying", "INSERT", true},
		{roleSettings, "rd.underlying_r1", "SELECT", true},
		{roleRoPages, "rd.underlying_r1", "SELECT", true},
		{roleRunner, "rd.underlying_r1", "SELECT", true},
		{roleRoPages, "settings.underlying", "SELECT", false},
		{roleRunner, "settings.underlying", "SELECT", false},
		{roleRoPages, "settings.underlying", "INSERT", false},
		{roleRunner, "settings.underlying", "INSERT", false},
		{roleUnderlyingResolve, "settings.rule_param", "INSERT", false},
		{roleUnderlyingResolve, "settings.option_exclusion", "INSERT", false},
		{roleUnderlyingResolve, "settings.underlying", "UPDATE", false},
		{roleSettings, "settings.underlying", "DELETE", false},
	} {
		var ok bool
		if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`, c.role, c.object, c.priv).Scan(&ok); err != nil || ok != c.want {
			t.Errorf("%s %s on %s = %v, %v; want %v", c.role, c.priv, c.object, ok, err, c.want)
		}
	}
}
