package events

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

var _ fundkey.DividendUpserter = (*dividendStore)(nil)

// Arguments of upsertDividendRowSQL: instrument_id, ex_date, issuer, amount, currency, source, received_at, written_by.
// A stored row is replaced only when issuer, amount, currency or source differs; received_at and the key are never set
// by an update.
const upsertDividendRowSQL = `INSERT INTO obs.dividend_expectation
 (instrument_id, ex_date, issuer, amount, currency, source, received_at, written_by)
VALUES ($1, $2::text::date, $3, $4, $5, $6, $7, $8)
ON CONFLICT (instrument_id, ex_date) DO UPDATE SET
 issuer = EXCLUDED.issuer, amount = EXCLUDED.amount, currency = EXCLUDED.currency, source = EXCLUDED.source,
 updated_at = now(), written_by = $8
WHERE (obs.dividend_expectation.issuer, obs.dividend_expectation.amount, obs.dividend_expectation.currency,
       obs.dividend_expectation.source)
 IS DISTINCT FROM (EXCLUDED.issuer, EXCLUDED.amount, EXCLUDED.currency, EXCLUDED.source)
RETURNING instrument_id`

// Arguments of the two answer statements: instrument_id, session_day, issuer, status, error_code, source, currency,
// ex_date, past_12m, next_12m, received_at, written_by. The rank rule of Decision 5: error 1; no_value and unsupported
// 2; present 3. past_12m and next_12m never lose a present value to a missing one.
const (
	dividendCheckInsert = `INSERT INTO obs.dividend_check
 (instrument_id, session_day, issuer, status, error_code, source, currency, ex_date, past_12m, next_12m, received_at,
  written_by)
VALUES ($1, $2::text::date, $3, $4, $5, $6, $7, $8::text::date, $9, $10, $11, $12)
ON CONFLICT (instrument_id, session_day) `

	upsertDividendCheckSQL = dividendCheckInsert + `DO UPDATE SET
 issuer = EXCLUDED.issuer, status = EXCLUDED.status, error_code = EXCLUDED.error_code, source = EXCLUDED.source,
 currency = EXCLUDED.currency, ex_date = EXCLUDED.ex_date,
 past_12m = COALESCE(NULLIF(EXCLUDED.past_12m, ''), obs.dividend_check.past_12m),
 next_12m = COALESCE(NULLIF(EXCLUDED.next_12m, ''), obs.dividend_check.next_12m),
 updated_at = now(), written_by = $12
WHERE (CASE EXCLUDED.status WHEN 'error' THEN 1 WHEN 'present' THEN 3 ELSE 2 END)
   >= (CASE obs.dividend_check.status WHEN 'error' THEN 1 WHEN 'present' THEN 3 ELSE 2 END)
 AND (obs.dividend_check.issuer, obs.dividend_check.status, obs.dividend_check.error_code, obs.dividend_check.source,
      obs.dividend_check.currency, obs.dividend_check.ex_date, obs.dividend_check.past_12m, obs.dividend_check.next_12m)
 IS DISTINCT FROM
     (EXCLUDED.issuer, EXCLUDED.status, EXCLUDED.error_code, EXCLUDED.source, EXCLUDED.currency, EXCLUDED.ex_date,
      COALESCE(NULLIF(EXCLUDED.past_12m, ''), obs.dividend_check.past_12m),
      COALESCE(NULLIF(EXCLUDED.next_12m, ''), obs.dividend_check.next_12m))
RETURNING instrument_id`

	// replayDividendCheckSQL writes the answer only where none is stored for its key.
	replayDividendCheckSQL = dividendCheckInsert + `DO NOTHING RETURNING instrument_id`
)

// dividendStmts is the statements of the rows in order: for a present row its data row, then the answer of every row.
// owner[i] is the index of the row statement i belongs to. Attempt is not an argument of any statement.
func dividendStmts(rows []contracts.DividendExpectation, writtenBy, answerSQL string) (stmts []stmt, owner []int, err error) {
	for i, r := range rows {
		if r.InstrumentID == 0 {
			return nil, nil, permanent(fmt.Errorf("events: a dividend row needs an instrument id"))
		}
		var exDate *string
		if string(r.Status) == "present" {
			if r.NextExDate == nil || r.NextAmount == nil {
				return nil, nil, permanent(fmt.Errorf("events: a present dividend row of %d needs an ex-date and an amount", r.InstrumentID))
			}
			exDate = dayText(r.NextExDate)
			stmts = append(stmts, stmt{upsertDividendRowSQL, []any{
				int64(r.InstrumentID), *exDate, r.Issuer, *r.NextAmount, r.Currency, string(r.Source), r.ReceivedAt.UTC(), writtenBy}})
			owner = append(owner, i)
		}
		stmts = append(stmts, stmt{answerSQL, []any{
			int64(r.InstrumentID), r.SessionDay.Format(time.DateOnly), r.Issuer, string(r.Status), r.ErrorCode,
			string(r.Source), r.Currency, exDate, r.Past12M, r.Next12M, r.ReceivedAt.UTC(), writtenBy}})
		owner = append(owner, i)
	}
	return stmts, owner, nil
}

func (s *dividendStore) upsert(ctx context.Context, rows []contracts.DividendExpectation, writtenBy, answerSQL, what string) (fundkey.WriteCount, error) {
	if len(rows) == 0 {
		return fundkey.WriteCount{}, nil
	}
	stmts, owner, err := dividendStmts(rows, writtenBy, answerSQL)
	if err != nil {
		return fundkey.WriteCount{}, err
	}
	counts, err := s.db.Upsert(ctx, stmts)
	if err != nil {
		return fundkey.WriteCount{}, upsertErr(what, err)
	}
	w := perRow(len(rows), owner, counts)
	return fundkey.WriteCount{Written: w, Unchanged: len(rows) - w}, nil
}

// UpsertDividends stores the rows in one transaction: a present row writes its data row by (instrument, ex-date),
// every row writes its answer by (instrument, session day), in the order given. Written counts the rows that inserted
// or replaced a stored row, Unchanged the others. An empty list makes no call; a constraint violation is Permanent.
func (s *dividendStore) UpsertDividends(ctx context.Context, rows []contracts.DividendExpectation, writtenBy string) (fundkey.WriteCount, error) {
	return s.upsert(ctx, rows, writtenBy, upsertDividendCheckSQL, "upsert dividends")
}

// ReplayDividends is UpsertDividends for a run of a past day: the data rows are upserted, an answer is written only
// where none is stored for its key.
func (s *dividendStore) ReplayDividends(ctx context.Context, rows []contracts.DividendExpectation, writtenBy string) (fundkey.WriteCount, error) {
	return s.upsert(ctx, rows, writtenBy, replayDividendCheckSQL, "replay dividends")
}
