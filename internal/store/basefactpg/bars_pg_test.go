//go:build pg

package basefactpg

import "testing"

const barsView = "rd.base_fact_bars_r1"

// bar inserts one bar of instrument id, off days from today (negative: in the past), with the four prices.
func bar(t *testing.T, name string, id int, off int, basis, o, h, l, c string) {
	t.Helper()
	conn := adminConn(t, name)
	pgExec(t, conn, `INSERT INTO rd.price_bar_r1 VALUES ($1, (now() AT TIME ZONE 'America/New_York')::date + $2::int, $3, $4, $5, $6, $7)`,
		id, off, basis, o, h, l, c)
}

func instruments(t *testing.T, name string, rows ...string) {
	t.Helper()
	conn := adminConn(t, name)
	for i := 0; i < len(rows); i += 3 {
		pgExec(t, conn, `INSERT INTO master.instrument VALUES ($1::bigint, $2, $3)`, rows[i], rows[i+1], rows[i+2])
	}
}

func TestBarsViewOhlcOrder(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	instruments(t, name, "1", "AAA", "USD")
	bar(t, name, 1, -2, "trades", "10", "12", "9", "11")      // good
	bar(t, name, 1, -3, "trades", "10", "8", "9", "9")        // high below low
	bar(t, name, 1, -4, "trades", "10", "10.5", "9", "11")    // high below close
	bar(t, name, 1, -5, "trades", "9", "12", "9.5", "11")     // low above open
	bar(t, name, 1, -6, "adjusted_last", "10", "8", "9", "9") // other basis: not judged
	bar(t, name, 1, -60, "trades", "10", "8", "9", "9")       // before the window: not judged
	facts := viewFacts(t, name, barsView)
	wantCounts(t, facts, "bars/ohlc_order/", 4, 3)
}

func TestBarsViewJumpAcrossStoredDays(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	instruments(t, name, "1", "AAA", "USD", "2", "BBB", "USD")
	// The jump is against the previous stored close, three days back here, not against the previous calendar day.
	bar(t, name, 1, -10, "trades", "100", "100", "100", "100")
	bar(t, name, 1, -7, "trades", "150", "150", "150", "150") // +50 percent: a jump
	bar(t, name, 1, -5, "trades", "151", "151", "151", "151")
	// Another instrument is never compared with the first: its 30 percent is under the limit.
	bar(t, name, 2, -8, "trades", "100", "100", "100", "100")
	bar(t, name, 2, -6, "trades", "130", "130", "130", "130")
	// A bar of the other basis is not in the series.
	bar(t, name, 1, -6, "adjusted_last", "500", "500", "500", "500")
	facts := viewFacts(t, name, barsView)
	wantCounts(t, facts, "bars/bar_jump/", 3, 1)
}

func TestBarsViewNewestPerCurrency(t *testing.T) {
	t.Parallel()
	name := newViewsDatabase(t)
	instruments(t, name, "1", "AAA", "USD", "2", "BBB", "JPY")
	bar(t, name, 1, -5, "trades", "10", "12", "9", "11")
	bar(t, name, 1, -3, "trades", "10", "12", "9", "11.5")
	bar(t, name, 2, -4, "trades", "2000", "2100", "1900", "2000")
	facts := viewFacts(t, name, barsView)
	conn := adminConn(t, name)
	usd, jpy := facts["bars/newest/USD"], facts["bars/newest/JPY"]
	if want := nyDate(t, conn, -3); usd.Day != want || usd.Value != "AAA close 11.5" {
		t.Errorf("USD newest = %q %q, want %q %q", usd.Day, usd.Value, want, "AAA close 11.5")
	}
	if want := nyDate(t, conn, -4); jpy.Day != want || jpy.Value != "BBB close 2000" {
		t.Errorf("JPY newest = %q %q, want %q %q", jpy.Day, jpy.Value, want, "BBB close 2000")
	}
	wantCounts(t, facts, "bars/rows/USD", 2, 0)
	wantCounts(t, facts, "bars/rows/JPY", 1, 0)
}
