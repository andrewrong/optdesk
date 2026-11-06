package series

import (
	"context"
	"fmt"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const selectSQL = `SELECT series_id, observation_date, realtime_start, realtime_end, value, status, unit, currency, ` +
	`published_at, source, source_revision ` +
	`FROM rd.series_point_r1 ` +
	`WHERE series_id = $1 AND observation_date >= $2 AND observation_date <= $3 ` +
	`ORDER BY observation_date, realtime_start`

// SeriesPoints returns the points of seriesID with observation date between from and to, both inclusive,
// ordered by observation date and realtime start. No point in the range gives an empty slice and no error.
// Instants are returned in UTC.
func (s *seriesStore) SeriesPoints(ctx context.Context, seriesID string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	f, err := parseDay(from)
	if err != nil {
		return nil, err
	}
	t, err := parseDay(to)
	if err != nil {
		return nil, err
	}
	out := []contracts.SeriesPoint{}
	err = s.db.Select(ctx, selectSQL, []any{seriesID, f, t}, func(sc scanner) error {
		var r contracts.SeriesPoint
		var obs, rs, re time.Time
		var unit string
		var value *float64
		if err := sc.Scan(&r.SeriesID, &obs, &rs, &re, &value, &r.Status, &unit, &r.Currency,
			&r.PublishedAt, &r.Source, &r.SourceRevision); err != nil {
			return err
		}
		r.ObservationDate = types.SerializableDate{Time: obs}
		r.RealtimeStart = types.SerializableDate{Time: rs}
		r.RealtimeEnd = types.SerializableDate{Time: re}
		r.Unit = contracts.SeriesUnit(unit)
		r.Value = value
		r.PublishedAt = r.PublishedAt.UTC()
		out = append(out, r)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("series: select: %w", err))
	}
	return out, nil
}
