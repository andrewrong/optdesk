//go:build pg

package basefactpg

import "testing"

const otherView = "rd.base_fact_other_r1"

// ny is the SQL of today's New York date.
const ny = `(now() AT TIME ZONE 'America/New_York')::date`

func TestOtherViewSeriesAge(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	for _, s := range []struct {
		id, freq string
		active   bool
		age      int // days since the newest point; -1 for no point
	}{
		{"D_OK", "daily", true, 3},
		{"D_OLD", "daily", true, 6}, // over 4 days
		{"W_OK", "weekly", true, 10},
		{"M_OLD", "monthly", true, 50}, // over 45 days
		{"Q_OK", "quarterly", true, 100},
		{"D_NONE", "daily", true, -1}, // no point at all
		{"NOFREQ", "", true, 400},     // no frequency: no limit to judge by, not counted
		{"OFF", "daily", false, 400},  // inactive: not counted
	} {
		pgExec(t, c, `INSERT INTO obs.series_definition VALUES ($1, $2, $3)`, s.id, s.freq, s.active)
		if s.age >= 0 {
			pgExec(t, c, `INSERT INTO rd.series_point_r1 VALUES ($1, `+ny+` - $2::int, 1.5)`, s.id, s.age)
		}
	}
	facts := viewFacts(t, name, otherView)
	wantCounts(t, facts, "rates/series_age/", 6, 3)
}

func TestOtherViewEarningsPast(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	earn := func(issuer, status string, report any, observedOff, attempt int) {
		t.Helper()
		pgExec(t, c, `INSERT INTO rd.earnings_date_r1 VALUES ($1, $2, `+ny+` + $3::int, `+ny+` + $4::int, $5)`,
			issuer, status, report, observedOff, attempt)
	}
	earn("XXX", "listed", -1, -1, 1)                                                            // listed and already past: bad
	earn("YYY", "listed", 5, -1, 1)                                                             // listed, ahead
	earn("ZZZ", "listed", -3, -4, 1)                                                            // an old check says past ...
	earn("ZZZ", "listed", 7, -1, 1)                                                             // ... the newest check says ahead: only the newest counts
	pgExec(t, c, `INSERT INTO rd.earnings_date_r1 VALUES ('WWW', 'no_value', NULL, `+ny+`, 1)`) // not listed: not in N
	facts := viewFacts(t, name, otherView)
	wantCounts(t, facts, "events/earnings_past/", 3, 1)
}

func TestOtherViewMacroExhausted(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	c := adminConn(t, name)
	for _, e := range []struct {
		kind string
		off  int
	}{
		{"cpi", -30}, {"cpi", 10}, // an event ahead: not exhausted
		{"fomc", -5}, // only past events: exhausted
		{"jobs", 2},
	} {
		pgExec(t, c, `INSERT INTO obs.macro_event VALUES ($1, `+ny+` + $2::int)`, e.kind, e.off)
	}
	facts := viewFacts(t, name, otherView)
	wantCounts(t, facts, "events/macro_exhausted/", 3, 1)
}
