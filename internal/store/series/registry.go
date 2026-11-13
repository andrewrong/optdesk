package series

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

var _ seriesdef.SeriesRegistry = (*seriesStore)(nil)

// activeSeriesSQL reads the active registry rows.
const activeSeriesSQL = `SELECT series_id, title, category, frequency, source, source_series_id, secondary_source, ` +
	`seasonal_adjustment, unit::text, currency, first_observation, active ` +
	`FROM obs.series_definition WHERE active ORDER BY series_id`

// ActiveSeries returns the registry rows of obs.series_definition that are active, ordered by series id.
func (s *seriesStore) ActiveSeries(ctx context.Context) ([]seriesdef.SeriesDefinition, error) {
	var out []seriesdef.SeriesDefinition
	err := s.db.Select(ctx, activeSeriesSQL, nil, func(sc scanner) error {
		var d seriesdef.SeriesDefinition
		var first *time.Time
		if err := sc.Scan(&d.SeriesID, &d.Title, &d.Category, &d.Frequency, &d.Source, &d.SourceSeriesID,
			&d.SecondarySource, &d.SeasonalAdjustment, &d.Unit, &d.Currency, &first, &d.Active); err != nil {
			return err
		}
		if first != nil {
			d.FirstObservation = first.Format(time.DateOnly)
		}
		out = append(out, d)
		return nil
	})
	if err != nil {
		return nil, retryable(fmt.Errorf("series: active series: %w", err))
	}
	return out, nil
}
