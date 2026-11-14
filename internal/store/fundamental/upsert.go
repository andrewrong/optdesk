package fundamental

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/fundkey"
)

var _ fundkey.FactUpserter = (*factStore)(nil)

// upsertFactSQL is Decision 4 of docs/design/fundamentals-keys.md: the conflict target is the eight key columns
// (doc_id included, so a restating filing stays a second row); a fact read differently replaces the stored one, and
// retrieved_at is not compared, so an equal fact is not rewritten. RETURNING gives a row only for an inserted or
// replaced fact.
const upsertFactSQL = `INSERT INTO obs.fundamental_fact AS f
 (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end,
  currency, value, form, filed, retrieved_at, raw_sha256, written_by)
VALUES ($1, $2, $3, $4, $5, $6, $7::text::date, $8::text::date, $9, $10, $11, $12::text::date, $13, $14, $15)
ON CONFLICT (source, subject, doc_id, taxonomy, concept, unit, period_start_key, period_end) DO UPDATE SET
 currency = EXCLUDED.currency, value = EXCLUDED.value, form = EXCLUDED.form, filed = EXCLUDED.filed,
 retrieved_at = EXCLUDED.retrieved_at, raw_sha256 = EXCLUDED.raw_sha256, updated_at = now(),
 written_by = $15
WHERE (f.currency, f.value, f.form, f.filed, f.raw_sha256)
 IS DISTINCT FROM (EXCLUDED.currency, EXCLUDED.value, EXCLUDED.form, EXCLUDED.filed, EXCLUDED.raw_sha256)
RETURNING f.source`

// returningDB is the optional seam of the upsert, next to database: it runs the statements in one transaction and
// reports for each whether it returned a row.
type returningDB interface {
	ExecReturning(ctx context.Context, stmts []stmt) ([]bool, error)
}

// UpsertFacts stores the rows by their eight-column key in one transaction: a fact whose currency, value, form,
// filed date or raw hash differs from the stored one replaces it, an equal fact is not rewritten. writtenBy names
// the run. An empty list makes no call. A constraint violation is Permanent, every other database error Retryable.
func (s *factStore) UpsertFacts(ctx context.Context, rows []contracts.FundamentalFact, writtenBy string) (fundkey.WriteCount, error) {
	stmts := make([]stmt, 0, len(rows))
	for _, r := range rows {
		if r.Subject == "" || r.DocID == "" || r.Source == "" {
			return fundkey.WriteCount{}, permanent(fmt.Errorf("fundamental: a fact needs source, subject and doc_id"))
		}
		start := noStart
		if r.PeriodStart != nil {
			start = r.PeriodStart.Format(time.DateOnly)
		}
		stmts = append(stmts, stmt{upsertFactSQL, []any{
			string(r.Source), r.Subject, r.DocID, r.Taxonomy, r.Concept, r.Unit, start, r.PeriodEnd.Format(time.DateOnly),
			r.Currency, r.Value, r.Form, r.Filed.Format(time.DateOnly), r.RetrievedAt.UTC(), r.RawSha256, writtenBy,
		}})
	}
	if len(stmts) == 0 {
		return fundkey.WriteCount{}, nil
	}
	db, ok := s.db.(returningDB)
	if !ok {
		return fundkey.WriteCount{}, permanent(fmt.Errorf("fundamental: upsert: the database cannot return rows"))
	}
	changed, err := db.ExecReturning(ctx, stmts)
	if err != nil {
		wrapped := fmt.Errorf("fundamental: upsert: %w", err)
		if isConstraintViolation(err) {
			return fundkey.WriteCount{}, permanent(wrapped)
		}
		return fundkey.WriteCount{}, retryable(wrapped)
	}
	var out fundkey.WriteCount
	for _, c := range changed {
		if c {
			out.Written++
		} else {
			out.Unchanged++
		}
	}
	return out, nil
}

// ExecReturning is the pgx adapter of returningDB.
func (d pgDB) ExecReturning(ctx context.Context, stmts []stmt) ([]bool, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	out := make([]bool, 0, len(stmts))
	for _, s := range stmts {
		rows, err := tx.Query(ctx, s.sql, s.args...)
		if err != nil {
			return nil, err
		}
		got := rows.Next()
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		out = append(out, got)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
