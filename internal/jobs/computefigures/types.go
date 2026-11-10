package computefigures

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
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
}

// NewComputeFiguresFunc is the constructor of the compute_figures job (internal/jobs/computefigures.New).
type NewComputeFiguresFunc func(ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, clock.Clock) (contracts.Job, error)

// NewComputeFiguresEaseFunc is the constructor of the compute_figures job that also stores the ease figure
// (internal/jobs/computefigures.NewWithEase); the Ease comes after the SeriesOps and
// before the clock. With a nil compute.Ease it builds the same job as
// NewComputeFiguresFunc; with one the job plans and stores the key macro.ease|market|WALCL.
type NewComputeFiguresEaseFunc func(
	ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps, compute.Ease, clock.Clock,
) (contracts.Job, error)
