package account

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

// The Reader reads the views of the schema rd and never a base table; dates and decimals come back as text.
const (
	// selectPositionSQL arguments: alias, session day.
	selectPositionSQL = `SELECT capture_id, alias, captured_at, session_day::text, symbol, sec_type, exchange, ` +
		`currency, expiry, strike, multiplier, "right", con_id, quantity::text, avg_cost::text ` +
		`FROM rd.account_position_r1 WHERE alias = $1 AND session_day = $2::date ORDER BY capture_id, con_id`
	// selectValueSQL arguments: alias, session day.
	selectValueSQL = `SELECT capture_id, alias, captured_at, session_day::text, tag, value::text, currency ` +
		`FROM rd.account_value_r1 WHERE alias = $1 AND session_day = $2::date ORDER BY capture_id, tag, currency`
	// selectExecutionSQL arguments: alias, first session day, last session day.
	selectExecutionSQL = `SELECT alias, exec_id, perm_id, executed_at, session_day::text, symbol, sec_type, expiry, ` +
		`strike, "right", multiplier, currency, side, quantity::text, price::text, commission::text, commission_currency ` +
		`FROM rd.account_execution_r1 WHERE alias = $1 AND session_day >= $2::date AND session_day <= $3::date ` +
		`ORDER BY exec_id`
)

// Positions returns the position rows of alias on session day day, ordered by capture id and con id.
func (s *accountStore) Positions(ctx context.Context, alias, day string) ([]store.Position, error) {
	if err := checkDay(day); err != nil {
		return nil, err
	}
	out := []store.Position{}
	err := s.db.Select(ctx, selectPositionSQL, []any{alias, day}, func(sc scanner) error {
		var p store.Position
		if err := sc.Scan(&p.CaptureID, &p.Alias, &p.CapturedAt, &p.SessionDay, &p.Symbol, &p.SecType, &p.Exchange,
			&p.Currency, &p.Expiry, &p.Strike, &p.Multiplier, &p.Right, &p.ConID, &p.Quantity, &p.AvgCost); err != nil {
			return err
		}
		p.CapturedAt = p.CapturedAt.UTC()
		out = append(out, p)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("account: positions: %w", err))
	}
	return out, nil
}

// Values returns the value rows of alias on session day day, ordered by capture id, tag and currency.
func (s *accountStore) Values(ctx context.Context, alias, day string) ([]store.AccountValue, error) {
	if err := checkDay(day); err != nil {
		return nil, err
	}
	out := []store.AccountValue{}
	err := s.db.Select(ctx, selectValueSQL, []any{alias, day}, func(sc scanner) error {
		var v store.AccountValue
		if err := sc.Scan(&v.CaptureID, &v.Alias, &v.CapturedAt, &v.SessionDay, &v.Tag, &v.Value, &v.Currency); err != nil {
			return err
		}
		v.CapturedAt = v.CapturedAt.UTC()
		out = append(out, v)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("account: values: %w", err))
	}
	return out, nil
}

// Executions returns the executions of alias with session day from to to, inclusive, ordered by exec id.
func (s *accountStore) Executions(ctx context.Context, alias, from, to string) ([]store.Execution, error) {
	if err := checkDay(from); err != nil {
		return nil, err
	}
	if err := checkDay(to); err != nil {
		return nil, err
	}
	out := []store.Execution{}
	err := s.db.Select(ctx, selectExecutionSQL, []any{alias, from, to}, func(sc scanner) error {
		var e store.Execution
		var at time.Time
		if err := sc.Scan(&e.Alias, &e.ExecID, &e.PermID, &at, &e.SessionDay, &e.Symbol, &e.SecType, &e.Expiry,
			&e.Strike, &e.Right, &e.Multiplier, &e.Currency, &e.Side, &e.Quantity, &e.Price, &e.Commission,
			&e.CommissionCurrency); err != nil {
			return err
		}
		e.ExecutedAt = at.UTC()
		out = append(out, e)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("account: executions: %w", err))
	}
	return out, nil
}
