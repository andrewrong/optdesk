// Package bar declares the store of daily price bars and daily FX rates
// (obs.price_bar r1 and obs.fx_rate r1). It holds declarations only; the
// implementation is internal/store/barpg.
package bar

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// DSNEnvBar names the line that holds the DSN of the role ot_w_bar.
const DSNEnvBar = "OT_PG_DSN_BAR"

// PriceBarWriter inserts price bars. The tables are insert-only: a row that
// exists is never changed, and a restated day is a new row under the next
// AdjustEpoch.
type PriceBarWriter interface {
	// InsertPriceBars writes the rows in one transaction (all rows or none) and
	// returns the number inserted; a key that exists is left as it is and adds
	// none. An empty input makes no database call.
	InsertPriceBars(ctx context.Context, rows []contracts.PriceBar) (inserted int, err error)
}

// PriceBarReader reads price bars.
type PriceBarReader interface {
	// PriceBars returns the winner rows of one instrument and basis ("trades" or
	// "adjusted_last") with session day between from and to, both ends inclusive:
	// for each day the row of the highest AdjustEpoch of the pair, ascending by
	// day. Rows carry AdjustEpoch. No rows gives an empty slice.
	PriceBars(ctx context.Context, instrumentID int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error)
}

// FxRateWriter inserts daily FX rates, insert-only like PriceBarWriter.
type FxRateWriter interface {
	// InsertFxRates writes the rows in one transaction and returns the number
	// inserted; a key that exists is left as it is and adds none.
	InsertFxRates(ctx context.Context, rows []contracts.FxRate) (inserted int, err error)
}

// FxRateReader reads daily FX rates.
type FxRateReader interface {
	// FxRates returns the rates of pair (six capitals, for example USDJPY) with
	// rate day between from and to, both ends inclusive, ascending by day.
	FxRates(ctx context.Context, pair string, from, to contracts.Day) ([]contracts.FxRate, error)
	// LatestFxRate returns the rate of the newest rate day on or before asOf.
	// There is no fallback: when no such row exists the answer is the zero rate
	// with found false and no error, never a default or a later day.
	LatestFxRate(ctx context.Context, pair string, asOf contracts.Day) (rate contracts.FxRate, found bool, err error)
}

// BarStore is the writers and the readers of both datasets, with Close.
type BarStore interface {
	PriceBarWriter
	PriceBarReader
	FxRateWriter
	FxRateReader
	Close() error
}

// OpenBarFunc opens the bar store (internal/store/barpg.Open, wrapped by cmd/ot)
// with the DSN of the line DSNEnvBar.
type OpenBarFunc func(ctx context.Context, dsn string) (BarStore, error)
