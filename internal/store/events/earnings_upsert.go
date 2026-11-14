package events

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

var _ fundkey.EarningsUpserter = (*earningsStore)(nil)

// Arguments of upsertEarningsRowSQL: issuer, period_key, symbol, market, report_date, fiscal_period_end, time_of_day,
// estimate, source, received_at, written_by. A stored row is replaced only when a compared column differs after the
// optional columns (fiscal_period_end, time_of_day, estimate) took COALESCE(NULLIF(new, ”), stored): a missing value
// never blanks a present one. received_at and the key are never set by an update.
const upsertEarningsRowSQL = `INSERT INTO obs.earnings_date
 (issuer, period_key, symbol, market, report_date, fiscal_period_end, time_of_day, estimate, source, received_at, written_by)
VALUES ($1, $2::text::date, $3, $4, $5::text::date, $6::text::date, $7, $8, $9, $10, $11)
ON CONFLICT (issuer, period_key) DO UPDATE SET
 symbol = EXCLUDED.symbol, market = EXCLUDED.market, report_date = EXCLUDED.report_date,
 fiscal_period_end = COALESCE(NULLIF(EXCLUDED.fiscal_period_end::text, '')::date, obs.earnings_date.fiscal_period_end),
 time_of_day = COALESCE(NULLIF(EXCLUDED.time_of_day, ''), obs.earnings_date.time_of_day),
 estimate = COALESCE(NULLIF(EXCLUDED.estimate, ''), obs.earnings_date.estimate),
 source = EXCLUDED.source, updated_at = now(), written_by = $11
WHERE (obs.earnings_date.symbol, obs.earnings_date.market, obs.earnings_date.report_date,
       obs.earnings_date.fiscal_period_end, obs.earnings_date.time_of_day, obs.earnings_date.estimate,
       obs.earnings_date.source)
 IS DISTINCT FROM
      (EXCLUDED.symbol, EXCLUDED.market, EXCLUDED.report_date,
       COALESCE(NULLIF(EXCLUDED.fiscal_period_end::text, '')::date, obs.earnings_date.fiscal_period_end),
       COALESCE(NULLIF(EXCLUDED.time_of_day, ''), obs.earnings_date.time_of_day),
       COALESCE(NULLIF(EXCLUDED.estimate, ''), obs.earnings_date.estimate), EXCLUDED.source)
RETURNING issuer`

// Arguments of the two answer statements: issuer, observed_on, market, symbol, status, error_code, source, period_key,
// received_at, written_by. The rank rule of Decision 5: error 1; absent and unsupported 2; listed 3; an upsert replaces
// a stored answer only when its rank is not lower and a column differs.
const (
	earningsCheckInsert = `INSERT INTO obs.earnings_check
 (issuer, observed_on, market, symbol, status, error_code, source, period_key, received_at, written_by)
VALUES ($1, $2::text::date, $3, $4, $5, $6, $7, $8::text::date, $9, $10)
ON CONFLICT (issuer, observed_on) `

	upsertEarningsCheckSQL = earningsCheckInsert + `DO UPDATE SET
 market = EXCLUDED.market, symbol = EXCLUDED.symbol, status = EXCLUDED.status, error_code = EXCLUDED.error_code,
 source = EXCLUDED.source, period_key = EXCLUDED.period_key, updated_at = now(), written_by = $10
WHERE (CASE EXCLUDED.status WHEN 'error' THEN 1 WHEN 'listed' THEN 3 ELSE 2 END)
   >= (CASE obs.earnings_check.status WHEN 'error' THEN 1 WHEN 'listed' THEN 3 ELSE 2 END)
 AND (obs.earnings_check.market, obs.earnings_check.symbol, obs.earnings_check.status, obs.earnings_check.error_code,
      obs.earnings_check.source, obs.earnings_check.period_key)
 IS DISTINCT FROM
     (EXCLUDED.market, EXCLUDED.symbol, EXCLUDED.status, EXCLUDED.error_code, EXCLUDED.source, EXCLUDED.period_key)
RETURNING issuer`

	// replayEarningsCheckSQL writes the answer only where none is stored for its key.
	replayEarningsCheckSQL = earningsCheckInsert + `DO NOTHING RETURNING issuer`
)

// earningsPeriodKey is Decision 2: the fiscal period end when the source is alphavantage (it names the quarter) and
// has one, the report date for every other source.
func earningsPeriodKey(r contracts.EarningsDate) string {
	if string(r.Source) == "alphavantage" && r.FiscalPeriodEnd != nil {
		return r.FiscalPeriodEnd.Format(time.DateOnly)
	}
	return r.ReportDate.Format(time.DateOnly)
}

// earningsStmts is the statements of the rows in order: for a listed row its data row, then the answer of every row.
// owner[i] is the index of the row statement i belongs to. Attempt is not an argument of any statement.
func earningsStmts(rows []contracts.EarningsDate, writtenBy string, answerSQL string) (stmts []stmt, owner []int, err error) {
	for i, r := range rows {
		if r.Issuer == "" {
			return nil, nil, permanent(fmt.Errorf("events: a row needs an issuer"))
		}
		var key *string
		if string(r.Status) == "listed" {
			if r.ReportDate == nil {
				return nil, nil, permanent(fmt.Errorf("events: a listed row of %s needs a report date", r.Issuer))
			}
			k := earningsPeriodKey(r)
			key = &k
			stmts = append(stmts, stmt{upsertEarningsRowSQL, []any{
				r.Issuer, k, r.Symbol, string(r.Market), r.ReportDate.Format(time.DateOnly), dayText(r.FiscalPeriodEnd),
				r.TimeOfDay, r.Estimate, string(r.Source), r.ReceivedAt.UTC(), writtenBy}})
			owner = append(owner, i)
		}
		stmts = append(stmts, stmt{answerSQL, []any{
			r.Issuer, r.ObservedOn.Format(time.DateOnly), string(r.Market), r.Symbol, string(r.Status), r.ErrorCode,
			string(r.Source), key, r.ReceivedAt.UTC(), writtenBy}})
		owner = append(owner, i)
	}
	return stmts, owner, nil
}

func (s *earningsStore) upsert(ctx context.Context, rows []contracts.EarningsDate, writtenBy, answerSQL, what string) (fundkey.WriteCount, error) {
	if len(rows) == 0 {
		return fundkey.WriteCount{}, nil
	}
	stmts, owner, err := earningsStmts(rows, writtenBy, answerSQL)
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

// UpsertEarnings stores the rows in one transaction: a listed row writes its data row by (issuer, period key), every
// row writes its answer by (issuer, observed day), in the order given. Written counts the rows that inserted or
// replaced a stored row, Unchanged the others. An empty list makes no call; a constraint violation is Permanent.
func (s *earningsStore) UpsertEarnings(ctx context.Context, rows []contracts.EarningsDate, writtenBy string) (fundkey.WriteCount, error) {
	return s.upsert(ctx, rows, writtenBy, upsertEarningsCheckSQL, "upsert earnings")
}

// ReplayEarnings is UpsertEarnings for a run of a past day: the data rows are upserted, an answer is written only
// where none is stored for its key.
func (s *earningsStore) ReplayEarnings(ctx context.Context, rows []contracts.EarningsDate, writtenBy string) (fundkey.WriteCount, error) {
	return s.upsert(ctx, rows, writtenBy, replayEarningsCheckSQL, "replay earnings")
}
