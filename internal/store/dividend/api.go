// Package dividend declares the store of obs.dividend_expectation (docs/design/event-sources.md Decisions 7 and 8).
// It holds declarations only; the implementation is internal/store/events.OpenDividend.
package dividend

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Writer appends the rows of obs.dividend_expectation.
type Writer interface {
	// InsertDividend inserts one row ON CONFLICT DO NOTHING on (session_day, instrument_id, attempt): a row whose
	// key exists stays as it is and the call returns no error. An instrument_id of 0 or an attempt below 1 is
	// Permanent, a database error Retryable.
	InsertDividend(ctx context.Context, row contracts.DividendExpectation) error
}

// Reader reads the rows through the view rd.dividend_expectation_r1.
type Reader interface {
	// Dividends returns the rows of one instrument with session_day from from to to, inclusive, ascending by
	// day, only the highest attempt of each day; none gives an empty slice. A bad or reversed day range is
	// Permanent, a database error Retryable.
	Dividends(ctx context.Context, instrumentID int, from, to contracts.Day) ([]contracts.DividendExpectation, error)
}

// Store is one store for both sides: the dividend job opens it with the line store.DSNEnvEvents (role
// ot_w_events, INSERT on the table and SELECT on the view), a page with store.DSNEnvPages (role ot_ro_pages,
// SELECT on the view only; its InsertDividend fails).
type Store interface {
	Writer
	Reader
	Close() error
}

// OpenFunc opens the dividend store (internal/store/events.OpenDividend).
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
