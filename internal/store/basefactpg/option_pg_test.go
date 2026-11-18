//go:build pg

package basefactpg

import "testing"

const optionView = "rd.base_fact_option_r1"

// optRow inserts one row of the option table (rd.quote_snapshot_r1, rd.option_eod_r1 or rd.option_open_r1) of the
// instrument id: today's New York date, USD, the status, minute minutes after a fixed instant, the market data type
// and the bid and ask ("" is null).
func optRow(t *testing.T, name, table string, id, status, minute, mdt int, bid, ask string) {
	t.Helper()
	conn := adminConn(t, name)
	nul := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	pgExec(t, conn, `INSERT INTO `+table+` (session_day, instrument_id, currency, status, received_at, market_data_type, bid, ask)
		VALUES ((now() AT TIME ZONE 'America/New_York')::date, $1, 'USD', $2,
		        timestamptz '2026-01-01 12:00:00+00' + make_interval(mins => $3), $4, $5::numeric, $6::numeric)`,
		id, status, minute, mdt, nul(bid), nul(ask))
}

func quote(t *testing.T, name string, id, status, minute, mdt int, bid, ask string) {
	t.Helper()
	optRow(t, name, "rd.quote_snapshot_r1", id, status, minute, mdt, bid, ask)
}

func TestOptionViewCountsOneRowPerContract(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	// Contract 1 was captured twice, contract 2 once: two contracts, not three rows.
	quote(t, name, 1, 1, 1, 1, "1", "2")
	quote(t, name, 1, 1, 2, 1, "1", "2")
	quote(t, name, 2, 1, 1, 1, "1", "2")
	facts := viewFacts(t, name, optionView)
	wantCounts(t, facts, "quotes/rows/USD", 2, 0)
	wantCounts(t, facts, "quotes/crossed/", 2, 0)
	wantCounts(t, facts, "quotes/non_live/", 2, 0)
	wantCounts(t, facts, "quotes/empty/", 2, 0)
	wantCounts(t, facts, "quotes/errors/", 2, 0)
}

func TestOptionViewPicksStatusOne(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	// A status-2 row newer than the status-1 row does not replace it: the crossed bid and ask are counted and the
	// contract is not empty.
	quote(t, name, 1, 1, 1, 1, "3", "2")
	quote(t, name, 1, 2, 5, 1, "", "")
	facts := viewFacts(t, name, optionView)
	wantCounts(t, facts, "quotes/crossed/", 1, 1)
	wantCounts(t, facts, "quotes/empty/", 1, 0)
	wantCounts(t, facts, "quotes/not_judged/", 1, 0)
}

func TestOptionViewEmptyOnlyWithoutStatusOne(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	quote(t, name, 1, 1, 1, 1, "1", "2") // a status-1 row: never empty
	quote(t, name, 1, 2, 5, 1, "", "")
	quote(t, name, 2, 2, 1, 1, "", "") // no quote only: empty
	quote(t, name, 3, 5, 1, 1, "", "") // error only: errors
	quote(t, name, 4, 3, 1, 1, "", "") // not listed: not judged
	quote(t, name, 5, 4, 1, 1, "", "") // stale: not judged
	quote(t, name, 6, 5, 1, 1, "", "") // error, then no quote: the newest row says empty
	quote(t, name, 6, 2, 2, 1, "", "")
	quote(t, name, 7, 2, 1, 1, "", "") // no quote, then error: the newest row says error
	quote(t, name, 7, 5, 2, 1, "", "")
	facts := viewFacts(t, name, optionView)
	wantCounts(t, facts, "quotes/empty/", 7, 2)
	wantCounts(t, facts, "quotes/errors/", 7, 2)
	wantCounts(t, facts, "quotes/not_judged/", 7, 2)
}

func TestOptionViewNonLiveShare(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	quote(t, name, 1, 1, 1, 3, "1", "2") // older row of contract 1 is delayed, the newer one is live
	quote(t, name, 1, 1, 2, 1, "1", "2")
	quote(t, name, 2, 1, 1, 1, "1", "2")
	quote(t, name, 3, 1, 1, 3, "1", "2") // delayed
	quote(t, name, 4, 1, 1, 2, "1", "2") // frozen
	facts := viewFacts(t, name, optionView)
	wantCounts(t, facts, "quotes/non_live/", 4, 2)
}

func TestOptionViewCrossedShare(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ table, dataset string }{
		{"rd.quote_snapshot_r1", "quotes"}, {"rd.option_eod_r1", "chains"}, {"rd.option_open_r1", "open"},
	} {
		t.Run(tc.dataset, func(t *testing.T) {
			t.Parallel()
			name := newViewsDatabase(t)
			optRow(t, name, tc.table, 1, 1, 1, 1, "5", "4") // crossed
			optRow(t, name, tc.table, 2, 1, 1, 1, "4", "5") // normal
			optRow(t, name, tc.table, 3, 1, 1, 1, "0", "0") // zero is no quote, not crossed
			optRow(t, name, tc.table, 4, 1, 1, 1, "", "3")  // one side missing
			optRow(t, name, tc.table, 5, 1, 1, 1, "3", "3") // locked is not crossed
			facts := viewFacts(t, name, optionView)
			wantCounts(t, facts, tc.dataset+"/crossed/", 5, 1)
		})
	}
}
