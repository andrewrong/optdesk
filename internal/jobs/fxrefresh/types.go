package fxrefresh

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// NewFxRefreshFunc is the constructor of the fx_refresh job (internal/jobs/fxrefresh.New). The arguments
// are the broker, the history client, the FX rate writer and reader and the clock.
type NewFxRefreshFunc func(broker.Broker, broker.HistoryClient, bar.FxRateWriter, bar.FxRateReader, clock.Clock) (contracts.Job, error)

// AttachSecondaryFunc attaches a fill source to an fx_refresh job built by New and reports whether j was such a
// job (false for any other contracts.Job, so a fake is left alone). A job without a source never asks one.
// The implementation is fxrefresh.AttachSecondary.
type AttachSecondaryFunc func(j contracts.Job, second sources.Source) bool
