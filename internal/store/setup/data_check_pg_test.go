//go:build pg

package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDataCheckGrants: ot_w_datacheck inserts and reads the three tables and updates or deletes none; ot_ro_pages reads
// the two views and no table; ot_w_datacheck reads the eight input views.
func TestDataCheckGrants(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	if _, err := c.Exec(ctx, `INSERT INTO datacheck.run (check_day, started_at, finished_at, rules_total, rules_unknown)
		VALUES ('2026-11-02', now(), now(), 3, 0);
		INSERT INTO datacheck.finding (run_id, rule, dataset, day, severity, evidence, is_new)
		VALUES (1, 'capture_gap', 'obs.price_bar', '2026-11-02', 'warn', '[]', true);
		INSERT INTO datacheck.notice (run_id, noticed_at) VALUES (1, now())`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	const runIns = `INSERT INTO datacheck.run (check_day, started_at, finished_at, rules_total, rules_unknown) VALUES ('2026-11-03', now(), now(), 1, 0)`
	for name, q := range map[string]string{
		"insert run": runIns,
		"insert finding": `INSERT INTO datacheck.finding (run_id, rule, dataset, day, severity, evidence, is_new)
			VALUES (1, 'plan_absent', 'obs.price_bar', '2026-11-02', 'error', '[]', false)`,
		"insert notice": runIns + `; INSERT INTO datacheck.notice (run_id, noticed_at) VALUES (currval(pg_get_serial_sequence('datacheck.run', 'run_id')), now())`,
	} {
		if err := asRole(ctx, t, c, "ot_w_datacheck", q); err != nil {
			t.Errorf("%s as ot_w_datacheck: %v", name, err)
		}
	}
	for _, tbl := range []string{"run", "finding", "notice"} {
		if err := asRole(ctx, t, c, "ot_w_datacheck", `SELECT * FROM datacheck.`+tbl); err != nil {
			t.Errorf("select %s as ot_w_datacheck: %v", tbl, err)
		}
		for name, q := range map[string]string{
			"update":   `UPDATE datacheck.` + tbl + ` SET run_id = run_id`,
			"delete":   `DELETE FROM datacheck.` + tbl,
			"truncate": `TRUNCATE datacheck.` + tbl,
		} {
			if err := asRole(ctx, t, c, "ot_w_datacheck", q); err == nil {
				t.Errorf("%s %s as ot_w_datacheck succeeded", name, tbl)
			}
		}
		if err := asRole(ctx, t, c, roleRoPages, `SELECT * FROM datacheck.`+tbl); err == nil {
			t.Errorf("ot_ro_pages read table %s", tbl)
		}
	}
	for _, v := range []string{"data_check_run_r1", "data_finding_r1"} {
		if err := asRole(ctx, t, c, roleRoPages, `SELECT * FROM rd.`+v); err != nil {
			t.Errorf("ot_ro_pages select rd.%s: %v", v, err)
		}
	}
	for _, v := range []string{"coverage_day_r1", "session_day_r1", "reading_r1", "option_open_r1", "option_candidate_r1",
		"rule_param_r1", "option_exclusion_r1", "job_gap_r1"} {
		if err := asRole(ctx, t, c, "ot_w_datacheck", `SELECT * FROM rd.`+v); err != nil {
			t.Errorf("ot_w_datacheck select rd.%s: %v", v, err)
		}
	}
}

// TestMigrate0073Down: after the Down section the schema, the tables and the two views are gone, and the grants with them.
func TestMigrate0073Down(t *testing.T) {
	t.Parallel()
	ctx, c := sessionDayDB(t)
	raw, err := os.ReadFile(filepath.Join(migrationsDir, "0073_data_check.sql"))
	if err != nil {
		t.Fatal(err)
	}
	_, down, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("no Down section")
	}
	if _, err := c.Exec(ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	count := func(q string) int {
		t.Helper()
		var n int
		if err := c.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(`SELECT count(*) FROM pg_namespace WHERE nspname = 'datacheck'`); n != 0 {
		t.Errorf("schema datacheck remains")
	}
	if n := count(`SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'rd' AND c.relname IN ('data_check_run_r1', 'data_finding_r1')`); n != 0 {
		t.Errorf("%d views remain", n)
	}
	var usage bool
	if err := c.QueryRow(ctx, `SELECT has_schema_privilege('ot_ro_pages', 'rd', 'USAGE')`).Scan(&usage); err != nil || !usage {
		t.Errorf("ot_ro_pages lost USAGE on rd: %v, %v", usage, err)
	}
	var sel bool
	if err := c.QueryRow(ctx, `SELECT has_table_privilege('ot_w_datacheck', 'rd.job_gap_r1', 'SELECT')`).Scan(&sel); err != nil || sel {
		t.Errorf("ot_w_datacheck still reads rd.job_gap_r1: %v, %v", sel, err)
	}
}
