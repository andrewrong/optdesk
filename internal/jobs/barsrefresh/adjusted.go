package barsrefresh

import (
	"context"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const basisAdjusted = "adjusted_last"

// adjusted is the weekly step of one key: when the key has no adjusted_last winner row, or its newest
// winner row was received adjusted_cadence_days or more calendar days before the run day, it asks
// once for the whole backfill window up to now and inserts every answered row under epoch + 1 in one
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
	epoch := maxEpoch(stored) + 1
	now := r.j.clk.Now().UTC()
	rows := make([]contracts.PriceBar, 0, len(bars))
	for _, b := range bars {
		d, err := time.Parse(dayLayout, b.Day)
		if err != nil {
			return newErr(contracts.Permanent, "bad bar day")
		}
		if _, err := parseRat(b.Close); err != nil {
			return asTyped(err, contracts.Permanent)
		}
		rows = append(rows, contracts.PriceBar{
			AdjustEpoch: epoch, Basis: basisAdjusted, Close: b.Close, Currency: c.Currency,
			High: b.High, Low: b.Low, Open: b.Open, InstrumentID: int(id), ReceivedAt: now,
			SessionDay: types.SerializableDate{Time: d}, Source: sourceIbkr, Volume: b.Volume,
		})
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := r.j.w.InsertPriceBars(ctx, rows); err != nil {
		return asTyped(err, contracts.Retryable)
	}
	return nil
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
