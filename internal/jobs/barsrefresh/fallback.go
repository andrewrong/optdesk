package barsrefresh

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

const (
	sourceYfinance = "yfinance"

	codeSourceFailed = "source_failed"

	// histDownAfter is the number of history retries one run makes: each waits retryPause.
	histDownAfter = 2
)

// WithFallback sets the yfinance source and the session calendar of a job built by New and returns it:
// a key with a yfinance route (USD or JPY stock or ETF) then gets its trades and adjusted_last bars from src
// and asks IB for none (docs/design/stock-history-yfinance.md, Decisions 1 to 6). A nil src or cal, or a job of another
// package, is an error.
func WithFallback(j contracts.Job, src sources.BarSource, cal clock.Calendar) (contracts.Job, error) {
	if src == nil || cal == nil {
		return nil, errors.New("barsrefresh: the fallback needs a bar source and a calendar")
	}
	jb, ok := j.(*job)
	if !ok {
		return nil, errors.New("barsrefresh: the fallback attaches only to a job built by New")
	}
	jb.src, jb.cal = src, cal
	return jb, nil
}

// pairOf maps the contract of a key to the pair the source is asked: an index, and a currency other
// than USD and JPY, have none.
func pairOf(c broker.Contract) (sources.BarPair, bool) {
	if c.SecType == "IND" || c.Symbol == "" || (c.Currency != "USD" && c.Currency != "JPY") {
		return sources.BarPair{}, false
	}
	return sources.BarPair{Symbol: c.Symbol, Currency: c.Currency}, true
}

func calendarOf(currency string) string {
	if currency == "JPY" {
		return "XTKS"
	}
	return "XNYS"
}

// sessionClosed reports whether the session of day in the calendar of currency is over: a day that is
// no session, or whose close is still ahead of the clock (a half day closes at its early close), is not.
func (r *runState) sessionClosed(ctx context.Context, currency string, day contracts.Day) (bool, error) {
	id := calendarOf(currency)
	s, err := r.j.cal.Session(ctx, id, day)
	if err != nil {
		return false, err
	}
	return s.IsSession && !r.j.clk.Now().Before(s.CloseAt), nil
}

// hasSource reports whether a row of rows has the given source.
func hasSource(rows []contracts.PriceBar, source string) bool {
	for _, r := range rows {
		if string(r.Source) == source {
			return true
		}
	}
	return false
}
