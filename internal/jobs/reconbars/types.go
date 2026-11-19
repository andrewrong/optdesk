package reconbars

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/reconapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

// NewReconBarsFunc is the constructor of the recon_bars job (internal/jobs/reconbars.New). The arguments are the
// bar source, the instrument resolver, the bar reader, the recon writer, the compare function, the watch list and
// the clock.
type NewReconBarsFunc func(
	sources.BarSource, store.InstrumentResolver, bar.PriceBarReader, reconstore.Writer, reconapi.Func,
	[]jobs.WatchEntry, clock.Clock,
) (contracts.Job, error)
