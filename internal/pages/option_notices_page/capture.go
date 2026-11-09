package option_notices_page

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// lookbackDays is how many calendar days before a day a capture with positions is searched for.
const lookbackDays = 7

// capture is the stored rows of one alias: the latest day with positions and the one before it.
type capture struct {
	Day       contracts.Day
	Positions []store.Position
	Values    []store.AccountValue
	PrevDay   contracts.Day // empty when no earlier day has positions
	Prev      []store.Position
}

// latestPositions is the first day from start, going back at most lookbackDays, that has positions.
func latestPositions(ctx context.Context, r store.AccountReader, alias string, start contracts.Day) (contracts.Day, []store.Position, error) {
	for i := 0; i <= lookbackDays; i++ {
		d, err := daysBack(start, i)
		if err != nil {
			return "", nil, fmt.Errorf("option_notices_page: bad day %q: %w", start, err)
		}
		rows, err := r.Positions(ctx, alias, string(d))
		if err != nil {
			return "", nil, fmt.Errorf("option_notices_page: read positions of %s: %w", alias, err)
		}
		if len(rows) > 0 {
			return d, rows, nil
		}
	}
	return "", nil, nil
}

// findCapture finds the capture of alias for day; ok is false when no day in the lookback has
// positions. A reader error is returned.
func findCapture(ctx context.Context, r store.AccountReader, alias string, day contracts.Day) (c capture, ok bool, err error) {
	capDay, rows, err := latestPositions(ctx, r, alias, day)
	if err != nil || capDay == "" {
		return capture{}, false, err
	}
	c = capture{Day: capDay, Positions: rows}
	if c.Values, err = r.Values(ctx, alias, string(capDay)); err != nil {
		return capture{}, false, fmt.Errorf("option_notices_page: read values of %s: %w", alias, err)
	}
	before, err := daysBack(capDay, 1)
	if err != nil {
		return capture{}, false, fmt.Errorf("option_notices_page: bad day %q: %w", capDay, err)
	}
	if c.PrevDay, c.Prev, err = latestPositions(ctx, r, alias, before); err != nil {
		return capture{}, false, err
	}
	return c, true, nil
}
