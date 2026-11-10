package fxrefresh

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// NewFxRefreshFunc is the constructor of the fx_refresh job (internal/jobs/fxrefresh.New). The arguments
// are the broker, the history client, the FX rate writer and reader and the clock.
type NewFxRefreshFunc func(broker.Broker, broker.HistoryClient, bar.FxRateWriter, bar.FxRateReader, clock.Clock) (contracts.Job, error)
