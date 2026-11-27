package barsrefresh

import (
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// windowDays is the length in calendar days of the one request that covers what the key is missing up to
// day. With no stored bar it is backfill. Otherwise the window starts the day after the newest stored bar,
// or at the oldest stored bar whose source is yfinance when there is one (those rows are replaced), and ends
// at day; the length is at least daily and at most backfill.
func windowDays(stored []contracts.PriceBar, day time.Time, daily, backfill int) int {
	if len(stored) == 0 {
		return backfill
	}
	newest := stored[0].SessionDay.Time
	var oldestYf time.Time
	for _, r := range stored {
		d := r.SessionDay.Time
		if d.After(newest) {
			newest = d
		}
		if r.Source == sourceYfinance && (oldestYf.IsZero() || d.Before(oldestYf)) { // the rows of a yfinance recovery
			oldestYf = d
		}
	}
	n := calendarDays(newest, day) // the days after the newest bar, through day
	if hasSource(stored, sourceYfinance) {
		n = calendarDays(oldestYf, day) + 1 // the yfinance day itself is asked again
	}
	if n > backfill {
		n = backfill
	}
	if n < daily {
		n = daily
	}
	return n
}

// calendarDays is the number of calendar days from `from` to `to` (zero for the same day).
func calendarDays(from, to time.Time) int {
	f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(t.Sub(f).Hours() / 24)
}

// dailyComplete reports whether the daily step has nothing to ask for a key: the newest stored session day
// is day itself and no stored row has source yfinance (such a row is replaced by a request). No rows is false.
func dailyComplete(stored []contracts.PriceBar, day time.Time) bool {
	if len(stored) == 0 || hasSource(stored, sourceYfinance) {
		return false
	}
	newest := stored[0].SessionDay.Time
	for _, r := range stored {
		if r.SessionDay.After(newest) {
			newest = r.SessionDay.Time
		}
	}
	return calendarDays(newest, day) == 0
}
