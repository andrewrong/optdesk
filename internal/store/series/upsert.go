package series

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

var (
	_ seriesdef.SeriesUpserter = (*seriesStore)(nil)
	_ seriesdef.SeriesFiller   = (*seriesStore)(nil)
)

// upsertPrefix is the insert shared by the upsert and the fill. Arguments: series_id, observation_date,
// realtime_start, realtime_end, value, status, unit, currency, published_at, source, source_revision, written_by.
const upsertPrefix = `INSERT INTO obs.series_point (series_id, observation_date, realtime_start, realtime_end, ` +
	`value, status, unit, currency, published_at, source, source_revision, written_by) ` +
	`VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) ` +
	`ON CONFLICT (series_id, observation_date) DO UPDATE SET value = EXCLUDED.value, status = EXCLUDED.status, ` +
	`unit = EXCLUDED.unit, currency = EXCLUDED.currency, realtime_start = EXCLUDED.realtime_start, ` +
	`realtime_end = EXCLUDED.realtime_end, published_at = EXCLUDED.published_at, source = EXCLUDED.source, ` +
	`source_revision = EXCLUDED.source_revision, updated_at = now(), written_by = $12 ` +
	`WHERE (obs.series_point.value, obs.series_point.status, obs.series_point.unit, obs.series_point.currency, ` +
	`obs.series_point.source) IS DISTINCT FROM (EXCLUDED.value, EXCLUDED.status, EXCLUDED.unit, EXCLUDED.currency, ` +
	`EXCLUDED.source)`

// upsertPointSQL replaces a stored point of the key when it differs; a no_value point never replaces a present one.
const upsertPointSQL = upsertPrefix +
	` AND NOT (obs.series_point.status = 'present' AND EXCLUDED.status = 'no_value') RETURNING series_id`

// fillPointSQL writes a backup point: only a stored no_value or void point is replaced.
const fillPointSQL = upsertPrefix +
	` AND obs.series_point.status IN ('no_value', 'void') RETURNING series_id`

// UpsertSeriesPoints writes the points by the key (series_id, observation_date) in one transaction: a differing
// point replaces the stored one, an equal one changes nothing, a key twice in the call is applied in order.
// A sentinel value is stored as NULL with StatusSentinelRejected. An empty list makes no database call.
func (s *seriesStore) UpsertSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint, writtenBy string) (seriesdef.WriteCount, error) {
	return s.write(ctx, "upsert", upsertPointSQL, rows, writtenBy)
}

// FillSeriesPoints writes backup points by the same key: a missing key, or one stored no_value or void, is written;
// a present value is never replaced. An empty list makes no database call.
func (s *seriesStore) FillSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint, writtenBy string) (seriesdef.WriteCount, error) {
	return s.write(ctx, "fill", fillPointSQL, rows, writtenBy)
}

func (s *seriesStore) write(ctx context.Context, op, pointSQL string, rows []contracts.SeriesPoint, writtenBy string) (seriesdef.WriteCount, error) {
	var c seriesdef.WriteCount
	if len(rows) == 0 {
		return c, nil
	}
	err := s.db.InTx(ctx, func(tx execer) error {
		c = seriesdef.WriteCount{} // a retried function starts from zero
		for _, r := range rows {
			value, status := screen(r.Value, r.Status)
			if err := tx.Exec(ctx, insertDefinitionSQL, r.SeriesID, string(r.Unit), r.Currency); err != nil {
				return err
			}
			n, err := tx.ExecReturning(ctx, pointSQL, r.SeriesID, r.ObservationDate.Time, r.RealtimeStart.Time,
				r.RealtimeEnd.Time, value, status, string(r.Unit), r.Currency, r.PublishedAt, r.Source, r.SourceRevision, writtenBy)
			if err != nil {
				return err
			}
			if n > 0 {
				c.Written++
			} else {
				c.Unchanged++
			}
		}
		return nil
	})
	if err != nil {
		return seriesdef.WriteCount{}, retryable(fmt.Errorf("series: %s: %w", op, err))
	}
	return c, nil
}
