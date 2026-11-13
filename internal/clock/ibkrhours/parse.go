// Package ibkrhours reads IBKR liquid-hours strings into UTC days. It is an own implementation,
// because no maintained Go library parses IBKR's session strings.
package ibkrhours

import (
	"fmt"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Parser is the clock.IBKRHours implementation.
type Parser struct{}

// New returns the parser.
func New() clock.IBKRHours { return Parser{} }

// Parse splits liquidHours on ";", groups tokens by start date and returns each day's first
// start and last end as UTC instants of the zone timeZoneID. A CLOSED token is a closed day.
func (Parser) Parse(liquidHours, timeZoneID string) ([]clock.HoursDay, error) {
	if timeZoneID == "" {
		return nil, fmt.Errorf(`ibkrhours: time zone "" is empty`)
	}
	loc, err := time.LoadLocation(timeZoneID)
	if err != nil {
		return nil, fmt.Errorf("ibkrhours: unknown time zone %q: %w", timeZoneID, err)
	}
	var days []clock.HoursDay
	index := map[contracts.Day]int{}
	for _, tok := range strings.Split(liquidHours, ";") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		day, open, closeAt, closed, err := parseToken(tok, loc)
		if err != nil {
			return nil, err
		}
		i, seen := index[day]
		switch {
		case !seen:
			index[day] = len(days)
			days = append(days, clock.HoursDay{Day: day, IsSession: !closed, OpenAt: open, CloseAt: closeAt})
		case closed:
			// A CLOSED token next to segments of the same day adds nothing.
		case !days[i].IsSession:
			days[i] = clock.HoursDay{Day: day, IsSession: true, OpenAt: open, CloseAt: closeAt}
		default:
			if open.Before(days[i].OpenAt) {
				days[i].OpenAt = open
			}
			if closeAt.After(days[i].CloseAt) {
				days[i].CloseAt = closeAt
			}
		}
	}
	return days, nil
}

func parseToken(tok string, loc *time.Location) (day contracts.Day, open, closeAt time.Time, closed bool, err error) {
	bad := func(why string) error { return fmt.Errorf("ibkrhours: token %q: %s", tok, why) }
	if date, rest, ok := strings.Cut(tok, ":"); ok && rest == "CLOSED" {
		d, perr := time.ParseInLocation("20060102", date, loc)
		if perr != nil {
			return "", time.Time{}, time.Time{}, false, bad("bad date")
		}
		return contracts.Day(d.Format("2006-01-02")), time.Time{}, time.Time{}, true, nil
	}
	start, end, ok := strings.Cut(tok, "-")
	if !ok {
		return "", time.Time{}, time.Time{}, false, bad("want YYYYMMDD:HHMM-YYYYMMDD:HHMM or YYYYMMDD:CLOSED")
	}
	s, serr := time.ParseInLocation("20060102:1504", start, loc)
	e, eerr := time.ParseInLocation("20060102:1504", end, loc)
	if serr != nil || eerr != nil {
		return "", time.Time{}, time.Time{}, false, bad("want YYYYMMDD:HHMM-YYYYMMDD:HHMM or YYYYMMDD:CLOSED")
	}
	if e.Before(s) {
		return "", time.Time{}, time.Time{}, false, bad("segment ends before it starts")
	}
	return contracts.Day(s.Format("2006-01-02")), s.UTC(), e.UTC(), false, nil
}
