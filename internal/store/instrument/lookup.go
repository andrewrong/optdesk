package instrument

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/store/stockreg"
)

var _ stockreg.StockDirectory = (*instrumentStore)(nil)

const (
	// lookupStockSQL answers at most two ids, enough to see an ambiguity.
	lookupStockSQL = `SELECT i.instrument_id FROM master.instrument i JOIN master.stock s ON s.instrument_id = i.instrument_id
WHERE i.symbol = $1 AND s.market = $2 ORDER BY i.instrument_id LIMIT 2`

	identifierSQL = `SELECT value FROM master.instrument_identifier WHERE instrument_id = $1 AND scheme = $2 AND valid_to IS NULL`
)

// LookupStock returns the id of the stock, etf or index with the symbol in the market (US, JP or EU). ok is false when
// there is none; an empty market never matches (no stock row has one), so it answers false without a statement. Two
// stocks with the symbol and market are a Permanent error naming both ids, never a pick.
func (s *instrumentStore) LookupStock(ctx context.Context, symbol, market string) (int64, bool, error) {
	if symbol == "" || market == "" {
		return 0, false, nil
	}
	var ids []int64
	err := s.db.Select(ctx, lookupStockSQL, []any{symbol, market}, func(r scanner) error {
		var id int64
		if err := r.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	})
	if err != nil {
		return 0, false, asTyped(err, "lookup stock")
	}
	switch len(ids) {
	case 0:
		return 0, false, nil
	case 1:
		return ids[0], true, nil
	}
	return 0, false, permanent(fmt.Errorf("instrument: stocks %d and %d both have symbol %q in market %s", ids[0], ids[1], symbol, market))
}

// Identifier returns the value of the scheme that is valid now for the instrument; ok is false when there is none (the
// source does not have one; nothing is guessed).
func (s *instrumentStore) Identifier(ctx context.Context, instrumentID int64, scheme string) (string, bool, error) {
	var value string
	var found bool
	err := s.db.Select(ctx, identifierSQL, []any{instrumentID, scheme}, func(r scanner) error {
		found = true
		return r.Scan(&value)
	})
	if err != nil {
		return "", false, asTyped(err, "identifier")
	}
	return value, found, nil
}
