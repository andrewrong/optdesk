// Package hoursnotice declares the store of calendar-hours notices: one row per day on which our exchange calendar
// and IBKR's hours differ (docs/design/calendar-ibkr-hours.md, entry 5, Decisions 6, 7 and 9). It holds
// declarations only; the implementation is internal/store/hoursnoticepg.
package hoursnotice

import "context"

// Notice is one differing day. Day is YYYY-MM-DD in the market zone; Ours and IBKR are the two hours strings.
type Notice struct {
	Market string
	Day    string
	Ours   string
	IBKR   string
}

// Reader reads the rows of the view rd.session_hours_notice_r1.
type Reader interface {
	// Notices returns every row, ordered by market and then day. No rows gives an empty slice.
	Notices(ctx context.Context) ([]Notice, error)
	Close() error
}

// Store is the Reader and the writer of one market's rows.
type Store interface {
	Reader
	// Replace deletes every row of market and inserts rows in one transaction (all or none): a day that is not in
	// rows is gone afterwards, and an empty rows deletes the market's rows. Rows of other markets are untouched.
	// Each row's Market must equal market, else the call fails and changes nothing.
	Replace(ctx context.Context, market string, rows []Notice) error
}
