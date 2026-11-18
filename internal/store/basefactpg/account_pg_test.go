//go:build pg

package basefactpg

import "testing"

const accountView = "rd.base_fact_account_r1"

// capture inserts a capture of alias; minute orders the captures of one alias.
func capture(t *testing.T, name, id, alias string, minute int) {
	t.Helper()
	pgExec(t, adminConn(t, name), `INSERT INTO rd.account_capture_r1
		VALUES ($2, $1, timestamptz '2026-01-01 12:00:00+00' + make_interval(mins => $3), `+ny+`)`, id, alias, minute)
}

func TestAccountViewNoNetLiq(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	capture(t, name, "a1-old", "a1", 1)
	capture(t, name, "a1-new", "a1", 2) // the newest capture of a1 has no NetLiquidation: bad
	capture(t, name, "a2-1", "a2", 1)
	capture(t, name, "a3-1", "a3", 1) // only another tag: bad
	capture(t, name, "a4-old", "a4", 1)
	capture(t, name, "a4-new", "a4", 2) // the old one lacks it, the newest has it: fine
	for _, v := range [][2]string{
		{"a1-old", "NetLiquidation"}, {"a2-1", "NetLiquidation"}, {"a3-1", "TotalCashValue"}, {"a4-new", "NetLiquidation"},
	} {
		pgExec(t, c, `INSERT INTO rd.account_value_r1 VALUES ($1, $2)`, v[0], v[1])
	}
	facts := viewFacts(t, name, accountView)
	wantCounts(t, facts, "account/no_netliq/", 4, 2)
}

func TestAccountViewPositionExpired(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	capture(t, name, "old", "a1", 1)
	capture(t, name, "new", "a1", 2)
	pos := func(capture, secType, expiry string) {
		t.Helper()
		pgExec(t, c, `INSERT INTO rd.account_position_r1 VALUES ($1, $2, $3)`, capture, secType, expiry)
	}
	date := func(off int) string { return nyDate(t, c, off) }
	pos("new", "option", date(-1)) // expired: bad
	pos("new", "option", date(10))
	pos("new", "stock", "")         // not an option
	pos("old", "option", date(-20)) // a capture that is not the newest
	facts := viewFacts(t, name, accountView)
	wantCounts(t, facts, "account/position_expired/", 2, 1)

	// An expiry that is not an ISO date: the fact is left out, the check is unknown.
	pos("new", "option", "20261120")
	facts = viewFacts(t, name, accountView)
	if f, ok := facts["account/position_expired/"]; ok {
		t.Errorf("position_expired with a non-ISO expiry = %+v, want no row", f)
	}
}

func TestAccountViewFlexSectionAbsentIsNote(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	st := func(fetch, alias, section, state string, minute int) {
		t.Helper()
		pgExec(t, c, `INSERT INTO rd.flex_section_status_r1
			VALUES ($1, $2, $3, $4, timestamptz '2026-01-01 12:00:00+00' + make_interval(mins => $5))`,
			fetch, alias, section, state, minute)
	}
	// Alias A: the older fetch has a section absent, the newest has OpenPosition absent only.
	st("f1", "A", "EquitySummaryByReportDateInBase", "absent", 1)
	st("f2", "A", "OpenPosition", "absent", 2)
	st("f2", "A", "EquitySummaryByReportDateInBase", "present", 2)
	st("f2", "A", "ConversionRate", "present", 2)
	// Alias B: all present, and a section with no status row is not absent.
	st("g1", "B", "OpenPosition", "present", 1)
	st("g1", "B", "EquitySummaryByReportDateInBase", "present", 1)
	facts := viewFacts(t, name, accountView)
	// Six wanted sections of two aliases, one absent in a newest fetch. The trade rows are empty: the note is
	// there all the same and the rows of the flex dataset say nothing of it.
	wantCounts(t, facts, "flex/flex_section_absent/", 6, 1)
	wantCounts(t, facts, "flex/rows/", 0, 0)
}
