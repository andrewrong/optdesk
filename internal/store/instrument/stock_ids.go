package instrument

import (
	"context"
	"fmt"
	"slices"

	"github.com/andrewrong/optdesk/internal/store"
)

// maxStockNames is the most names one StockIDs call accepts.
const maxStockNames = 500

// stockIDsSQL reads the page view, rows of the type stock only.
const stockIDsSQL = `SELECT instrument_id, instrument_type, name
FROM rd.instrument_r1
WHERE instrument_type = 'stock' AND name = ANY($1)
ORDER BY instrument_id`

// StockIDs reads the instrument ids of stocks by name.
type StockIDs struct{ db database }

// OpenStockIDs connects to the database named by dsn (the line store.DSNEnvPages, role ot_ro_pages) and returns the
// reader. It applies no migration.
func OpenStockIDs(ctx context.Context, dsn string) (*StockIDs, error) {
	db, err := openDB(ctx, "stock ids", dsn)
	if err != nil {
		return nil, err
	}
	return &StockIDs{db: db}, nil
}

// Close releases the connections.
func (s *StockIDs) Close() error { return s.db.Close() }

// StockIDs returns the rows of type stock whose name is in names, ordered by InstrumentID; an unknown name is absent
// and a name held by two rows gives two rows. An empty names makes no database call; more than 500 names or a name
// of empty text is a Permanent error.
func (s *StockIDs) StockIDs(ctx context.Context, names []string) ([]store.InstrumentName, error) {
	if len(names) > maxStockNames {
		return nil, permanent(fmt.Errorf("instrument: stock ids: %d names, at most %d", len(names), maxStockNames))
	}
	if slices.Contains(names, "") {
		return nil, permanent(fmt.Errorf("instrument: stock ids: empty name"))
	}
	if len(names) == 0 {
		return nil, nil
	}
	var out []store.InstrumentName
	err := s.db.Select(ctx, stockIDsSQL, []any{names}, func(r scanner) error {
		var n store.InstrumentName
		if err := r.Scan(&n.InstrumentID, &n.Type, &n.Name); err != nil {
			return err
		}
		out = append(out, n)
		return nil
	})
	if err != nil {
		return nil, asTyped(err, "stock ids")
	}
	slices.SortFunc(out, func(a, b store.InstrumentName) int {
		switch {
		case a.InstrumentID < b.InstrumentID:
			return -1
		case a.InstrumentID > b.InstrumentID:
			return 1
		}
		return 0
	})
	return out, nil
}
