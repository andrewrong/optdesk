package fredseries

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const dayLayout = "2006-01-02"

// gapFrom is the first day a run for day must fetch for ids: the day after the oldest of the
// newest stored valued points of the ids, not before groupFrom and not after day. An id with no
// valued point in [groupFrom, day] gives groupFrom.
func gapFrom(ctx context.Context, r store.SeriesReader, ids []string, groupFrom, day contracts.Day) (contracts.Day, error) {
	var oldest time.Time
	for i, id := range ids {
		pts, err := r.SeriesPoints(ctx, id, groupFrom, day)
		if err != nil {
			return groupFrom, err
		}
		var newest time.Time
		for _, p := range pts {
			if p.Value != nil && p.ObservationDate.After(newest) {
				newest = p.ObservationDate.Time
			}
		}
		if newest.IsZero() {
			return groupFrom, nil
		}
		if i == 0 || newest.Before(oldest) {
			oldest = newest
		}
	}
	if oldest.IsZero() {
		return groupFrom, nil
	}
	from := contracts.Day(oldest.AddDate(0, 0, 1).Format(dayLayout))
	if from < groupFrom {
		return groupFrom, nil
	}
	if from > day {
		return day, nil
	}
	return from, nil
}
