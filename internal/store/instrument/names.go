package instrument

import (
	"context"
	"fmt"
	"slices"

	"github.com/andrewrong/optdesk/internal/store"
)

var _ store.OpenInstrumentNameFunc = OpenInstrumentNames

// namesSQL reads the page view; the role of the page reader has no right on the tables behind it.
const namesSQL = `SELECT instrument_id, instrument_type, name
FROM rd.instrument_r1
WHERE instrument_id = ANY($1)
ORDER BY instrument_id`

// instrumentNameStore is the store.InstrumentNameStore over a database seam.
type instrumentNameStore struct{ db database }

// OpenInstrumentNames connects to the database named by dsn (the line store.DSNEnvPages, role ot_ro_pages) and
// returns the name reader. It applies no migration.
func OpenInstrumentNames(ctx context.Context, dsn string) (store.InstrumentNameStore, error) {
	db, err := openDB(ctx, "instrument names", dsn)
	if err != nil {
		return nil, err
	}
	return &instrumentNameStore{db: db}, nil
}

// Close releases the connections.
func (s *instrumentNameStore) Close() error { return s.db.Close() }

// InstrumentNames returns the names of ids ordered by InstrumentID; an unknown id is absent. An empty ids makes no
// database call; more than 500 ids or an id of 0 is a Permanent error.
func (s *instrumentNameStore) InstrumentNames(ctx context.Context, ids []int64) ([]store.InstrumentName, error) {
	if len(ids) > maxTermsIDs {
		return nil, permanent(fmt.Errorf("instrument: instrument names: %d ids, at most %d", len(ids), maxTermsIDs))
	}
	if slices.Contains(ids, 0) {
		return nil, permanent(fmt.Errorf("instrument: instrument names: instrument id 0"))
	}
	if len(ids) == 0 {
		return nil, nil
	}
	var out []store.InstrumentName
	err := s.db.Select(ctx, namesSQL, []any{ids}, func(r scanner) error {
		var n store.InstrumentName
		if err := r.Scan(&n.InstrumentID, &n.Type, &n.Name); err != nil {
			return err
		}
		out = append(out, n)
		return nil
	})
	if err != nil {
		return nil, asTyped(err, "instrument names")
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
