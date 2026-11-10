package fredseries

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// NewFredSeriesFunc is the constructor of the FRED series job (internal/jobs/fredseries.New).
type NewFredSeriesFunc func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error)
