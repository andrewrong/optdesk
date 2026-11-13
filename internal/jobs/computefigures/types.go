package computefigures

import (
	"context"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// ComputeStores is the seven store interfaces the compute_figures job reads and writes, and the optional PriceBars.
type ComputeStores struct {
	Catalog     store.OptionEodCatalog
	OptionEod   store.OptionEodReader
	Terms       store.OptionTermsReader
	Names       store.InstrumentNameReader
	SessionDays store.SessionDayReader
	Series      store.SeriesReader
	Readings    store.ReadingStore
	// PriceBars is optional: nil skips the HV30 step (no key market.hv30_own is planned); set, the job plans and
	// stores market.hv30_own for each underlying from the winner "trades" rows.
	PriceBars bar.PriceBarReader
	// IndexIDs is optional and declared for the rv20 step, which no code uses yet: it maps the index names of
	// the manifest to instrument ids.
	IndexIDs IndexIDReader
}

// TechDeps is what the tech step of the compute_figures job needs. With Classify nil, or PriceBars or IndexIDs
// of the ComputeStores nil, WithTech returns the job as it is.
type TechDeps struct {
	Classify blockapi.TechFunc
	Params   blockapi.TechParams
}

// IndexIDReader reads the instruments of type index whose name is in names, ordered by instrument id.
type IndexIDReader interface {
	IndexIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)
}

// IndexIDStore is an IndexIDReader that is opened from a dsn and must be closed.
type IndexIDStore interface {
	IndexIDReader
	Close() error
}

// OpenIndexIDsFunc opens an IndexIDStore on the dsn.
type OpenIndexIDsFunc func(ctx context.Context, dsn string) (IndexIDStore, error)

// NewComputeFiguresFunc is the constructor of the compute_figures job (internal/jobs/computefigures.New).
type NewComputeFiguresFunc func(ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock) (contracts.Job, error)

// NewComputeFiguresEaseFunc is the constructor of the compute_figures job that also stores the ease figure
// (internal/jobs/computefigures.NewWithEase); the Ease comes after the SeriesOps and
// before the clock. With a nil compute.Ease it builds the same job as
// NewComputeFiguresFunc; with one the job plans and stores the key macro.ease|market|WALCL.
type NewComputeFiguresEaseFunc func(
	ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
) (contracts.Job, error)

// ValuationDeps is what the valuation step of the compute_figures job needs (docs/design/valuation.md
// Decisions 3, 5 and 7). Facts is read for the issuer of each entry, source sec, cutoff the run day; Entries
// is the watch list the keys are planned from (one set of three per entry with an Issuer, subject the
// Contract.Symbol). The five functions are the compute functions; with any of them nil the job has no
// valuation key.
type ValuationDeps struct {
	Facts        store.FactReader
	Entries      []jobs.WatchEntry
	StandardOf   valuationapi.StandardOfFunc
	BuildFilings valuationapi.BuildFilingsFunc
	Band         valuationapi.BandFunc
	Questions    valuationapi.QuestionsFunc
	Diagnosis    valuationapi.DiagnosisFunc
}

// NewComputeFiguresValuationFunc is the constructor of the compute_figures job that also stores, for each
// watched company, the readings valuation.buy_band, valuation.question and valuation.diagnosis
// (internal/jobs/computefigures.NewWithValuation); the ValuationDeps come after the SeriesOps and before the
// clock. With any function of the deps nil it builds the plain job, whose plan has no valuation key.
type NewComputeFiguresValuationFunc func(
	ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, ValuationDeps, clock.Clock,
) (contracts.Job, error)
