package serieshistory

import (
	"context"

	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// SeriesLoaded is the count of points fetched for one series.
type SeriesLoaded struct {
	SeriesID string
	Fetched  int
}

// SeriesHistoryLoader loads the history of series once, outside the daily job. Load fetches plan.From..plan.To
// once per series and writes each series once through the idempotent store.SeriesWriter. It refuses an empty
// series list, From after To and a range over 5 years (Permanent), and returns contracts.TypedError values only.
type SeriesHistoryLoader interface {
	Load(ctx context.Context, plan sources.FetchPlan) ([]SeriesLoaded, error)
}

// NewSeriesHistoryFunc is the constructor of the loader (internal/jobs/serieshistory.New).
type NewSeriesHistoryFunc func(sources.Source, store.SeriesWriter) (SeriesHistoryLoader, error)
