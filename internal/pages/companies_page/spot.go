package companies_page

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Spot is the last price of a symbol with the day it was struck: Price is the decimal text of the
// newest stored daily close and Day the session day of that bar, which may differ from the page day.
// Design: docs/design/companies-day.md#Decisions (4).
type Spot struct {
	Price string
	Day   contracts.Day
}

// spotAt is the one Spot rule of the page: ok is false for a nil Spot, a not-ok answer, an empty
// Price or an empty Day. Build and coreRowsOf both read through it, so a row with "no quote" in the
// table has no price in the growth legend, and the reverse.
func (c Config) spotAt(ctx context.Context, symbol string, day contracts.Day) (Spot, bool, error) {
	if c.Spot == nil {
		return Spot{}, false, nil
	}
	s, ok, err := c.Spot(ctx, symbol, day)
	if err != nil {
		return Spot{}, false, fmt.Errorf("companies_page: spot of %s: %w", symbol, err)
	}
	if !ok || s.Price == "" || s.Day == "" {
		return Spot{}, false, nil
	}
	return s, true, nil
}
