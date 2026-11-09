package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

var _ dividend.OpenFunc = OpenDividend

// No conflict target, as for earnings: the primary key is the only unique index, so DO NOTHING acts on
// (session_day, instrument_id, attempt). Dates travel as text and are cast in SQL.
const (
	insertDividendSQL = `INSERT INTO obs.dividend_expectation
 (session_day, instrument_id, attempt, issuer, status, past_12m, next_12m, next_amount, next_ex_date, currency,
  error_code, source, received_at)
VALUES ($1::text::date, $2, $3, $4, $5, $6, $7, $8, $9::text::date, $10, $11, $12, $13)
ON CONFLICT DO NOTHING`

	selectDividendSQL = `SELECT session_day, instrument_id, attempt, issuer, status, past_12m, next_12m, next_amount,
 next_ex_date, currency, error_code, source, received_at
FROM rd.dividend_expectation_r1 WHERE instrument_id = $1 AND session_day >= $2::text::date AND session_day <= $3::text::date
ORDER BY session_day, attempt`
)

type dividendStore struct{ db database }

// OpenDividend connects to the database named by dsn and returns the dividend store. It applies no migration.
// The DSN is that of ot_w_events (insert and read) or ot_ro_pages (read only).
func OpenDividend(ctx context.Context, dsn string) (dividend.Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, permanent(fmt.Errorf("events: open: %w", err))
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, retryable(fmt.Errorf("events: ping: %w", err))
	}
	return &dividendStore{db: pgDB{pool: pool}}, nil
}

// Close releases the connections.
func (s *dividendStore) Close() error { return s.db.Close() }

// InsertDividend inserts the row; a row whose key exists stays as it is and is no error.
func (s *dividendStore) InsertDividend(ctx context.Context, r contracts.DividendExpectation) error {
	if r.InstrumentID == 0 || r.Attempt < 1 {
		return permanent(fmt.Errorf("events: a dividend row needs an instrument id and an attempt from 1"))
	}
	err := s.db.Exec(ctx, insertDividendSQL,
		r.SessionDay.Format(time.DateOnly), r.InstrumentID, r.Attempt, r.Issuer, string(r.Status),
		r.Past12M, r.Next12M, r.NextAmount, dayText(r.NextExDate), r.Currency, r.ErrorCode, string(r.Source),
		r.ReceivedAt.UTC())
	if err != nil {
		return retryable(fmt.Errorf("events: insert dividend: %w", err))
	}
	return nil
}

// Dividends returns the rows of the instrument from from to to, inclusive, ascending by day, the highest attempt
// of each day only, read from the view.
func (s *dividendStore) Dividends(ctx context.Context, instrumentID int, from, to contracts.Day) ([]contracts.DividendExpectation, error) {
	f, err := time.Parse(time.DateOnly, string(from))
	if err != nil {
		return nil, permanent(fmt.Errorf("events: bad from %q: %w", from, err))
	}
	t, err := time.Parse(time.DateOnly, string(to))
	if err != nil {
		return nil, permanent(fmt.Errorf("events: bad to %q: %w", to, err))
	}
	if t.Before(f) {
		return nil, permanent(fmt.Errorf("events: range %s to %s is reversed", from, to))
	}
	out := []contracts.DividendExpectation{}
	err = s.db.Select(ctx, selectDividendSQL, []any{instrumentID, string(from), string(to)}, func(sc scanner) error {
		r, err := scanDividend(sc)
		if err != nil {
			return err
		}
		// Rows come ordered by day then attempt, so a later row of the same day has the higher attempt.
		if n := len(out); n > 0 && out[n-1].SessionDay.Equal(r.SessionDay.Time) {
			out[n-1] = r
		} else {
			out = append(out, r)
		}
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("events: select dividends: %w", err))
	}
	return out, nil
}

func scanDividend(sc scanner) (contracts.DividendExpectation, error) {
	var r contracts.DividendExpectation
	var day, received time.Time
	var status, source string
	var exDate *time.Time
	if err := sc.Scan(&day, &r.InstrumentID, &r.Attempt, &r.Issuer, &status, &r.Past12M, &r.Next12M, &r.NextAmount,
		&exDate, &r.Currency, &r.ErrorCode, &source, &received); err != nil {
		return r, err
	}
	// The enum types live in the generated package: their UnmarshalJSON refuses a word the dataset does not list.
	for _, e := range []struct {
		word string
		to   any
	}{{status, &r.Status}, {source, &r.Source}} {
		if err := json.Unmarshal([]byte(strconv.Quote(e.word)), e.to); err != nil {
			return r, err
		}
	}
	r.SessionDay = types.SerializableDate{Time: day}
	if exDate != nil {
		r.NextExDate = &types.SerializableDate{Time: *exDate}
	}
	r.ReceivedAt = received.UTC()
	return r, nil
}
