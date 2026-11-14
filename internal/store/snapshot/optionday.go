package snapshot

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

var _ optionday.OpenDayReaderFunc = OpenDayReader

// optionDayWriter is written to obs.option_day.written_by by this store.
const optionDayWriter = "option_eod"

// optionDayUsable reports whether r writes a day row: status 1 and snapshot_kind session
// (docs/design/option-captures.md Decision 6).
func optionDayUsable(r contracts.OptionEod) bool {
	return r.Status == optionEodOK && r.SnapshotKind == "session"
}

// optionDayArgs lists the arguments of shared.UpsertDaySQL for the usable row r, every numeric column screened, in the
// order that statement documents (shared.DayArgCount in all).
func optionDayArgs(r contracts.OptionEod) []any {
	var oi *time.Time
	if r.OiAsofDate != nil {
		oi = &r.OiAsofDate.Time
	}
	return []any{
		r.InstrumentID, r.SessionDay.Time, r.UnderlyingID, "option_eod", r.ReceivedAt, r.Status, r.Currency,
		screenSentinel(r.Bid), screenSentinel(r.Ask), screenSentinel(r.BidSize), screenSentinel(r.AskSize),
		screenSentinel(r.Last), screenSentinel(r.Volume), screenSentinel(r.UnderlyingPrice), r.UnderlyingCcy,
		screenSentinel(r.IvBidDecimal), screenSentinel(r.IvMidDecimal), screenSentinel(r.IvAskDecimal),
		screenSentinel(r.IvDecimal), screenSentinel(r.Delta), screenSentinel(r.Gamma),
		screenSentinel(r.ThetaPerCalendarDay), screenSentinel(r.VegaPerVolPoint), screenSentinel(r.OpenInterest), oi,
		optionDayWriter,
	}
}

const selectOptionDayColumns = `SELECT instrument_id, underlying_id, session_day, source, received_at, status, ` +
	`currency, underlying_ccy, bid::text, ask::text, bid_size::text, ask_size::text, last::text, volume::text, ` +
	`underlying_price::text, iv_bid_decimal::text, iv_mid_decimal::text, iv_ask_decimal::text, iv_decimal::text, ` +
	`delta::text, gamma::text, theta_per_calendar_day::text, vega_per_vol_point::text, open_interest::text, ` +
	`oi_asof_date FROM rd.option_day_r1 `

// selectOptionDaysSQL reads the days of some contracts. Arguments: instrument ids, first day, last day.
const selectOptionDaysSQL = selectOptionDayColumns +
	`WHERE instrument_id = ANY($1) AND session_day >= $2 AND session_day <= $3 ORDER BY session_day, instrument_id`

// selectOptionChainDaySQL reads the chain of one underlying on a day. Arguments: underlying id, day.
const selectOptionChainDaySQL = selectOptionDayColumns +
	`WHERE underlying_id = $1 AND session_day = $2 ORDER BY session_day, instrument_id`

type dayReader struct{ db database }

// OpenDayReader connects to the database named by dsn (role ot_ro_pages or ot_runner) and returns the reader of
// rd.option_day_r1. It applies no migration.
func OpenDayReader(ctx context.Context, dsn string) (optionday.DayReader, error) {
	db, err := openDB(ctx, "option_day", dsn)
	if err != nil {
		return nil, err
	}
	return &dayReader{db: db}, nil
}

// Days returns the rows of the given contracts with session_day between from and to, both inclusive.
func (d *dayReader) Days(ctx context.Context, instrumentIDs []int64, from, to contracts.Day) ([]optionday.Day, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	if instrumentIDs == nil {
		instrumentIDs = []int64{}
	}
	return d.read(ctx, selectOptionDaysSQL, []any{instrumentIDs, f, t})
}

// ChainDay returns the rows of every contract of the underlying on day.
func (d *dayReader) ChainDay(ctx context.Context, underlyingID int64, day contracts.Day) ([]optionday.Day, error) {
	t, err := parseDay(day)
	if err != nil {
		return nil, err
	}
	return d.read(ctx, selectOptionChainDaySQL, []any{underlyingID, t})
}

func (d *dayReader) read(ctx context.Context, sql string, args []any) ([]optionday.Day, error) {
	out := []optionday.Day{}
	err := d.db.Select(ctx, sql, args, func(sc scanner) error {
		var r optionday.Day
		var sessionDay time.Time
		var oi *time.Time
		if err := sc.Scan(&r.InstrumentID, &r.UnderlyingID, &sessionDay, &r.Source, &r.ReceivedAt, &r.Status,
			&r.Currency, &r.UnderlyingCcy, &r.Bid, &r.Ask, &r.BidSize, &r.AskSize, &r.Last, &r.Volume,
			&r.UnderlyingPrice, &r.IvBid, &r.IvMid, &r.IvAsk, &r.Iv, &r.Delta, &r.Gamma, &r.ThetaPerCalendarDay,
			&r.VegaPerVolPoint, &r.OpenInterest, &oi); err != nil {
			return err
		}
		r.SessionDay = contracts.Day(sessionDay.Format(time.DateOnly))
		r.ReceivedAt = r.ReceivedAt.UTC()
		if oi != nil {
			od := contracts.Day(oi.Format(time.DateOnly))
			r.OiAsofDate = &od
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("option_day: select: %w", err))
	}
	return out, nil
}

// Close releases the connections.
func (d *dayReader) Close() error { return d.db.Close() }
