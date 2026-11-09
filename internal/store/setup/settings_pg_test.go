//go:build pg

package setup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestSettingsRolesCannotUpdate: the settings migration applies on a clean database; ot_w_settings can insert into
// and select from the two tables, read the two views and do nothing more; ot_ro_pages and ot_runner read the views
// and no table; no role of the list may update, delete or truncate a row.
func TestSettingsRolesCannotUpdate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if _, err := s.admin.Exec(ctx, `SELECT pg_advisory_lock(962)`); err != nil {
		t.Fatal(err)
	}
	_, err := s.admin.Exec(ctx, "ALTER ROLE ot_w_settings PASSWORD "+literal(testPassword))
	var w *pgx.Conn
	if err == nil {
		w = connectAs(ctx, t, adminDSN(t, s.db), roleSettings, testPassword)
	}
	_, _ = s.admin.Exec(ctx, `SELECT pg_advisory_unlock(962)`)
	if err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{
		`INSERT INTO settings.rule_param (param_key, value) VALUES ('min_dte', '21')`,
		`INSERT INTO settings.option_exclusion (instrument_id, excluded) VALUES (7, true)`,
	} {
		if _, err := w.Exec(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var n int
	for _, q := range []string{
		`SELECT count(*) FROM settings.rule_param`, `SELECT count(*) FROM settings.option_exclusion`,
		`SELECT count(*) FROM rd.rule_param_r1`, `SELECT count(*) FROM rd.option_exclusion_r1`,
	} {
		if err := w.QueryRow(ctx, q).Scan(&n); err != nil || n != 1 {
			t.Errorf("%s = %d, %v; want 1", q, n, err)
		}
	}
	for _, q := range []string{
		`UPDATE settings.rule_param SET value = 'x'`,
		`DELETE FROM settings.rule_param`,
		`TRUNCATE settings.rule_param`,
		`UPDATE settings.option_exclusion SET excluded = false`,
		`DELETE FROM settings.option_exclusion`,
		`TRUNCATE settings.option_exclusion`,
		`CREATE TABLE settings.t (a int)`,
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
	checkSettingsPrivileges(ctx, t, inDB)
}

// checkSettingsPrivileges asks the catalog what each role holds on the tables and the views.
func checkSettingsPrivileges(ctx context.Context, t *testing.T, inDB *pgx.Conn) {
	t.Helper()
	for _, role := range []string{roleSettings, roleRoPages, roleRunner, roleAlerts, roleCandidate} {
		for _, table := range []string{"settings.rule_param", "settings.option_exclusion"} {
			for _, priv := range []string{"UPDATE", "DELETE", "TRUNCATE"} {
				var ok bool
				if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`, role, table, priv).Scan(&ok); err != nil || ok {
					t.Errorf("%s %s on %s = %v, %v; want false", role, priv, table, ok, err)
				}
			}
		}
	}
	for _, c := range []struct {
		role, object, priv string
		want               bool
	}{
		{roleRoPages, "rd.rule_param_r1", "SELECT", true},
		{roleRoPages, "rd.option_exclusion_r1", "SELECT", true},
		{roleRunner, "rd.rule_param_r1", "SELECT", true},
		{roleRunner, "rd.option_exclusion_r1", "SELECT", true},
		{roleSettings, "rd.rule_param_r1", "SELECT", true},
		{roleSettings, "rd.option_exclusion_r1", "SELECT", true},
		{roleRoPages, "settings.rule_param", "SELECT", false},
		{roleRoPages, "settings.option_exclusion", "SELECT", false},
		{roleRunner, "settings.rule_param", "SELECT", false},
		{roleRunner, "settings.option_exclusion", "SELECT", false},
		{roleRoPages, "settings.rule_param", "INSERT", false},
		{roleRunner, "settings.option_exclusion", "INSERT", false},
		{roleAlerts, "settings.rule_param", "SELECT", false},
		{roleSettings, "settings.rule_param", "INSERT", true},
		{roleSettings, "settings.option_exclusion", "INSERT", true},
	} {
		var ok bool
		if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, $2, $3)`, c.role, c.object, c.priv).Scan(&ok); err != nil || ok != c.want {
			t.Errorf("%s %s on %s = %v, %v; want %v", c.role, c.priv, c.object, ok, err, c.want)
		}
	}
}

// TestSettingsDownKeepsRunnerReadOfOtherViews: the Down section of the settings migration takes back what that
// migration granted and nothing more: ot_runner and ot_ro_pages keep USAGE on rd and their other views.
func TestSettingsDownKeepsRunnerReadOfOtherViews(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newScratch(t, true)
	if err := migrateWith(ctx, s.openAs(t, testPassword), gooseUp, os.DirFS(migrationsDir), quietLog()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(migrationsDir, "*_settings.sql"))
	if err != nil || len(files) != 1 {
		t.Fatalf("settings migration = %v, %v", files, err)
	}
	raw, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	_, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("no Down section")
	}
	inDB, err := pgx.Connect(ctx, adminDSN(t, s.db))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = inDB.Close(ctx) })
	if _, err := inDB.Exec(ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	for _, role := range []string{roleRunner, roleRoPages} {
		var usage, view bool
		if err := inDB.QueryRow(ctx, `SELECT has_schema_privilege($1, 'rd', 'USAGE')`, role).Scan(&usage); err != nil || !usage {
			t.Errorf("%s lost USAGE on rd after settings down: %v, %v", role, usage, err)
		}
		if err := inDB.QueryRow(ctx, `SELECT has_table_privilege($1, 'rd.option_candidate_r1', 'SELECT')`, role).Scan(&view); err != nil {
			t.Fatal(err)
		}
		if role == roleRoPages && !view {
			t.Errorf("%s lost SELECT on rd.option_candidate_r1", role)
		}
	}
}
