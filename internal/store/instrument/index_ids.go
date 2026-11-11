package instrument

import (
	"context"
	"fmt"
	"slices"

	"github.com/andrewrong/optdesk/internal/store"
)

// maxIndexNames is the most names one IndexIDs call accepts.
const maxIndexNames = 500

// indexIDsSQL reads the page view, rows of the type index only.
const indexIDsSQL = `SELECT instrument_id, instrument_type, name
FROM rd.instrument_r1
WHERE instrument_type = 'index' AND name = ANY($1)
ORDER BY instrument_id`

// IndexIDs reads the instrument ids of indices by name.
type IndexIDs struct{ db database }

// OpenIndexIDs connects to the database named by dsn (the line store.DSNEnvPages, role ot_ro_pages) and returns the
// reader. It applies no migration.
func OpenIndexIDs(ctx context.Context, dsn string) (*IndexIDs, error) {
	db, err := openDB(ctx, "index ids", dsn)
	if err != nil {
		return nil, err
	}
	return &IndexIDs{db: db}, nil
}

// Close releases the connections.
func (s *IndexIDs) Close() error { return s.db.Close() }

// IndexIDs returns the rows of type index whose name is in names, ordered by InstrumentID; an unknown name is absent
// and a name held by two rows gives two rows. An empty names makes no database call; more than 500 names or a name
// of empty text is a Permanent error.
func (s *IndexIDs) IndexIDs(ctx context.Context, names []string) ([]store.InstrumentName, error) {
	if len(names) > maxIndexNames {
		return nil, permanent(fmt.Errorf("instrument: index ids: %d names, at most %d", len(names), maxIndexNames))
	}
	if slices.Contains(names, "") {
		return nil, permanent(fmt.Errorf("instrument: index ids: empty name"))
	}
	if len(names) == 0 {
		return nil, nil
	}
	var out []store.InstrumentName
	err := s.db.Select(ctx, indexIDsSQL, []any{names}, func(r scanner) error {
		var n store.InstrumentName
		if err := r.Scan(&n.InstrumentID, &n.Type, &n.Name); err != nil {
			return err
		}
		out = append(out, n)
		return nil
	})
	if err != nil {
		return nil, asTyped(err, "index ids")
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
