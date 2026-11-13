// Package seriesdef declares the optional store methods of the series
// registry: upsert by key, backup fill and the registry read of
// obs.series_definition. It holds declarations only; the implementation is
// internal/store/series.
package seriesdef

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// SeriesDefinition is one registry row of obs.series_definition. Empty strings
// mean "not set".
type SeriesDefinition struct {
	SeriesID, Title, Category, Frequency, Source, SourceSeriesID, SecondarySource, SeasonalAdjustment string
	Unit, Currency                                                                                    string
	FirstObservation                                                                                  string // a day, YYYY-MM-DD, or empty
	Active                                                                                            bool
}

// WriteCount is what one UpsertSeriesPoints or FillSeriesPoints call did:
// Written rows were inserted or replaced, Unchanged rows were equal to the
// stored one (or, for a fill, left because a present value stands).
type WriteCount struct{ Written, Unchanged int }

// SeriesUpserter writes points by the key (SeriesID, ObservationDate): a
// differing row replaces the stored one, an equal row changes nothing, no old
// value is kept. writtenBy names the run (job and day). An empty list gives
// zero counts. It is an optional method of the value OpenSeriesFunc returns;
// the job asserts it at Run.
type SeriesUpserter interface {
	UpsertSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint, writtenBy string) (WriteCount, error)
}

// SeriesFiller writes backup points: a key that is missing, or stored no_value
// or void, is written; a present value is never replaced. It is an optional
// method of the same value.
type SeriesFiller interface {
	FillSeriesPoints(ctx context.Context, rows []contracts.SeriesPoint, writtenBy string) (WriteCount, error)
}

// SeriesRegistry reads the registry. It is an optional method of the same
// value.
type SeriesRegistry interface {
	// ActiveSeries returns the definitions whose Active is true.
	ActiveSeries(ctx context.Context) ([]SeriesDefinition, error)
}
