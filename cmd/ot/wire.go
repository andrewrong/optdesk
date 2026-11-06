package main

import (
	"context"
	"io/fs"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/calendar"
	"github.com/andrewrong/optdesk/internal/clock/realclock"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/embed"
	"github.com/andrewrong/optdesk/internal/pages/macro_page"
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
	newClock       clock.NewClockFunc
	newCalendar    clock.NewCalendarFunc
	openSessionDay store.OpenSessionDayFunc
	openSeries     store.OpenSeriesFunc
	openLedger     store.OpenLedgerFunc
	setup          store.SetupFunc
	newFRED        sources.NewFREDFunc
	newFredSeries  jobs.NewFredSeriesFunc
	newRunner      runner.NewWithRuleFunc
	pageList       []pages.Page
	newHandler     pages.NewHandlerFunc
	migrateUp      func(ctx context.Context, adminDSN string, fsys fs.FS) error
	ownerDSN       func(adminDSN string) (string, error)
	readEnv        store.ReadEnvFunc
	macroPage      func(zone *time.Location, buildID string) pages.Page
}

// defaultReadEnv is the reader used when deps.readEnv is nil (a test deps literal without newDeps).
var defaultReadEnv store.ReadEnvFunc = setup.ReadEnv

// newDeps returns the real constructors.
func newDeps() deps {
	return deps{
		newClock:       realclock.NewReal,
		newCalendar:    calendar.NewCalendar,
		openSessionDay: sessionday.Open,
		openSeries:     series.Open,
		openLedger:     ops.Open,
		setup:          setup.Setup,
		newFRED:        fred.New,
		newFredSeries:  fredseries.New,
		newRunner:      reconcile.NewWithRule,
		pageList:       []pages.Page{macro_rates.Page()},
		newHandler:     embed.Handler,
		migrateUp:      setup.MigrateUp,
		ownerDSN:       setup.OwnerDSN,
		readEnv:        setup.ReadEnv,
		macroPage:      newMacroPage,
	}
}

// newMacroPage builds the page macro_page in zone; the run id is a label inside the page JSON only.
func newMacroPage(zone *time.Location, buildID string) pages.Page {
	cfg := macro_page.Config{
		RunID:   "ot-page",
		BuildID: buildID,
		Scope:   "macro",
		Zone:    zone,
		Indicators: []macro_page.Indicator{
			// single source moves to the job manifest in a later design
			{SeriesID: "DGS10", Name: "10年期美债收益率", SourceText: "FRED"},
		},
	}
	return macro_page.NewPage(cfg)
}
