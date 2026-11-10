package barsrefresh

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// NewBarsRefreshFunc is the constructor of the bars_refresh job (internal/jobs/barsrefresh.New). The
// arguments are the broker, the history client, the instrument resolver, the bar writer and reader, the
// watch list and the clock.
type NewBarsRefreshFunc func(
	broker.Broker, broker.HistoryClient, store.InstrumentResolver, bar.PriceBarWriter, bar.PriceBarReader,
	[]jobs.WatchEntry, clock.Clock,
) (contracts.Job, error)
