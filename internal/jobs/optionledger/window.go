package optionledger

import (
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout   = "2006-01-02"
	overlapDays = 30  // days before the last day of the latest ok fetch that are fetched again
	maxSpanDays = 365 // inclusive days of one window
)

// window is one inclusive span of days asked in one Fetch.
type window struct{ first, last contracts.Day }

// planWindows returns the windows of one query, oldest first. last is the day before the job's time in New
// York. The first day is historyFrom when the query has no stored ok fetch (latestOkLast empty) or when an
// allow-listed account has no stored row (rowsForAll false); else it is overlapDays before latestOkLast and
// never before historyFrom. The span is cut into windows of at most maxSpanDays days. A first day after last
// gives no window.
func planWindows(historyFrom, last, latestOkLast contracts.Day, rowsForAll bool) ([]window, error) {
	from, err := time.Parse(dayLayout, string(historyFrom))
	if err != nil {
		return nil, fmt.Errorf("optionledger: history_from is not a day")
	}
	end, err := time.Parse(dayLayout, string(last))
	if err != nil {
		return nil, fmt.Errorf("optionledger: last day is not a day")
	}
	first := from
	if latestOkLast != "" && rowsForAll {
		ok, err := time.Parse(dayLayout, string(latestOkLast))
		if err != nil {
			return nil, fmt.Errorf("optionledger: stored last day is not a day")
		}
		if c := ok.AddDate(0, 0, -overlapDays); c.After(first) {
			first = c
		}
	}
	var out []window
	for !first.After(end) {
		stop := first.AddDate(0, 0, maxSpanDays-1)
		if stop.After(end) {
			stop = end
		}
		out = append(out, window{first: contracts.Day(first.Format(dayLayout)), last: contracts.Day(stop.Format(dayLayout))})
		first = stop.AddDate(0, 0, 1)
	}
	return out, nil
}
