// Package optionday declares the reader of obs.option_day: the last good quote
// of one contract on one session day. It holds declarations only; the
// implementation is internal/store/snapshot.
package optionday

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Day is one row of obs.option_day: the last good quote of one contract on one session day.
// Decimals are text as stored; a column with no value is nil.
type Day struct {
	InstrumentID, UnderlyingID                                                  int64
	SessionDay                                                                  contracts.Day
	Source                                                                      string // option_eod or option_open: the capture the quote came from
	ReceivedAt                                                                  time.Time
	Status                                                                      int
	Currency, UnderlyingCcy                                                     string
	Bid, Ask, BidSize, AskSize, Last, Volume, UnderlyingPrice                   *string
	IvBid, IvMid, IvAsk, Iv, Delta, Gamma, ThetaPerCalendarDay, VegaPerVolPoint *string
	OpenInterest                                                                *string
	OiAsofDate                                                                  *contracts.Day
}

// DayReader reads the view rd.option_day_r1, ordered by session_day, then instrument_id. No row in the range
// gives an empty slice and no error; instants are UTC.
type DayReader interface {
	// Days returns the rows of the given contracts with session_day between from and to, both inclusive.
	Days(ctx context.Context, instrumentIDs []int64, from, to contracts.Day) ([]Day, error)
	// ChainDay returns the rows of every contract of the underlying on day: the chain as the day last saw it.
	ChainDay(ctx context.Context, underlyingID int64, day contracts.Day) ([]Day, error)
	// Close releases the connection.
	Close() error
}

// OpenDayReaderFunc connects to the database named by dsn (role ot_ro_pages or ot_runner); implemented by snapshot.OpenDayReader.
type OpenDayReaderFunc func(ctx context.Context, dsn string) (DayReader, error)
