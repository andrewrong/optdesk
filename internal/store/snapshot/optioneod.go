package snapshot

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenOptionEodFunc = OpenOptionEod

// insertOptionEodSQL appends one row; a retry of the same capture is a conflict on the primary key and adds no row.
// Arguments, in order: session_day, capture_id, instrument_id, underlying_id, generation, market_data_type,
// received_at, event_ts, status, error_code, currency, snapshot_kind, terms_version_id, bid, ask, bid_size,
// ask_size, last, volume, open_interest, oi_asof_date, underlying_price, underlying_ccy, underlying_event_ts,
// iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma, theta_per_calendar_day,
// vega_per_vol_point.
const insertOptionEodSQL = `INSERT INTO obs.option_eod (session_day, capture_id, instrument_id, underlying_id, ` +
	`generation, market_data_type, received_at, event_ts, status, error_code, currency, snapshot_kind, ` +
	`terms_version_id, bid, ask, bid_size, ask_size, last, volume, open_interest, oi_asof_date, ` +
	`underlying_price, underlying_ccy, underlying_event_ts, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, ` +
	`iv_decimal, delta, gamma, theta_per_calendar_day, vega_per_vol_point) ` +
	`VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, ` +
	`$21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32) ON CONFLICT DO NOTHING`

// selectOptionEodSQL reads the view of the table. Arguments: underlying_id, first day, last day.
const selectOptionEodSQL = `SELECT session_day, capture_id::text, instrument_id, underlying_id, generation, ` +
	`market_data_type, received_at, event_ts, status, error_code, currency, snapshot_kind, terms_version_id, ` +
	`bid, ask, bid_size, ask_size, last, volume, open_interest, oi_asof_date, underlying_price, underlying_ccy, ` +
	`underlying_event_ts, iv_bid_decimal, iv_mid_decimal, iv_ask_decimal, iv_decimal, delta, gamma, ` +
	`theta_per_calendar_day, vega_per_vol_point ` +
	`FROM rd.option_eod_r1 ` +
	`WHERE underlying_id = $1 AND session_day >= $2 AND session_day <= $3 ` +
	`ORDER BY session_day, instrument_id, capture_id`

type optionEodStore struct{ db database }

// OpenOptionEod connects to the database named by dsn and returns the option end-of-day store.
// It applies no migration: the schema is made by goose with the role ot_owner.
func OpenOptionEod(ctx context.Context, dsn string) (store.OptionEodStore, error) {
	db, err := openDB(ctx, "option_eod", dsn)
	if err != nil {
		return nil, err
	}
	return &optionEodStore{db: db}, nil
}

// InsertOptionEod appends the rows in one transaction; a row whose key (session_day, capture_id, instrument_id)
// exists is left as it is. A -1, NaN or infinity in any numeric column is stored as NULL; the status of the row
// and the other columns are kept. An empty input makes no database call.
func (s *optionEodStore) InsertOptionEod(ctx context.Context, rows []contracts.OptionEod) error {
	if len(rows) == 0 {
		return nil
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		for _, r := range rows {
			if err := tx.Exec(ctx, insertOptionEodSQL, optionEodArgs(r)...); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return retryable(fmt.Errorf("option_eod: insert: %w", err))
	}
	return nil
}

// optionEodArgs lists the arguments of insertOptionEodSQL for r, every numeric column screened.
func optionEodArgs(r contracts.OptionEod) []any {
	var oi *time.Time
	if r.OiAsofDate != nil {
		oi = &r.OiAsofDate.Time
	}
	return []any{
		r.SessionDay.Time, r.CaptureID, r.InstrumentID, r.UnderlyingID, r.Generation, r.MarketDataType,
		r.ReceivedAt, r.EventTs, r.Status, r.ErrorCode, r.Currency, r.SnapshotKind, r.TermsVersionID,
		screenSentinel(r.Bid), screenSentinel(r.Ask), screenSentinel(r.BidSize), screenSentinel(r.AskSize),
		screenSentinel(r.Last), screenSentinel(r.Volume), screenSentinel(r.OpenInterest), oi,
		screenSentinel(r.UnderlyingPrice), r.UnderlyingCcy, r.UnderlyingEventTs,
		screenSentinel(r.IvBidDecimal), screenSentinel(r.IvMidDecimal), screenSentinel(r.IvAskDecimal),
		screenSentinel(r.IvDecimal), screenSentinel(r.Delta), screenSentinel(r.Gamma),
		screenSentinel(r.ThetaPerCalendarDay), screenSentinel(r.VegaPerVolPoint),
	}
}

// OptionEod returns the rows of underlyingID with session_day between from and to, both inclusive, ordered by
// session_day, instrument_id and capture_id: one picked row per session_day and instrument_id (see
// pickOptionEod). No row in the range gives an empty slice and no error.
// Instants are returned in UTC.
func (s *optionEodStore) OptionEod(ctx context.Context, underlyingID int64, from, to contracts.Day) ([]contracts.OptionEod, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []contracts.OptionEod{}
	err = s.db.Select(ctx, selectOptionEodSQL, []any{underlyingID, f, t}, func(sc scanner) error {
		var r contracts.OptionEod
		var day time.Time
		var oi *time.Time
		if err := sc.Scan(&day, &r.CaptureID, &r.InstrumentID, &r.UnderlyingID, &r.Generation, &r.MarketDataType,
			&r.ReceivedAt, &r.EventTs, &r.Status, &r.ErrorCode, &r.Currency, &r.SnapshotKind, &r.TermsVersionID,
			&r.Bid, &r.Ask, &r.BidSize, &r.AskSize, &r.Last, &r.Volume, &r.OpenInterest, &oi,
			&r.UnderlyingPrice, &r.UnderlyingCcy, &r.UnderlyingEventTs,
			&r.IvBidDecimal, &r.IvMidDecimal, &r.IvAskDecimal, &r.IvDecimal, &r.Delta, &r.Gamma,
			&r.ThetaPerCalendarDay, &r.VegaPerVolPoint); err != nil {
			return err
		}
		r.SessionDay = types.SerializableDate{Time: day}
		if oi != nil {
			r.OiAsofDate = &types.SerializableDate{Time: *oi}
		}
		r.ReceivedAt = r.ReceivedAt.UTC()
		r.EventTs = utc(r.EventTs)
		r.UnderlyingEventTs = utc(r.UnderlyingEventTs)
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("option_eod: select: %w", err))
	}
	return pickOptionEod(out), nil
}

// optionEodOK is the status of a row that holds a quote.
const optionEodOK = 1

type optionEodKey struct {
	day        time.Time
	instrument int
}

// optionEodBetter reports whether a beats b. It orders the candidates of one key, the greater the better: an ok row beats a non-ok row, a
// session row (a legacy eod row included) beats an off_session row, then the greater received_at, then the
// greater capture_id. A non-ok row ranks by received_at and capture_id alone.
func optionEodBetter(a, b contracts.OptionEod) bool {
	if aok, bok := a.Status == optionEodOK, b.Status == optionEodOK; aok != bok {
		return aok
	}
	if a.Status == optionEodOK {
		if as, bs := a.SnapshotKind != "off_session", b.SnapshotKind != "off_session"; as != bs {
			return as
		}
	}
	if !a.ReceivedAt.Equal(b.ReceivedAt) {
		return a.ReceivedAt.After(b.ReceivedAt)
	}
	return a.CaptureID > b.CaptureID
}

// pickOptionEod keeps one row per (session_day, instrument_id) and returns them ordered by session_day,
// instrument_id and capture_id.
func pickOptionEod(rows []contracts.OptionEod) []contracts.OptionEod {
	best := map[optionEodKey]contracts.OptionEod{}
	for _, r := range rows {
		k := optionEodKey{r.SessionDay.Time, r.InstrumentID}
		if cur, ok := best[k]; !ok || optionEodBetter(r, cur) {
			best[k] = r
		}
	}
	out := make([]contracts.OptionEod, 0, len(best))
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.SessionDay.Equal(b.SessionDay.Time) {
			return a.SessionDay.Before(b.SessionDay.Time)
		}
		if a.InstrumentID != b.InstrumentID {
			return a.InstrumentID < b.InstrumentID
		}
		return a.CaptureID < b.CaptureID
	})
	return out
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// Close releases the connections.
func (s *optionEodStore) Close() error { return s.db.Close() }
