//go:build pg

package reading

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

func openUpserter(t *testing.T, name string) (store.ReadingStore, readingkey.ReadingUpserter) {
	t.Helper()
	st, err := Open(context.Background(), connURL(t, name, "ot_w_readings"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	up, ok := st.(readingkey.ReadingUpserter)
	if !ok {
		t.Fatal("the reading store has no UpsertReadings")
	}
	return st, up
}

func count1(t *testing.T, c *pgx.Conn, sql string) int {
	t.Helper()
	var n int
	if err := c.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func TestUpsertReplacesDifferingAndSkipsEqualPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, up := openUpserter(t, name)

	one := rec(1, "ok")
	if c, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{one}); err != nil || c != (readingkey.WriteCount{Written: 1}) {
		t.Fatalf("first = %+v, %v", c, err)
	}
	// An equal reading, even from another run, is skipped.
	same := rec(1, "ok")
	same.JobRunID = "22222222-2222-2222-2222-222222222222"
	if c, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{same}); err != nil || c != (readingkey.WriteCount{Skipped: 1}) {
		t.Fatalf("equal = %+v, %v", c, err)
	}
	// A differing one replaces it with its children, rev as sent.
	two := rec(5, "ok")
	two.Value = strPtr("0.750000000000")
	two.Inputs[0].Keys = []string{"only"}
	two.Parameters[0].Param = "window2"
	if c, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{two}); err != nil || c != (readingkey.WriteCount{Written: 1}) {
		t.Fatalf("differing = %+v, %v", c, err)
	}
	got, err := st.LatestReadings(ctx, one.Name, one.SubjectKind, one.Subject, "2026-11-01", "2026-11-30")
	if err != nil || len(got) != 1 || got[0].Rev != 5 || *got[0].Value != "0.750000000000" {
		t.Fatalf("latest = %+v, %v", got, err)
	}
	if len(got[0].Inputs) != 1 || len(got[0].Inputs[0].Keys) != 1 || got[0].Inputs[0].Keys[0] != "only" ||
		len(got[0].Parameters) != 1 || got[0].Parameters[0].Param != "window2" {
		t.Errorf("children = %+v %+v", got[0].Inputs, got[0].Parameters)
	}
	admin := pgxAs(t, name, "")
	for sql, want := range map[string]int{
		`SELECT count(*) FROM readings.reading`:                                1,
		`SELECT count(*) FROM readings.reading_input_key`:                      1,
		`SELECT count(*) FROM readings.reading_parameter WHERE rev = 5`:        1,
		`SELECT count(*) FROM readings.reading_parameter WHERE rev <> 5`:       0,
		`SELECT count(*) FROM readings.reading_input_key WHERE rev <> 5`:       0,
		`SELECT count(*) FROM readings.reading WHERE value = 0.75 AND rev = 5`: 1,
	} {
		if n := count1(t, admin, sql); n != want {
			t.Errorf("%s = %d, want %d", sql, n, want)
		}
	}
}

func TestUpsertMissingNeverReplacesOkPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, up := openUpserter(t, name)
	latest := func() contracts.ReadingRecord {
		t.Helper()
		got, err := st.LatestReadings(ctx, "market.ivp_own", "market", "VIXCLS", "2026-11-06", "2026-11-06")
		if err != nil || len(got) != 1 {
			t.Fatalf("latest = %+v, %v", got, err)
		}
		return got[0]
	}
	miss := rec(1, "missing_input")
	miss.Value = nil
	if c, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{miss}); err != nil || c.Written != 1 {
		t.Fatalf("missing = %+v, %v", c, err)
	}
	if c, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{rec(2, "ok")}); err != nil || c.Written != 1 {
		t.Fatalf("ok over missing = %+v, %v", c, err)
	}
	if r := latest(); string(r.State) != "ok" || r.Rev != 2 {
		t.Fatalf("after ok: %+v", r)
	}
	miss3 := rec(3, "missing_input")
	miss3.Value = nil
	if c, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{miss3}); err != nil || c != (readingkey.WriteCount{Skipped: 1}) {
		t.Fatalf("missing over ok = %+v, %v", c, err)
	}
	if r := latest(); string(r.State) != "ok" || r.Rev != 2 || len(r.Inputs) != 1 {
		t.Errorf("after missing: %+v", r)
	}
}

func TestOldInsertAddsNoRowForStoredKeyPG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	st, up := openUpserter(t, name)
	if _, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{rec(1, "ok")}); err != nil {
		t.Fatal(err)
	}
	other := rec(2, "ok")
	other.Value = strPtr("0.9")
	if n, err := st.InsertReadings(ctx, []contracts.ReadingRecord{other}); err != nil || n != 0 {
		t.Fatalf("insert on a stored key = %d, %v", n, err)
	}
	if n := count1(t, pgxAs(t, name, ""), `SELECT count(*) FROM readings.reading WHERE rev = 1`); n != 1 {
		t.Errorf("stored reading changed: %d rows at rev 1", n)
	}
}

// insertRaw writes one reading and its children as the owner, at the old keys (migration 0013 only).
func insertRaw(t *testing.T, c *pgx.Conn, subject string, rev int, state string) {
	t.Helper()
	ctx := context.Background()
	value := "0.5"
	if state != "ok" {
		value = ""
	}
	if _, err := c.Exec(ctx, `INSERT INTO readings.reading (name, version, subject_kind, subject, day, rev, value, unit, currency, state,
		origin, cutoff, job_run_id, recorded_at, rows_sha256) VALUES ('n', 1, 'market', $1, '2026-11-06', $2, NULLIF($3, '')::numeric, 'decimal', '', $4,
		'o', 'c', '11111111-1111-1111-1111-111111111111', now(), repeat('a', 64))`, subject, rev, value, state); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, `INSERT INTO readings.reading_input VALUES ('n', 1, 'market', $1, '2026-11-06', $2, 0, 't', 'h', 'c')`, subject, rev); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, `INSERT INTO readings.reading_input_key VALUES ('n', 1, 'market', $1, '2026-11-06', $2, 0, 0, 'k')`, subject, rev); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Exec(ctx, `INSERT INTO readings.reading_parameter VALUES ('n', 1, 'market', $1, '2026-11-06', $2, 'p', 1, NULL)`, subject, rev); err != nil {
		t.Fatal(err)
	}
}

func pageRows(t *testing.T, c *pgx.Conn, sql string) []string {
	t.Helper()
	rows, err := c.Query(context.Background(), sql)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, fmt.Sprint(vals...))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func TestMigration0045KeepsTheWinnerOfTheOldRulePG(t *testing.T) {
	t.Parallel()
	name := newDatabaseWith(t, "0013_readings.sql", "0016_reading_parameter_view.sql")
	admin := pgxAs(t, name, "")
	// A: ok at rev 1 below missing_input at rev 2 and 3 (winner rev 1). B: missing, then ok at rev 2 and 3 (winner 3).
	// C: one missing_input only (winner rev 1).
	for _, r := range []struct {
		s     string
		rev   int
		state string
	}{{"A", 1, "ok"}, {"A", 2, "missing_input"}, {"A", 3, "missing_input"}, {"B", 1, "missing_input"}, {"B", 2, "ok"},
		{"B", 3, "ok"}, {"C", 1, "missing_input"}} {
		insertRaw(t, admin, r.s, r.rev, r.state)
	}
	pages := pgxAs(t, name, "ot_ro_pages")
	const viewSQL = `SELECT * FROM rd.reading_r1`
	const paramSQL = `SELECT subject || rev::text FROM rd.reading_parameter_r1`
	viewBefore := pageRows(t, pages, viewSQL)
	if len(viewBefore) != 3 {
		t.Fatalf("view before has %d rows", len(viewBefore))
	}
	paramBefore := pageRows(t, pages, paramSQL)

	applyUp(t, admin, "0045_reading_key.sql")

	if after := pageRows(t, pages, viewSQL); !slices.Equal(after, viewBefore) {
		t.Errorf("rd.reading_r1 changed:\nbefore %v\nafter  %v", viewBefore, after)
	}
	// Per key only the winner stays, with its children and no other revision's.
	want := []string{"A1", "B3", "C1"}
	for _, tbl := range []string{"reading", "reading_input", "reading_input_key", "reading_parameter"} {
		if got := pageRows(t, admin, `SELECT subject || rev::text FROM readings.`+tbl); !slices.Equal(got, want) {
			t.Errorf("%s left = %v, want %v", tbl, got, want)
		}
	}
	// The parameter view keeps the rows of the winners it showed before; the losers' rows went with their readings.
	var winners []string
	for _, r := range paramBefore {
		if slices.Contains(want, r) {
			winners = append(winners, r)
		}
	}
	if after := pageRows(t, pages, paramSQL); !slices.Equal(after, winners) || len(winners) != 3 {
		t.Errorf("rd.reading_parameter_r1 after = %v, want the winners' rows %v (before %v)", after, winners, paramBefore)
	}
}

func TestMigration0045PrimaryKeysHaveNoRevPG(t *testing.T) {
	t.Parallel()
	admin := pgxAs(t, newDatabase(t), "")
	rows, err := admin.Query(context.Background(), `SELECT c.conrelid::regclass::text, array_agg(a.attname::text ORDER BY a.attname)
		FROM pg_constraint c JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
		WHERE c.contype = 'p' AND c.connamespace = 'readings'::regnamespace GROUP BY c.oid, c.conrelid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var tbl string
		var cols []string
		if err := rows.Scan(&tbl, &cols); err != nil {
			t.Fatal(err)
		}
		n++
		if slices.Contains(cols, "rev") {
			t.Errorf("primary key of %s has rev: %v", tbl, cols)
		}
	}
	if n != 4 {
		t.Errorf("%d primary keys, want 4", n)
	}
}

func TestGrantsAfter0045PG(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	_, up := openUpserter(t, name)
	if _, err := up.UpsertReadings(ctx, []contracts.ReadingRecord{rec(1, "ok")}); err != nil {
		t.Fatal(err)
	}
	w := pgxAs(t, name, "ot_w_readings")
	// Denied: DELETE from readings.reading, UPDATE of a key column, TRUNCATE.
	for _, sql := range []string{`DELETE FROM readings.reading`, `UPDATE readings.reading SET name = 'x'`,
		`UPDATE readings.reading SET day = day`, `TRUNCATE readings.reading_input`} {
		if _, err := w.Exec(ctx, sql); err == nil {
			t.Errorf("ot_w_readings may run %s (denied)", sql)
		}
	}
	// Allowed: UPDATE of a non-key column and DELETE of a child.
	for _, sql := range []string{`UPDATE readings.reading SET rev = rev`, `DELETE FROM readings.reading_parameter`} {
		if _, err := w.Exec(ctx, sql); err != nil {
			t.Errorf("ot_w_readings cannot run %s: %v", sql, err)
		}
	}
	pages := pgxAs(t, name, "ot_ro_pages")
	for _, sql := range []string{`SELECT 1 FROM rd.reading_r1`, `SELECT 1 FROM rd.reading_parameter_r1`} {
		if _, err := pages.Exec(ctx, sql); err != nil {
			t.Errorf("ot_ro_pages cannot run %s: %v", sql, err)
		}
	}
}

// mustUpsert upserts the rows through the optional method and fails unless the counts are as wanted.
func mustUpsert(t *testing.T, st store.ReadingStore, want readingkey.WriteCount, rows ...contracts.ReadingRecord) {
	t.Helper()
	up, ok := st.(readingkey.ReadingUpserter)
	if !ok {
		t.Fatal("the reading store has no UpsertReadings")
	}
	if got, err := up.UpsertReadings(context.Background(), rows); err != nil || got != want {
		t.Fatalf("upsert = %+v, %v; want %+v", got, err, want)
	}
}
