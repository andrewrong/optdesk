// Package jobs owns the jobs the binary runs.
package jobs

import (
	"context"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// Registry is the jobs the binary runs.
type Registry []contracts.Job

// NewFredSeriesFunc is the constructor of the FRED series job (internal/jobs/fredseries.New).
type NewFredSeriesFunc func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error)

// CaptureID identifies one capture: one per key, deterministic,
// uuid.NewSHA1(uuid.NameSpaceOID, []byte(day + "|" + dataset + "|" + subject)).
type CaptureID = uuid.UUID

// NewOptionEodFunc is the constructor of the option_eod job (internal/jobs/optioneod.New).
type NewOptionEodFunc func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error)

// NewQuoteSnapshotFunc is the constructor of the quote_snapshot job (internal/jobs/quotesnapshot.New).
type NewQuoteSnapshotFunc func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error)

// ComputeStores is the seven store interfaces the compute_figures job reads and writes.
type ComputeStores struct {
	Catalog     store.OptionEodCatalog
	OptionEod   store.OptionEodReader
	Terms       store.OptionTermsReader
	Names       store.InstrumentNameReader
	SessionDays store.SessionDayReader
	Series      store.SeriesReader
	Readings    store.ReadingStore
}

// NewComputeFiguresFunc is the constructor of the compute_figures job (internal/jobs/computefigures.New).
type NewComputeFiguresFunc func(ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock) (contracts.Job, error)

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

// CalendarObserver is called once per changed day of the calendar_refresh job, before the job writes; the
// wiring logs one line per call. It must not block and its return value is not used.
type CalendarObserver func(clock.SessionDayChange)

// NewCalendarRefreshFunc is the constructor of the calendar_refresh job (internal/jobs/calendarrefresh.New).
type NewCalendarRefreshFunc func(clock.SessionSource, store.SessionDayStore, clock.SessionDayDiffer, clock.Clock, CalendarObserver) (contracts.Job, error)
