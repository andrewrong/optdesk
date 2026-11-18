//go:build pg

package snapshot

import (
	"context"
	"testing"
)

// conidFixtureDatabase is newDatabase with the master tables the view reads and migration 0055 applied. The stub of
// master.instrument gains instrument_type; the role ot_owner (owner of the view) gets the rights master gives it.
func conidFixtureDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	owner := connectAs(t, name, "")
	ctx := context.Background()
	for _, s := range []string{
		`ALTER TABLE master.instrument ADD COLUMN instrument_type text NOT NULL DEFAULT 'option'`,
		`CREATE TABLE master.instrument_identifier (instrument_id bigint NOT NULL, scheme text NOT NULL, value text NOT NULL, ` +
			`valid_from timestamptz NOT NULL, valid_to timestamptz)`,
		`GRANT USAGE ON SCHEMA master TO ot_owner`,
		`GRANT SELECT ON master.instrument, master.instrument_identifier TO ot_owner, ot_ro_pages`,
		`UPDATE master.instrument SET instrument_type = 'stock' WHERE instrument_id = 900003`,
		// 900001: option, conid 5001 (current). 900002: option whose conid 4999 is closed and 5002 current.
		// 900003: a stock with a conid.
		`INSERT INTO master.instrument_identifier VALUES ` +
			`(900001, 'ibkr_conid', '5001', now(), NULL), ` +
			`(900002, 'ibkr_conid', '4999', now() - interval '2 days', now() - interval '1 day'), ` +
			`(900002, 'ibkr_conid', '5002', now() - interval '1 day', NULL), ` +
			`(900002, 'osi', 'OSI 1', now(), NULL), ` +
			`(900003, 'ibkr_conid', '5003', now(), NULL)`,
	} {
		if _, err := owner.Exec(ctx, s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
	applyMigration(t, connURL(t, name, ""), "0055_option_conid_view.sql")
	return name
}

func TestOptionConidViewMapsIdentifier(t *testing.T) {
	t.Parallel()
	name := conidFixtureDatabase(t)
	p := connectAs(t, name, "ot_ro_pages")
	rows, err := p.Query(context.Background(), `SELECT instrument_id, conid FROM rd.option_conid_r1 ORDER BY conid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[int64]int64{}
	for rows.Next() {
		var id, conid int64
		if err := rows.Scan(&id, &conid); err != nil {
			t.Fatal(err)
		}
		got[conid] = id
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[5001] != 900001 || got[5002] != 900002 {
		t.Fatalf("view %v, want conid 5001 -> 900001 and 5002 -> 900002 only (a closed row and a stock are out)", got)
	}
	// The reader over the view, as the role, maps a conid to its row of obs.option_day.
	r, err := OpenConidDayReader(context.Background(), connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	days, err := r.Days(context.Background(), []int64{5001, 4999, 5003}, "2026-11-01", "2026-11-30")
	if err != nil || len(days) != 0 {
		t.Fatalf("days %+v, err %v, want none and no error", days, err)
	}
}

func TestOptionConidViewHidesMaster(t *testing.T) {
	t.Parallel()
	name := conidFixtureDatabase(t)
	p := connectAs(t, name, "ot_ro_pages")
	var usage bool
	if err := p.QueryRow(context.Background(), `SELECT has_schema_privilege('ot_ro_pages', 'master', 'USAGE')`).Scan(&usage); err != nil {
		t.Fatal(err)
	}
	if usage {
		t.Fatal("ot_ro_pages has USAGE on schema master")
	}
	// ot_runner gets no grant on the view.
	var runner bool
	if err := p.QueryRow(context.Background(), `SELECT has_table_privilege('ot_runner', 'rd.option_conid_r1', 'SELECT')`).Scan(&runner); err != nil {
		t.Fatal(err)
	}
	if runner {
		t.Fatal("ot_runner can read the view")
	}
}
