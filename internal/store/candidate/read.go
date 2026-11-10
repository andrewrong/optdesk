package candidate

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// selectSQL reads the rows of one day and one kind through the view, ranked rows first.
const selectSQL = `SELECT ` + columns + ` FROM rd.option_candidate_r1 WHERE session_day = $1 AND kind = $2 ` +
	`ORDER BY rank ASC NULLS LAST, instrument_id, alias, run_id`

// Candidates returns the rows of one kind of one day written by the newest run of that day (the latest
// computed_at), ranked rows first; none gives an empty slice. A zero or bad day or an unknown kind is an error.
// The query returns every run of the day and the newest is picked here: a day holds a few hundred rows a run.
func (s *Store) Candidates(ctx context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	if !validKind(kind) {
		return nil, permanent(fmt.Errorf("candidate: unknown kind %q", kind))
	}
	var all []store.Candidate
	err = s.db.Select(ctx, selectSQL, []any{d, kind}, func(sc scanner) error {
		c, err := scanRow(sc)
		if err != nil {
			return err
		}
		all = append(all, c)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("candidate: select: %w", err))
	}
	return newestRun(all), nil
}

// newestRun keeps the rows of the run with the latest ComputedAt, in their order; a tie goes to the larger run id.
func newestRun(all []store.Candidate) []store.Candidate {
	out := []store.Candidate{}
	if len(all) == 0 {
		return out
	}
	best := all[0]
	for _, c := range all[1:] {
		if c.ComputedAt.After(best.ComputedAt) || (c.ComputedAt.Equal(best.ComputedAt) && c.RunID > best.RunID) {
			best = c
		}
	}
	for _, c := range all {
		if c.RunID == best.RunID {
			out = append(out, c)
		}
	}
	return out
}

func scanRow(sc scanner) (store.Candidate, error) {
	var c store.Candidate
	var day, expiry time.Time
	var next, ex *time.Time
	err := sc.Scan(&day, &c.RunID, &c.Kind, &c.InstrumentID, &c.Alias, &c.RuleVersion, &c.Issuer, &c.UnderlyingID,
		&expiry, &c.DTE, &c.Right, &c.Strike, &c.Multiplier, &c.BidEod, &c.Ask, &c.Spot, &c.BuyPrice, &c.SalePrice,
		&c.BreakEven, &c.AnnualizedYield, &c.Delta, &c.Iv, &c.Status, &c.Gates, &c.Marks, &c.Fits, &next,
		&c.EarningsState, &ex, &c.ExAmount, &c.Extrinsic, &c.Rank, &c.RankInUnderlying, &c.ComputedAt,
		&c.SettingsVersion)
	if err != nil {
		return c, err
	}
	c.SessionDay = contracts.Day(day.Format(time.DateOnly))
	c.Expiry = contracts.Day(expiry.Format(time.DateOnly))
	c.NextEarnings = dayPtr(next)
	c.NextExDate = dayPtr(ex)
	return c, nil
}

func dayPtr(t *time.Time) *contracts.Day {
	if t == nil {
		return nil
	}
	d := contracts.Day(t.Format(time.DateOnly))
	return &d
}
