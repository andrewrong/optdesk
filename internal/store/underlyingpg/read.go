package underlyingpg

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// The reads go through the view rd.underlying_r1 and nothing else: a page role and the runner have no grant on the
// table. The newest row per key is DISTINCT ON the key, newest first, ties by id. con_id is NULL until resolved and
// reads as 0.
const (
	columns = `id, name_key, symbol, market, kind, exchange, currency, name, issuer, COALESCE(con_id, 0), status, reason, ` +
		`options, fundamentals, buy_price, sale_price, report_date_override, note, set_at`

	newestPerKey = `(SELECT DISTINCT ON (name_key) ` + columns + ` FROM rd.underlying_r1 ` +
		`ORDER BY name_key, set_at DESC, id DESC) c`

	currentSQL = `SELECT * FROM ` + newestPerKey + ` ORDER BY name_key`
	pendingSQL = `SELECT * FROM ` + newestPerKey +
		` WHERE status = 'pending' OR (status = 'unresolved' AND set_at < $1) ORDER BY (note = $2) DESC, name_key`
	historySQL = `SELECT ` + columns + ` FROM rd.underlying_r1 ORDER BY set_at DESC, id DESC LIMIT $1`
	// The newest row of one key, read by Settle in its transaction.
	newestSQL = `SELECT ` + columns + ` FROM rd.underlying_r1 WHERE name_key = $1 ORDER BY set_at DESC, id DESC LIMIT 1`
)

// scanRow reads the columns of columns into a Row.
func scanRow(sc scanner) (underlying.Row, error) {
	var r underlying.Row
	err := sc.Scan(&r.ID, &r.Key, &r.Symbol, &r.Market, &r.Kind, &r.Exchange, &r.Currency, &r.Name, &r.Issuer,
		&r.ConID, &r.Status, &r.Reason, &r.Options, &r.Fundamentals, &r.BuyPrice, &r.SalePrice,
		&r.ReportDateOverride, &r.Note, &r.SetAt)
	return r, err
}

func selectRows(ctx context.Context, q querier, sql string, args ...any) ([]underlying.Row, error) {
	out := []underlying.Row{}
	err := q.Select(ctx, sql, args, func(sc scanner) error {
		r, err := scanRow(sc)
		if err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	return out, err
}

// Current returns the newest row of every key, sorted by Key; none gives an empty slice.
func (s *Store) Current(ctx context.Context) ([]underlying.Row, error) {
	rows, err := selectRows(ctx, s.db, currentSQL)
	if err != nil {
		return nil, retryable(fmt.Errorf("underlyingpg: current: %w", err))
	}
	return rows, nil
}

// History returns the newest versions first, at most limit; a limit below 1 gives an empty slice.
func (s *Store) History(ctx context.Context, limit int) ([]underlying.Row, error) {
	if limit < 1 {
		return []underlying.Row{}, nil
	}
	rows, err := selectRows(ctx, s.db, historySQL, limit)
	if err != nil {
		return nil, retryable(fmt.Errorf("underlyingpg: history: %w", err))
	}
	return rows, nil
}

// Pending returns the newest rows that are pending, or unresolved with SetAt before retryBefore; rows with Note
// NoteHeld come first, then Key order.
func (s *Store) Pending(ctx context.Context, retryBefore time.Time) ([]underlying.Row, error) {
	rows, err := selectRows(ctx, s.db, pendingSQL, retryBefore, underlying.NoteHeld)
	if err != nil {
		return nil, retryable(fmt.Errorf("underlyingpg: pending: %w", err))
	}
	return rows, nil
}
