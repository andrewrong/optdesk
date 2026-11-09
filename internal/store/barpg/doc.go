// Package barpg is the bar store over pgx for the DSN line OT_PG_DSN_BAR (role ot_w_bar): the writer of obs.price_bar
// and obs.fx_rate, insert-only, and the reader of the views rd.price_bar_r1 and rd.fx_rate_r1
// (docs/design/market-bars.md Decisions 4 and 8). It imports contracts and not internal/store/bar: its test asserts
// that *Store satisfies bar.BarStore. No statement of this package changes or removes a row.
package barpg
