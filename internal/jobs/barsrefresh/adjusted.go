package barsrefresh

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const basisAdjusted = "adjusted_last"

// adjusted is the weekly step of one key: when the key has no adjusted_last winner row, or its newest
// winner row was received adjusted_cadence_days or more calendar days before the run day, it asks
// once for the whole backfill window up to now and upserts every answered row by key in one
// call. A failure leaves the stored rows as they are. A cadence of 0 or less switches the step off.
func (r *runState) adjusted(ctx context.Context, id int64, c broker.Contract) contracts.TypedError {
	cadence := r.j.m.AdjustedCadenceDays
	if cadence <= 0 {
		return nil
	}
	stored, err := r.j.r.PriceBars(ctx, id, basisAdjusted, firstDay, r.day)
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	if len(stored) > 0 && r.dayTime.Sub(newestReceivedDay(stored)) < time.Duration(cadence)*24*time.Hour {
		return nil
	}
	bars, err := r.j.h.Bars(ctx, broker.BarQuery{Contract: c, Kind: broker.BarAdjustedLast, DurationDays: r.j.m.BackfillDays})
	if err != nil {
		return asTyped(err, contracts.Retryable)
	}
	rows, terr := r.rows(bars, true, id, c.Currency)
	if terr != nil {
		return terr
	}
	return r.upsert(ctx, rows)
}

// newestReceivedDay is the UTC calendar day of the latest ReceivedAt of rows.
func newestReceivedDay(rows []contracts.PriceBar) time.Time {
	var newest time.Time
	for _, r := range rows {
		if r.ReceivedAt.After(newest) {
			newest = r.ReceivedAt
		}
	}
	y, m, d := newest.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
