// Package shared holds what the stores of one row of docs/design/modules.md write once and both import: it imports no
// sibling package of internal/store.
package shared

import (
	"strconv"
	"strings"
)

// DayArgCount is the number of arguments of UpsertDaySQL.
const DayArgCount = 26

// dayInsertColumns are the columns the statement writes, in argument order $1..$26. updated_at is the database's now().
var dayInsertColumns = []string{
	"instrument_id", "session_day", "underlying_id", "source", "received_at", "status", "currency",
	"bid", "ask", "bid_size", "ask_size", "last", "volume", "underlying_price", "underlying_ccy",
	"iv_bid_decimal", "iv_mid_decimal", "iv_ask_decimal", "iv_decimal", "delta", "gamma", "theta_per_calendar_day",
	"vega_per_vol_point", "open_interest", "oi_asof_date", "written_by",
}

// dayQuoteColumns are the quote columns: a new NULL never replaces a stored value.
var dayQuoteColumns = []string{
	"bid", "ask", "bid_size", "ask_size", "last", "volume", "underlying_price", "iv_bid_decimal", "iv_mid_decimal",
	"iv_ask_decimal", "iv_decimal", "delta", "gamma", "theta_per_calendar_day", "vega_per_vol_point", "open_interest",
	"oi_asof_date",
}

// dayCompared are the columns a rewrite is judged by besides the quote columns (not the key, not received_at).
var dayCompared = []string{"underlying_id", "source", "status", "currency", "underlying_ccy"}

// UpsertDaySQL writes one row of obs.option_day (docs/design/option-captures.md Decision 6). Arguments, in order:
// instrument_id, session_day, underlying_id, source, received_at, status, currency, bid, ask, bid_size, ask_size,
// last, volume, underlying_price, underlying_ccy, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal, delta,
// gamma, theta_per_calendar_day, vega_per_vol_point, open_interest, oi_asof_date, written_by (DayArgCount in all).
// A quote column that the new row lacks keeps the stored value; a row older than the stored one changes nothing, and a
// row that would leave every compared column as it is is not rewritten.
var UpsertDaySQL = buildUpsertDaySQL()

func buildUpsertDaySQL() string {
	var sets, newVals, oldVals []string
	for _, c := range dayCompared {
		sets = append(sets, c+" = EXCLUDED."+c)
		newVals = append(newVals, "EXCLUDED."+c)
		oldVals = append(oldVals, "option_day."+c)
	}
	for _, c := range dayQuoteColumns {
		co := "COALESCE(EXCLUDED." + c + ", option_day." + c + ")"
		sets = append(sets, c+" = "+co)
		newVals = append(newVals, co)
		oldVals = append(oldVals, "option_day."+c)
	}
	sets = append(sets, "received_at = EXCLUDED.received_at", "updated_at = now()", "written_by = EXCLUDED.written_by")
	ph := make([]string, DayArgCount)
	for i := range ph {
		ph[i] = "$" + strconv.Itoa(i+1)
	}
	return "INSERT INTO obs.option_day (" + strings.Join(dayInsertColumns, ", ") + ") VALUES (" +
		strings.Join(ph, ", ") + ")\nON CONFLICT (instrument_id, session_day) DO UPDATE SET " +
		strings.Join(sets, ",\n") + "\n WHERE EXCLUDED.received_at >= option_day.received_at AND (" +
		strings.Join(oldVals, ", ") + ") IS DISTINCT FROM (" + strings.Join(newVals, ", ") + ")"
}
