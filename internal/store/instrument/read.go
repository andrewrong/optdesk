package instrument

import (
	"context"

	"github.com/andrewrong/optdesk/internal/store"
)

// selectSQL returns one row per valid identifier of each wanted instrument, and one row with NULL identifier
// columns for an instrument that has none; the numerics are read as text so that they round-trip as written.
const selectSQL = `SELECT i.instrument_id, i.instrument_type, i.name, i.currency, i.mic, i.calendar_id, i.status,
       i.price_increment::text, i.lot_size::text, d.scheme, d.value
FROM master.instrument i
LEFT JOIN master.instrument_identifier d ON d.instrument_id = i.instrument_id AND d.valid_to IS NULL
WHERE i.instrument_id = ANY($1)
ORDER BY i.instrument_id, d.scheme, d.value`

// Instruments returns the instruments of ids with the identifiers that are valid now, ordered by id; an unknown id
// is absent. An empty ids makes no database call.
func (s *instrumentStore) Instruments(ctx context.Context, ids []int64) ([]store.Instrument, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var out []store.Instrument
	err := s.db.Select(ctx, selectSQL, []any{ids}, func(r scanner) error {
		var in store.Instrument
		var scheme, value *string
		sp := &in.Spec
		if err := r.Scan(&in.ID, &sp.Type, &sp.Name, &sp.Currency, &sp.MIC, &sp.CalendarID, &sp.Status,
			&sp.PriceIncrement, &sp.LotSize, &scheme, &value); err != nil {
			return err
		}
		if n := len(out); n == 0 || out[n-1].ID != in.ID {
			out = append(out, in)
		}
		if scheme != nil && value != nil {
			cur := &out[len(out)-1]
			cur.Identifiers = append(cur.Identifiers, store.Identifier{Scheme: *scheme, Value: *value})
		}
		return nil
	})
	if err != nil {
		return nil, asTyped(err, "instruments")
	}
	return out, nil
}
