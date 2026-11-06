package main

import (
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/calendar"
	"github.com/andrewrong/optdesk/internal/clock/realclock"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/embed"
	"github.com/andrewrong/optdesk/internal/pages/macro_rates"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/reconcile"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/fred"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ops"
	"github.com/andrewrong/optdesk/internal/store/series"
	"github.com/andrewrong/optdesk/internal/store/sessionday"
	"github.com/andrewrong/optdesk/internal/store/setup"
)

// deps holds the constructors the subcommands call; a test fills it with fakes.
type deps struct {
	newClock        clock.NewClockFunc
	newCalendar     clock.NewCalendarFunc
	openSessionDay  store.OpenSessionDayFunc
	openSeries      store.OpenSeriesFunc
	openLedger      store.OpenLedgerFunc
	setup           store.SetupFunc
	newFRED         sources.NewFREDFunc
	newFredSeries   jobs.NewFredSeriesFunc
	newRunner       runner.NewFunc
	buildMacroRates pages.BuildMacroRatesFunc
	newHandler      pages.NewHandlerFunc
}

// newDeps returns the real constructors.
func newDeps() deps {
	return deps{
		newClock:        realclock.NewReal,
		newCalendar:     calendar.NewCalendar,
		openSessionDay:  sessionday.Open,
		openSeries:      series.Open,
		openLedger:      ops.Open,
		setup:           setup.Setup,
		newFRED:         fred.New,
		newFredSeries:   fredseries.New,
		newRunner:       reconcile.New,
		buildMacroRates: macro_rates.Build,
		newHandler:      embed.Handler,
	}
}
