package optionopenpg

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// insertOptionOpenSQL appends one row; a retry of the same key adds no row. Arguments, in order: session_day,
// capture_id, slot, instrument_id, underlying_id, plan_id, generation, market_data_type, requested_at, sent_at,
// received_at, event_ts, connection_generation, status, error_code, skew, outside_band, currency, bid, ask,
// bid_size, ask_size, last, volume, underlying_price, underlying_ccy, underlying_event_ts, iv_bid_decimal,
// iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma, theta_per_calendar_day, vega_per_vol_point.
const insertOptionOpenSQL = `INSERT INTO obs.option_open (` + optionOpenCols + `) VALUES ($1, $2::uuid, $3, $4, $5, ` +
	`$6::uuid, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, ` +
	`$28, $29, $30, $31, $32, $33, $34, $35) ON CONFLICT DO NOTHING`

const optionOpenCols = `session_day, capture_id, slot, instrument_id, underlying_id, plan_id, generation, ` +
	`market_data_type, requested_at, sent_at, received_at, event_ts, connection_generation, status, error_code, ` +
	`skew, outside_band, currency, bid, ask, bid_size, ask_size, last, volume, underlying_price, underlying_ccy, ` +
	`underlying_event_ts, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma, ` +
	`theta_per_calendar_day, vega_per_vol_point`

// selectOptionOpenSQL reads the view: the newest row of each instrument of one day (greatest received_at, ties by
// slot and capture id). Argument: session_day.
const selectOptionOpenSQL = `SELECT DISTINCT ON (instrument_id) session_day, capture_id::text, slot, instrument_id, ` +
	`underlying_id, plan_id::text, generation, market_data_type, requested_at, sent_at, received_at, event_ts, ` +
	`connection_generation, status, error_code, skew, outside_band, currency, bid, ask, bid_size, ask_size, last, ` +
	`volume, underlying_price, underlying_ccy, underlying_event_ts, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, ` +
	`iv_decimal, delta, gamma, theta_per_calendar_day, vega_per_vol_point ` +
	`FROM rd.option_open_r1 WHERE session_day = $1 ` +
	`ORDER BY instrument_id, received_at DESC, slot DESC, capture_id DESC`

// InsertOptionOpen appends the rows in one transaction; see openquote.OptionOpenWriter.
func (s *store) InsertOptionOpen(ctx context.Context, rows []contracts.OptionOpen) error {
	if len(rows) == 0 {
		return nil
	}
	for _, r := range rows {
		if r.Slot == openquote.SlotRefresh {
			return permanent(fmt.Errorf("optionopenpg: a row of slot %q is not stored", openquote.SlotRefresh))
		}
	}
	return s.inTx(ctx, "insert option_open", func(tx execer) error {
		for _, r := range rows {
			if _, err := tx.Exec(ctx, insertOptionOpenSQL, optionOpenArgs(r)...); err != nil {
				return err
			}
		}
		return nil
	})
}

func optionOpenArgs(r contracts.OptionOpen) []any {
	return []any{
		r.SessionDay.Time, r.CaptureID, r.Slot, r.InstrumentID, r.UnderlyingID, r.PlanID, r.Generation,
		r.MarketDataType, r.RequestedAt, r.SentAt, r.ReceivedAt, r.EventTs, r.ConnectionGeneration, r.Status,
		r.ErrorCode, r.Skew, r.OutsideBand, r.Currency, screen(r.Bid), screen(r.Ask), screen(r.BidSize),
		screen(r.AskSize), screen(r.Last), screen(r.Volume), screen(r.UnderlyingPrice), r.UnderlyingCcy,
		r.UnderlyingEventTs, screen(r.IvBidDecimal), screen(r.IvMidDecimal), screen(r.IvAskDecimal),
		screen(r.IvDecimal), screen(r.Delta), screen(r.Gamma), screen(r.ThetaPerCalendarDay),
		screen(r.VegaPerVolPoint),
	}
}

// OptionOpen returns the newest row of each instrument of the day; see openquote.OptionOpenReader.
func (s *store) OptionOpen(ctx context.Context, day contracts.Day) ([]contracts.OptionOpen, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	out := []contracts.OptionOpen{}
	err = s.selectRows(ctx, "select option_open", selectOptionOpenSQL, []any{d}, func(sc scanner) error {
		var r contracts.OptionOpen
		var sd time.Time
		if err := sc.Scan(&sd, &r.CaptureID, &r.Slot, &r.InstrumentID, &r.UnderlyingID, &r.PlanID, &r.Generation,
			&r.MarketDataType, &r.RequestedAt, &r.SentAt, &r.ReceivedAt, &r.EventTs, &r.ConnectionGeneration,
			&r.Status, &r.ErrorCode, &r.Skew, &r.OutsideBand, &r.Currency, &r.Bid, &r.Ask, &r.BidSize, &r.AskSize,
			&r.Last, &r.Volume, &r.UnderlyingPrice, &r.UnderlyingCcy, &r.UnderlyingEventTs, &r.IvBidDecimal,
			&r.IvMidDecimal, &r.IvAskDecimal, &r.IvDecimal, &r.Delta, &r.Gamma, &r.ThetaPerCalendarDay,
			&r.VegaPerVolPoint); err != nil {
			return err
		}
		r.SessionDay = serial(sd)
		r.RequestedAt = r.RequestedAt.UTC()
		r.ReceivedAt = r.ReceivedAt.UTC()
		r.SentAt, r.EventTs, r.UnderlyingEventTs = utc(r.SentAt), utc(r.EventTs), utc(r.UnderlyingEventTs)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// insertPlanSQL arguments: session_day, slot, plan_id, instrument_id, underlying_id, planned_at, class, deferred,
// rank, planned_spot, sigma, sigma_source.
const insertPlanSQL = `INSERT INTO plan.option_open_item (session_day, slot, plan_id, instrument_id, underlying_id, ` +
	`planned_at, class, deferred, rank, planned_spot, sigma, sigma_source) ` +
	`VALUES ($1, $2, $3::uuid, $4, $5, $6, $7, $8, $9, $10, $11, $12) ON CONFLICT DO NOTHING`

// selectPlanSQL reads the view: the items of the latest plan of each underlying. Arguments: session_day, slot,
// underlying ids (empty array: all).
const selectPlanSQL = `SELECT session_day, slot, plan_id::text, instrument_id, underlying_id, planned_at, class, ` +
	`deferred, rank, planned_spot::float8, sigma::float8, sigma_source FROM rd.option_open_item_r1 i ` +
	`WHERE session_day = $1 AND slot = $2 AND (cardinality($3::bigint[]) = 0 OR underlying_id = ANY($3)) ` +
	`AND planned_at = (SELECT max(planned_at) FROM rd.option_open_item_r1 m WHERE m.session_day = i.session_day ` +
	`AND m.slot = i.slot AND m.underlying_id = i.underlying_id) ` +
	`ORDER BY underlying_id, rank, instrument_id`

// PutPlan writes the items; see openquote.OptionOpenPlanStore.
func (s *store) PutPlan(ctx context.Context, items []contracts.OptionOpenItem) error {
	if len(items) == 0 {
		return nil
	}
	return s.inTx(ctx, "put plan", func(tx execer) error {
		for _, it := range items {
			if _, err := tx.Exec(ctx, insertPlanSQL, it.SessionDay.Time, it.Slot, it.PlanID, it.InstrumentID,
				it.UnderlyingID, it.PlannedAt, string(it.Class), it.Deferred, it.Rank, it.PlannedSpot, it.Sigma,
				string(it.SigmaSource)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Plan returns the items of the latest plan of each underlying; see openquote.OptionOpenPlanStore.
func (s *store) Plan(ctx context.Context, day contracts.Day, slot string, underlyingIDs ...int64) ([]contracts.OptionOpenItem, error) {
	d, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	ids := append([]int64{}, underlyingIDs...)
	out := []contracts.OptionOpenItem{}
	err = s.selectRows(ctx, "select plan", selectPlanSQL, []any{d, slot, ids}, func(sc scanner) error {
		var it contracts.OptionOpenItem
		var sd time.Time
		if err := sc.Scan(&sd, &it.Slot, &it.PlanID, &it.InstrumentID, &it.UnderlyingID, &it.PlannedAt, &it.Class,
			&it.Deferred, &it.Rank, &it.PlannedSpot, &it.Sigma, &it.SigmaSource); err != nil {
			return err
		}
		it.SessionDay = serial(sd)
		it.PlannedAt = it.PlannedAt.UTC()
		out = append(out, it)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

const (
	// addNotListedSQL arguments: underlying_id, expiry, strike, right, trading_class, checked_at, retry_at,
	// error_code, error_text. A mark that exists is replaced.
	addNotListedSQL = `INSERT INTO master.option_not_listed (underlying_id, expiry, strike, "right", trading_class, ` +
		`checked_at, retry_at, error_code, error_text) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) ` +
		`ON CONFLICT (underlying_id, expiry, strike, "right", trading_class) DO UPDATE SET ` +
		`checked_at = EXCLUDED.checked_at, retry_at = EXCLUDED.retry_at, error_code = EXCLUDED.error_code, ` +
		`error_text = EXCLUDED.error_text`
	// notListedSQL argument: now.
	notListedSQL = `SELECT underlying_id, expiry, strike, "right", trading_class, checked_at, retry_at, error_code, ` +
		`error_text FROM master.option_not_listed WHERE retry_at > $1 ` +
		`ORDER BY underlying_id, expiry, strike::numeric, "right", trading_class`
	removeNotListedSQL = `DELETE FROM master.option_not_listed WHERE underlying_id = $1 AND expiry = $2 AND ` +
		`strike = $3 AND "right" = $4 AND trading_class = $5`
	removeByUnderlyingSQL = `DELETE FROM master.option_not_listed WHERE underlying_id = $1`
	removeExpiredSQL      = `DELETE FROM master.option_not_listed WHERE expiry < $1`
)

// AddNotListed writes the marks; see openquote.OptionNotListedStore.
func (s *store) AddNotListed(ctx context.Context, marks []contracts.OptionNotListed) error {
	if len(marks) == 0 {
		return nil
	}
	return s.inTx(ctx, "add not_listed", func(tx execer) error {
		for _, m := range marks {
			if _, err := tx.Exec(ctx, addNotListedSQL, m.UnderlyingID, m.Expiry.Time, m.Strike, string(m.Right),
				m.TradingClass, m.CheckedAt, m.RetryAt, m.ErrorCode, m.ErrorText); err != nil {
				return err
			}
		}
		return nil
	})
}

// NotListed returns the marks not yet due at now.
func (s *store) NotListed(ctx context.Context, now time.Time) ([]contracts.OptionNotListed, error) {
	out := []contracts.OptionNotListed{}
	err := s.selectRows(ctx, "select not_listed", notListedSQL, []any{now}, func(sc scanner) error {
		var m contracts.OptionNotListed
		var exp time.Time
		if err := sc.Scan(&m.UnderlyingID, &exp, &m.Strike, &m.Right, &m.TradingClass, &m.CheckedAt, &m.RetryAt,
			&m.ErrorCode, &m.ErrorText); err != nil {
			return err
		}
		m.Expiry = serial(exp)
		m.CheckedAt, m.RetryAt = m.CheckedAt.UTC(), m.RetryAt.UTC()
		out = append(out, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// exec runs one statement in a transaction and returns the rows it affected.
func (s *store) exec(ctx context.Context, what, sql string, args ...any) (int, error) {
	var n int64
	err := s.inTx(ctx, what, func(tx execer) error {
		var err error
		n, err = tx.Exec(ctx, sql, args...)
		return err
	})
	return int(n), err
}

// RemoveNotListed removes the mark of key and reports whether one was removed.
func (s *store) RemoveNotListed(ctx context.Context, k openquote.NotListedKey) (bool, error) {
	exp, err := parseDay(k.Expiry)
	if err != nil {
		return false, err
	}
	n, err := s.exec(ctx, "remove not_listed", removeNotListedSQL, k.UnderlyingID, exp, k.Strike, k.Right, k.TradingClass)
	return n > 0, err
}

// RemoveNotListedByUnderlying removes every mark of the underlying.
func (s *store) RemoveNotListedByUnderlying(ctx context.Context, underlyingID int64) (int, error) {
	return s.exec(ctx, "remove not_listed by underlying", removeByUnderlyingSQL, underlyingID)
}

// RemoveNotListedExpired removes the marks whose expiry is before the day.
func (s *store) RemoveNotListedExpired(ctx context.Context, day contracts.Day) (int, error) {
	d, err := parseDay(day)
	if err != nil {
		return 0, err
	}
	return s.exec(ctx, "remove expired not_listed", removeExpiredSQL, d)
}

const (
	// putOptionSQL arguments: instrument_id, underlying_id, currency, exchange, trading_class, multiplier, expiry,
	// strike, right. A contract that exists takes the new instrument id and resolved_at.
	putOptionSQL = `INSERT INTO master.option_contract (instrument_id, underlying_id, currency, exchange, ` +
		`trading_class, multiplier, expiry, strike, "right", resolved_at) ` +
		`VALUES ($1, $2, $3, $4, $5, $6::numeric, $7, $8, $9, now()) ` +
		`ON CONFLICT (underlying_id, currency, exchange, trading_class, multiplier, expiry, strike, "right") ` +
		`DO UPDATE SET instrument_id = EXCLUDED.instrument_id, resolved_at = EXCLUDED.resolved_at`
	// listOptionsSQL argument: underlying_id.
	listOptionsSQL = `SELECT instrument_id, underlying_id, currency, exchange, trading_class, multiplier::text, ` +
		`expiry, strike, "right" FROM master.option_contract WHERE underlying_id = $1 ` +
		`ORDER BY expiry, strike::numeric, "right", trading_class, exchange, currency, multiplier`
)

// PutOption stores the contract; see openquote.OptionContractStore.
func (s *store) PutOption(ctx context.Context, r openquote.OptionRef) error {
	exp, err := parseDay(r.Expiry)
	if err != nil {
		return err
	}
	_, err = s.exec(ctx, "put option", putOptionSQL, r.InstrumentID, r.UnderlyingID, r.Currency, r.Exchange,
		r.TradingClass, r.Multiplier, exp, r.Strike, r.Right)
	return err
}

// ListOptions returns the contracts of the underlying.
func (s *store) ListOptions(ctx context.Context, underlyingID int64) ([]openquote.OptionRef, error) {
	out := []openquote.OptionRef{}
	err := s.selectRows(ctx, "list options", listOptionsSQL, []any{underlyingID}, func(sc scanner) error {
		var r openquote.OptionRef
		var exp time.Time
		if err := sc.Scan(&r.InstrumentID, &r.UnderlyingID, &r.Currency, &r.Exchange, &r.TradingClass,
			&r.Multiplier, &exp, &r.Strike, &r.Right); err != nil {
			return err
		}
		r.Expiry = dayOf(exp)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
