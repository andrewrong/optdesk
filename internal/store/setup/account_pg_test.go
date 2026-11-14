//go:build pg

package setup

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestAccountMigration: the account migration applies on a clean database; ot_w_account can insert into and select
// from the four tables, read the four views and do nothing more; ot_ro_pages reads the views and no table; no other
// role of the list reads a table.
func TestAccountMigration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateLocked(ctx, t, s.openAs(t, testPassword), os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(54)`); err != nil {
		t.Fatal(err)
	}
	_, err := s.admin.Exec(ctx, "ALTER ROLE ot_w_account PASSWORD "+literal(testPassword))
	var w *pgx.Conn
	if err == nil {
		w = connectAs(ctx, t, adminDSN(t, s.db), roleAccount, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(54)`)
	if err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{
		`INSERT INTO account.capture (capture_id, alias, captured_at, session_day, status) VALUES ('a:2026-11-06:r1', 'a', now(), '2026-11-06', 'ok')`,
		`INSERT INTO account.position (capture_id, alias, captured_at, session_day, symbol, sec_type, exchange, currency, expiry,
			strike, multiplier, option_right, con_id, quantity, avg_cost)
			VALUES ('a:2026-11-06:r1', 'a', now(), '2026-11-06', 'AAPL', 'stock', 'SMART', 'USD', '', '', '', '', 1, '10', '150.25')`,
		`INSERT INTO account.value (capture_id, alias, captured_at, session_day, tag, value, currency)
			VALUES ('a:2026-11-06:r1', 'a', now(), '2026-11-06', 'NetLiquidation', '1000.50', 'USD')`,
		`INSERT INTO account.execution (alias, exec_id, perm_id, executed_at, session_day, symbol, sec_type, expiry, strike,
			option_right, multiplier, currency, side, quantity, price, commission, commission_currency)
			VALUES ('a', 'e1', 7, now(), '2026-11-06', 'AAPL', 'stock', '', '', '', '', 'USD', 'BOT', '10', '150.25', '1.00', 'USD')`,
	} {
		if _, err := w.Exec(ctx, q); err != nil {
			t.Fatalf("insert: %v\n%s", err, q)
		}
	}
	if _, err := w.Exec(ctx, `INSERT INTO account.capture (capture_id, alias, captured_at, session_day, status) VALUES ('a:2026-11-06:r1', 'a', now(), '2026-11-06', 'ok')`); err == nil {
		t.Error("a second capture of the same id was accepted")
	}
	if _, err := w.Exec(ctx, `INSERT INTO account.value (capture_id, alias, captured_at, session_day, tag, value, currency)
		VALUES ('nope', 'a', now(), '2026-11-06', 'x', '1', 'USD')`); err == nil {
		t.Error("a value of an unknown capture was accepted")
	}
	for _, v := range []string{"capture", "position", "value", "execution"} {
		var n int
		if err := w.QueryRow(ctx, `SELECT count(*) FROM account.`+v).Scan(&n); err != nil || n != 1 {
			t.Errorf("table %s count = %d, %v; want 1", v, n, err)
		}
		if err := w.QueryRow(ctx, `SELECT count(*) FROM rd.account_`+v+`_r1`).Scan(&n); err != nil || n != 1 {
			t.Errorf("view %s count = %d, %v; want 1", v, n, err)
		}
	}
	for _, q := range []string{
		`UPDATE account.capture SET status = 'x'`,
		`DELETE FROM account.capture`,
		`TRUNCATE account.capture`,
		`UPDATE account.execution SET price = 1`,
		`DELETE FROM account.execution`,
		`TRUNCATE account.execution`,
		`CREATE TABLE account.t (a int)`,
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
	for _, v := range []string{"capture", "position", "value", "execution"} {
		view, table := "rd.account_"+v+"_r1", "account."+v
		for _, c := range []struct {
			role, object, priv string
			want               bool
		}{
			{roleRoPages, view, "SELECT", true},
			{roleRoPages, table, "SELECT", false},
			{roleRoPages, table, "INSERT", false},
			{roleRunner, view, "SELECT", false},
			{roleRunner, table, "SELECT", false},
			{roleAlerts, table, "SELECT", false},
			{roleCandidate, view, "SELECT", false},
			{roleAccount, view, "SELECT", true},
			{roleAccount, table, "INSERT", true},
			{roleAccount, table, "SELECT", true},
			{roleAccount, table, "UPDATE", false},
			{roleAccount, table, "DELETE", false},
			{roleAccount, table, "TRUNCATE", false},
			{roleAccount, view, "INSERT", false},
		} {
			var ok bool
			if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`, c.role, c.object, c.priv).Scan(&ok); err != nil || ok != c.want {
				t.Errorf("%s %s on %s = %v, %v; want %v", c.role, c.priv, c.object, ok, err, c.want)
			}
		}
	}
}
