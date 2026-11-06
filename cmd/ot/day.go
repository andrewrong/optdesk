package main

import (
	"fmt"
	"time"
	_ "time/tzdata" // the zone database travels with the binary

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// marketZone is the zone whose date is the default day.
const marketZone = "America/New_York"

const dayLayout = "2006-01-02"

// dayOrDefault returns the --day flag as a Day, or the date of clk.Now() in marketZone when
// the flag is empty. A malformed flag is an error.
func dayOrDefault(clk clock.Clock, flag string) (contracts.Day, error) {
	if flag != "" {
		t, err := time.Parse(dayLayout, flag)
		if err != nil {
			return "", fmt.Errorf("--day %q is not YYYY-MM-DD", flag)
		}
		return contracts.Day(t.Format(dayLayout)), nil
	}
	loc, err := time.LoadLocation(marketZone)
	if err != nil {
		return "", fmt.Errorf("zone %s: %w", marketZone, err)
	}
	return contracts.Day(clk.Now().In(loc).Format(dayLayout)), nil
}
