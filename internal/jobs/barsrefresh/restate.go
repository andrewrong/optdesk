package barsrefresh

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func parseRat(s string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, fmt.Errorf("barsrefresh: %q is not a decimal", s)
	}
	return r, nil
}

// differs reports whether pulled differs from stored by more than tol, relative to stored
// (Decision 4): |pulled - stored| > tol * |stored|. A stored zero differs from any other value.
func differs(stored, pulled, tol *big.Rat) bool {
	diff := new(big.Rat).Sub(pulled, stored)
	diff.Abs(diff)
	limit := new(big.Rat).Mul(tol, new(big.Rat).Abs(stored))
	return diff.Cmp(limit) > 0
}

// restated reports whether a pulled day that is already stored differs from the stored winner close by
// more than tol. Days not stored never count. A close that is not a decimal is an error.
func restated(stored []contracts.PriceBar, pulled []broker.Bar, tol *big.Rat) (bool, error) {
	byDay := make(map[string]string, len(stored))
	for _, s := range stored {
		byDay[s.SessionDay.Format(dayLayout)] = s.Close
	}
	out := false
	for _, p := range pulled {
		sc, ok := byDay[p.Day]
		if !ok {
			continue
		}
		sr, err := parseRat(sc)
		if err != nil {
			return false, err
		}
		pr, err := parseRat(p.Close)
		if err != nil {
			return false, err
		}
		if differs(sr, pr, tol) {
			out = true
		}
	}
	return out, nil
}

// olderThanWindow pulls again the stored days older than the oldest day of window: pieces of at most
// deepen_piece_days ending the day before the oldest pulled day, down to the oldest stored day. A piece
// with no older day stops the loop. It returns the rows of every piece and writes nothing, so that the
// restatement is written in one call after the last pull.
func (r *runState) olderThanWindow(
	ctx context.Context, c broker.Contract, id int64, stored []contracts.PriceBar, window []broker.Bar,
) ([]contracts.PriceBar, contracts.TypedError) {
	if len(window) == 0 || len(stored) == 0 {
		return nil, nil
	}
	oldest, err := time.Parse(dayLayout, window[0].Day)
	if err != nil {
		return nil, newErr(contracts.Permanent, "bad bar day")
	}
	for _, b := range window {
		d, err := time.Parse(dayLayout, b.Day)
		if err != nil {
			return nil, newErr(contracts.Permanent, "bad bar day")
		}
		if d.Before(oldest) {
			oldest = d
		}
	}
	floor := oldestDay(stored)
	var out []contracts.PriceBar
	for oldest.After(floor) {
		days := min(r.j.m.DeepenPieceDays, int(oldest.Sub(floor).Hours()/24))
		bars, err := r.j.h.Bars(ctx, broker.BarQuery{
			Contract: c, Kind: broker.BarTrades, DurationDays: days, End: oldest.AddDate(0, 0, -1).Format(dayLayout),
		})
		if err != nil {
			return nil, asTyped(err, contracts.Retryable)
		}
		older := make([]broker.Bar, 0, len(bars))
		next := oldest
		for _, b := range bars {
			d, err := time.Parse(dayLayout, b.Day)
			if err != nil {
				return nil, newErr(contracts.Permanent, "bad bar day")
			}
			if !d.Before(oldest) {
				continue
			}
			older = append(older, b)
			next = minTime(next, d)
		}
		if len(older) == 0 {
			break
		}
		rows, terr := r.rows(older, false, id, c.Currency)
		if terr != nil {
			return nil, terr
		}
		out = append(out, rows...)
		oldest = next
	}
	return out, nil
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
