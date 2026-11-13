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
// the oldest stored day, and upserts the older rows of each; it stops when
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
	if r.askedRecently(key) {
		return false, nil
	}
	if !r.claimDeepen(key) {
		return true, nil
	}
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
		rows, terr := r.rows(older, id, c.Currency)
		if terr != nil {
			return false, terr
		}
		if terr := r.upsert(ctx, rows); terr != nil {
			return false, terr
		}
		oldest = next
	}
	return false, nil
}

// deepenDayKey is the day key under which the job's started map records the keys that asked a
// deepening piece on that day: it gives the per-day cap and the once-a-week limit, which must hold
// even when the weekly adjusted_last answer is empty and stores nothing.
func deepenDayKey(day time.Time) contracts.Day {
	return contracts.Day("deepen " + day.Format(dayLayout))
}

// askedRecently reports whether key asked a deepening piece on one of the previous
// adjusted_cadence_days - 1 days (a retry on the same day is not recent).
func (r *runState) askedRecently(key string) bool {
	j := r.j
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := 1; i < j.m.AdjustedCadenceDays; i++ {
		if j.started[deepenDayKey(r.dayTime.AddDate(0, 0, -i))][key] {
			return true
		}
	}
	return false
}

// claimDeepen records that key asks today and reports whether it may: at most deepen_names_per_run
// keys do, and 0 is no cap.
func (r *runState) claimDeepen(key string) bool {
	j := r.j
	j.mu.Lock()
	defer j.mu.Unlock()
	dk := deepenDayKey(r.dayTime)
	set := j.started[dk]
	if set == nil {
		set = map[string]bool{}
		j.started[dk] = set
	}
	if set[key] {
		return true
	}
	if limit := j.m.DeepenNamesPerRun; limit != 0 && len(set) >= limit {
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
