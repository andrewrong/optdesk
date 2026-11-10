package barsrefresh

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// deepenSlackDays is how close to the deepen_days target the oldest stored day must come: a listing
// date or a holiday gap near the target must not cause a request every week.
const deepenSlackDays = 30

// deepen is the history deepening step of one key, run on the day its weekly adjusted_last step is
// due (so a name is asked at most once a week) and only when the 400-day backfill was complete before
// this run (backfilled). While the oldest stored trades day is newer than the run day minus
// deepen_days plus 30 days it asks pieces of at most deepen_piece_days days ending on the day before
// the oldest stored day, and inserts the older rows of each under the current epoch; it stops when
// the oldest day reaches the target or a piece holds no older day. There is no diff test: a piece
// overlaps no stored row.
//
// deferred is true when the name is past deepen_names_per_run for the day: no request was made and
// the caller leaves the weekly step undone, so the name is due again the next day.
func (r *runState) deepen(ctx context.Context, key string, id int64, c broker.Contract, backfilled bool) (deferred bool, terr contracts.TypedError) {
	m := r.j.m
	if !backfilled || m.AdjustedCadenceDays <= 0 {
		return false, nil
	}
	adj, err := r.j.r.PriceBars(ctx, id, basisAdjusted, firstDay, r.day)
	if err != nil {
		return false, asTyped(err, contracts.Retryable)
	}
	if len(adj) > 0 && r.dayTime.Sub(newestReceivedDay(adj)) < time.Duration(m.AdjustedCadenceDays)*24*time.Hour {
		return false, nil
	}
	stored, err := r.j.r.PriceBars(ctx, id, basisTrades, firstDay, r.day)
	if err != nil {
		return false, asTyped(err, contracts.Retryable)
	}
	if len(stored) == 0 {
		return false, nil
	}
	oldest := oldestDay(stored)
	target := r.dayTime.AddDate(0, 0, -m.DeepenDays)
	reach := target.AddDate(0, 0, deepenSlackDays)
	if !oldest.After(reach) {
		return false, nil
	}
	if !r.claimDeepen(key) {
		return true, nil
	}
	epoch := max(maxEpoch(stored), 1)
	for oldest.After(reach) {
		days := min(m.DeepenPieceDays, int(oldest.Sub(target).Hours()/24))
		bars, err := r.j.h.Bars(ctx, broker.BarQuery{
			Contract: c, Kind: broker.BarTrades, DurationDays: days, End: oldest.AddDate(0, 0, -1).Format(dayLayout),
		})
		if err != nil {
			return false, asTyped(err, contracts.Retryable)
		}
		older := make([]broker.Bar, 0, len(bars))
		next := oldest
		for _, b := range bars {
			d, err := time.Parse(dayLayout, b.Day)
			if err != nil {
				return false, newErr(contracts.Permanent, "bad bar day")
			}
			if !d.Before(oldest) {
				continue
			}
			older = append(older, b)
			if d.Before(next) {
				next = d
			}
		}
		if len(older) == 0 {
			return false, nil
		}
		rows, terr := r.rows(older, nil, id, epoch, c.Currency)
		if terr != nil {
			return false, terr
		}
		if _, err := r.j.w.InsertPriceBars(ctx, rows); err != nil {
			return false, asTyped(err, contracts.Retryable)
		}
		oldest = next
	}
	return false, nil
}

// claimDeepen reports whether the key may deepen today: at most deepen_names_per_run keys
// do, and 0 is no cap. The count shares the job's started map under a day key of its own.
func (r *runState) claimDeepen(key string) bool {
	limit := r.j.m.DeepenNamesPerRun
	if limit == 0 {
		return true
	}
	j := r.j
	j.mu.Lock()
	defer j.mu.Unlock()
	day := "deepen " + r.day
	set := j.started[day]
	if set == nil {
		set = map[string]bool{}
		j.started[day] = set
	}
	if set[key] {
		return true
	}
	if len(set) >= limit {
		return false
	}
	set[key] = true
	return true
}

// oldestDay is the earliest session day of rows (not empty).
func oldestDay(rows []contracts.PriceBar) time.Time {
	oldest := rows[0].SessionDay.Time
	for _, r := range rows[1:] {
		if r.SessionDay.Before(oldest) {
			oldest = r.SessionDay.Time
		}
	}
	return oldest
}
