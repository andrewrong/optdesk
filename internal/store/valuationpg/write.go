package valuationpg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/andrewrong/optdesk/internal/store/valuation"
)

// The statements the writer sends. The tables are insert-only: none of them updates or deletes, and set_at,
// entered_at and effective_at are the database's now(). Dates and json travel as text and are cast in the statement.
const (
	insertProfileSQL = `INSERT INTO valuation.company_profile ` +
		`(company_key, name, symbol, currency, r_market, pool, pool_date, note) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7::text::date, $8)`
	insertReviewSQL = `INSERT INTO valuation.review ` +
		`(review_id, company_key, type, version_date, fiscal_period_end, report_date, status, source, v, p, ` +
		`trigger_type, earnings_base, base_basis, forward_factor, locked_multiple, method_results, adjustments, ` +
		`scenarios, change_memo, reaction, moat, understand, management, financial, reasons, report_text, void_reason) ` +
		`VALUES ($1, $2, $3, $4::text::date, $5::text::date, $6::text::date, $7, $8, $9, $10, $11, $12, $13, $14, $15, ` +
		`$16::text::jsonb, $17::text::jsonb, $18::text::jsonb, $19, $20, $21, $22, $23, $24, $25::text::jsonb, $26, $27)`
	reviewStatusSQL = `SELECT ` +
		`COALESCE(bool_or(status = 'effective'), false), COALESCE(bool_or(status = 'void'), false) ` +
		`FROM valuation.review WHERE review_id = $1`
	insertRVersionSQL = `INSERT INTO valuation.r_version ` +
		`(effective_date, yield_used, r_us, r_jp, r_hkcn, ai_switch, ai_expiry, note) ` +
		`VALUES ($1::text::date, $2, $3, $4, $5, $6, $7, $8)`
	insertRTableSQL = `INSERT INTO valuation.r_table (effective_date, yield_up_to, us, jp, hkcn) ` +
		`VALUES ($1::text::date, $2, $3, $4, $5)`
	insertMarginSQL = `INSERT INTO valuation.margin_table (effective_date, p_from, m, w, h, s) ` +
		`VALUES ($1::text::date, $2, $3, $4, $5, $6)`
)

// statements lists every SQL string the writer sends.
var statements = []string{
	insertProfileSQL, insertReviewSQL, reviewStatusSQL, insertRVersionSQL, insertRTableSQL, insertMarginSQL,
}

// The names of the two unique partial indexes of migration 0052; a violation of one means another writer closed the
// review_id between the status read and the insert.
const (
	effectiveIndex = "review_effective_uq"
	voidIndex      = "review_void_uq"
)

// inTx runs fn in one transaction: committed when fn returns nil, rolled back otherwise.
func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// nullText is "" as SQL NULL.
func nullText(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullJSON is the json text of v, or SQL NULL when v is empty (a nil slice or map).
func nullJSON(v any, empty bool) (any, error) {
	if empty {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// PutProfile inserts one profile row.
func (s *Store) PutProfile(ctx context.Context, p valuation.Profile) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, insertProfileSQL, p.CompanyKey, p.Name, p.Symbol, p.Currency, p.RMarket, p.Pool,
			nullText(string(p.PoolDate)), p.Note)
		return err
	})
	if err != nil {
		return fmt.Errorf("valuationpg: put profile: %w", err)
	}
	return nil
}

// PutReview inserts one review row after reading, in the same transaction, whether its ReviewID is closed. A ReviewID
// with an effective row refuses every row but its one void row (ErrClosed); a void row for a ReviewID with no
// effective row is refused with ErrNotEffective. A second void row is ErrClosed. Source "" is written as owner.
func (s *Store) PutReview(ctx context.Context, r valuation.Review) error {
	args, err := reviewArgs(r)
	if err != nil {
		return fmt.Errorf("valuationpg: put review: %w", err)
	}
	err = s.inTx(ctx, func(tx *sql.Tx) error {
		var hasEffective, hasVoid bool
		if err := tx.QueryRowContext(ctx, reviewStatusSQL, r.ReviewID).Scan(&hasEffective, &hasVoid); err != nil {
			return err
		}
		if r.Status == "void" && !hasEffective {
			return valuation.ErrNotEffective
		}
		if (hasEffective && r.Status != "void") || hasVoid {
			return valuation.ErrClosed
		}
		_, err := tx.ExecContext(ctx, insertReviewSQL, args...)
		return err
	})
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" && (pe.ConstraintName == effectiveIndex || pe.ConstraintName == voidIndex) {
			return valuation.ErrClosed
		}
		if errors.Is(err, valuation.ErrClosed) || errors.Is(err, valuation.ErrNotEffective) {
			return err
		}
		return fmt.Errorf("valuationpg: put review: %w", err)
	}
	return nil
}

type adjustmentJSON struct {
	Percent string `json:"percent"`
	Reason  string `json:"reason"`
}

type scenarioJSON struct {
	Label       string `json:"label"`
	Probability string `json:"probability"`
	EPS         string `json:"eps"`
	Multiple    string `json:"multiple"`
}

func reviewArgs(r valuation.Review) ([]any, error) {
	adj := make([]adjustmentJSON, len(r.Adjustments))
	for i, a := range r.Adjustments {
		adj[i] = adjustmentJSON(a)
	}
	sc := make([]scenarioJSON, len(r.Scenarios))
	for i, x := range r.Scenarios {
		sc[i] = scenarioJSON(x)
	}
	methods, err := nullJSON(r.MethodResults, len(r.MethodResults) == 0)
	if err != nil {
		return nil, err
	}
	adjustments, err := nullJSON(adj, len(adj) == 0)
	if err != nil {
		return nil, err
	}
	scenarios, err := nullJSON(sc, len(sc) == 0)
	if err != nil {
		return nil, err
	}
	reasons, err := nullJSON(r.Reasons, len(r.Reasons) == 0)
	if err != nil {
		return nil, err
	}
	source := r.Source
	if source == "" {
		source = "owner"
	}
	return []any{
		r.ReviewID, r.CompanyKey, nullText(r.Type), nullText(string(r.VersionDate)),
		nullText(string(r.FiscalPeriodEnd)), nullText(string(r.ReportDate)), r.Status, source,
		nullText(r.V), nullText(r.P), nullText(r.Trigger), nullText(r.EarningsBase), nullText(r.BaseBasis),
		nullText(r.ForwardFactor), nullText(r.LockedMultiple), methods, adjustments, scenarios,
		nullText(r.ChangeMemo), nullText(r.Reaction), nullText(r.Moat), nullText(r.Understand),
		nullText(r.Management), nullText(r.Financial), reasons, nullText(r.ReportText), nullText(r.VoidReason),
	}, nil
}

// PutRVersion inserts one R version row.
func (s *Store) PutRVersion(ctx context.Context, v valuation.RVersion) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, insertRVersionSQL, string(v.EffectiveDate), v.YieldUsed, v.RUS, v.RJP, v.RHKCN,
			v.AISwitch, v.AIExpiry, v.Note)
		return err
	})
	if err != nil {
		return fmt.Errorf("valuationpg: put r version: %w", err)
	}
	return nil
}

// PutRTable inserts every bucket of the version in one transaction, so the rows share set_at (the transaction's now()).
func (s *Store) PutRTable(ctx context.Context, t valuation.RTableVersion) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, r := range t.Rows {
			if _, err := tx.ExecContext(ctx, insertRTableSQL, string(t.EffectiveDate), r.YieldUpTo, r.US, r.JP, r.HKCN); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("valuationpg: put r table: %w", err)
	}
	return nil
}

// PutMarginTable inserts every bucket of the version in one transaction, so the rows share set_at.
func (s *Store) PutMarginTable(ctx context.Context, t valuation.MarginTableVersion) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, r := range t.Rows {
			if _, err := tx.ExecContext(ctx, insertMarginSQL, string(t.EffectiveDate), r.PFrom, r.M, r.W, r.H, r.S); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("valuationpg: put margin table: %w", err)
	}
	return nil
}
