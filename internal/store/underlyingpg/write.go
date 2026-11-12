package underlyingpg

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Plain INSERTs: the table is insert-only and set_at is the database's now(). The lock serializes the writers of the
// table for the length of one transaction, so a check of the newest row and the insert after it cannot interleave
// with another writer.
const (
	lockSQL   = `SELECT pg_advisory_xact_lock(hashtext('settings.underlying'))`
	insertSQL = `INSERT INTO settings.underlying (name_key, symbol, market, kind, exchange, currency, name, issuer, ` +
		`con_id, status, reason, options, fundamentals, buy_price, sale_price, report_date_override, note) ` +
		`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`
	maxReason = 200
)

func insertRow(ctx context.Context, q querier, r underlying.Row) error {
	var conID any // NULL until resolved
	if r.ConID > 0 {
		conID = r.ConID
	}
	return q.Exec(ctx, insertSQL, r.Key, r.Symbol, r.Market, r.Kind, r.Exchange, r.Currency, r.Name, r.Issuer,
		conID, r.Status, r.Reason, r.Options, r.Fundamentals, r.BuyPrice, r.SalePrice, r.ReportDateOverride, r.Note)
}

// Apply validates every add and edit against the newest rows, then inserts one row for each that changes
// something, all in one transaction, and returns the number of rows inserted. A refused value returns an
// *underlying.InvalidError and writes nothing. Adds are checked before edits; an edit may name a name added in the
// same change.
func (s *Store) Apply(ctx context.Context, c underlying.Change) (int, error) {
	if len(c.Adds) == 0 && len(c.Edits) == 0 {
		return 0, nil
	}
	n := 0
	err := s.db.InTx(ctx, func(q querier) error {
		n = 0
		if err := q.Exec(ctx, lockSQL); err != nil {
			return err
		}
		rows, err := selectRows(ctx, q, currentSQL)
		if err != nil {
			return err
		}
		cur := map[string]underlying.Row{}
		for _, r := range rows {
			cur[r.Key] = r
		}
		var out []underlying.Row
		for _, a := range c.Adds {
			r, err := ValidateAdd(a, func(k string) bool { _, ok := cur[k]; return ok })
			if err != nil {
				return err
			}
			cur[r.Key] = r
			out = append(out, r)
		}
		for _, e := range c.Edits {
			old, ok := cur[e.Key]
			if !ok {
				return invalid("key", "unknown name "+e.Key)
			}
			next, changed, err := ValidateEdit(old, e)
			if err != nil {
				return err
			}
			if changed {
				cur[e.Key] = next
				out = append(out, next)
			}
		}
		for _, r := range out {
			if err := insertRow(ctx, q, r); err != nil {
				return err
			}
		}
		n = len(out)
		return nil
	})
	if err != nil {
		var ie underlying.InvalidError
		if errors.As(err, &ie) {
			return 0, ie
		}
		return 0, classify(fmt.Errorf("underlyingpg: apply: %w", err))
	}
	return n, nil
}

// Settle inserts the outcome only if the newest row of o.Key still has ID o.AfterID and status pending or
// unresolved; otherwise it writes nothing and returns false with no error. A ConID above 0 makes the row active;
// otherwise it is unresolved with the reason (cut to 200 characters).
func (s *Store) Settle(ctx context.Context, o underlying.Outcome) (bool, error) {
	written := false
	err := s.db.InTx(ctx, func(q querier) error {
		written = false
		if err := q.Exec(ctx, lockSQL); err != nil {
			return err
		}
		rows, err := selectRows(ctx, q, newestSQL, o.Key)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		r := rows[0]
		if r.ID != o.AfterID || (r.Status != underlying.StatusPending && r.Status != underlying.StatusUnresolved) {
			return nil
		}
		if o.ConID > 0 {
			r.ConID, r.Status, r.Reason = o.ConID, underlying.StatusActive, ""
		} else {
			r.ConID, r.Status, r.Reason = 0, underlying.StatusUnresolved, cut(o.Reason, maxReason)
		}
		if err := insertRow(ctx, q, r); err != nil {
			return err
		}
		written = true
		return nil
	})
	if err != nil {
		return false, classify(fmt.Errorf("underlyingpg: settle: %w", err))
	}
	return written, nil
}

// cut shortens s to at most n characters.
func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

var _ underlying.HeldAdder = (*Store)(nil)

// Check applies the rules of Add without a write: the market of the currency, then the symbol rules of that market.
// It returns the key, market:SYMBOL.
func (s *Store) Check(symbol, currency string) (string, error) {
	r, err := heldRow(symbol, currency)
	if err != nil {
		return "", err
	}
	return r.Key, nil
}

// heldRow is the row Add writes for symbol in the market of currency.
func heldRow(symbol, currency string) (underlying.Row, error) {
	market, ok := marketOfCurrency[currency]
	if !ok {
		return underlying.Row{}, invalid("currency", "no market for currency "+currency)
	}
	return ValidateAdd(underlying.Add{
		Symbol: symbol, Market: market, Kind: underlying.KindStock, Options: true, Note: underlying.NoteHeld,
	}, nil)
}

// Add inserts one pending held row under the table lock, unless a row of the symbol exists in any status under
// either market; then it writes nothing and returns false.
func (s *Store) Add(ctx context.Context, symbol, currency string) (bool, error) {
	row, err := heldRow(symbol, currency)
	if err != nil {
		return false, err
	}
	added := false
	err = s.db.InTx(ctx, func(q querier) error {
		added = false
		if err := q.Exec(ctx, lockSQL); err != nil {
			return err
		}
		rows, err := selectRows(ctx, q, currentSQL)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if r.Symbol == symbol {
				return nil
			}
		}
		if err := insertRow(ctx, q, row); err != nil {
			return err
		}
		added = true
		return nil
	})
	if err != nil {
		return false, classify(fmt.Errorf("underlyingpg: add: %w", err))
	}
	return added, nil
}
