package main

import (
	"errors"
	"log/slog"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
)

// Design: docs/design/bars-fallback.md#11-bars-fallback-wire.

var errNoFallbackCalendar = errors.New("no session calendar")

// sourceYfinance is the registry id of the bar source of the fallback.
const sourceYfinance = "yfinance"

// withBarsFallback gives j, a bars_refresh job, the yfinance source and the session calendar the factory holds (f.cal)
// (barsrefresh.WithFallback). A source the registry cannot build, a missing command or calendar, or a job that
// takes none leaves j as built and logs one line; the rule of the fallback is in barsrefresh.
func (f jobFactory) withBarsFallback(j contracts.Job) contracts.Job {
	return f.withBarsFallbackIn(j, barSources, barsrefresh.WithFallback)
}

// attachFallbackFunc is barsrefresh.WithFallback; a test passes its own.
type attachFallbackFunc func(contracts.Job, sources.BarSource, clock.Calendar) (contracts.Job, error)

// withBarsFallbackIn is withBarsFallback over the registry reg and the attach func.
func (f jobFactory) withBarsFallbackIn(j contracts.Job, reg map[string]sources.NewBarSourceFunc, attach attachFallbackFunc) contracts.Job {
	left := func(err error) contracts.Job {
		log := f.log
		if log == nil {
			log = slog.Default()
		}
		log.Warn("bars_refresh runs without the yfinance fallback", "component", "ot", "job", jobBarsRefresh, "status", err.Error())
		return j
	}
	src, err := f.barSourceIn(reg, sourceYfinance)
	if err != nil {
		return left(err)
	}
	if f.cal == nil {
		return left(errNoFallbackCalendar)
	}
	out, err := attach(j, src, f.cal)
	if err != nil {
		return left(err)
	}
	return out
}
