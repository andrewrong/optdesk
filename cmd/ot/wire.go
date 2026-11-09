package main

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/broker/grpcbroker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/clock/calendar"
	"github.com/andrewrong/optdesk/internal/clock/calendarcli"
	"github.com/andrewrong/optdesk/internal/clock/realclock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/alertrules"
	"github.com/andrewrong/optdesk/internal/compute/ease"
	"github.com/andrewrong/optdesk/internal/compute/figures"
	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/jobs/quotesnapshot"
	"github.com/andrewrong/optdesk/internal/jobs/serieshistory"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/dryrun"
	"github.com/andrewrong/optdesk/internal/notify/telegram"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/embed"
	"github.com/andrewrong/optdesk/internal/pages/macro_page"
	"github.com/andrewrong/optdesk/internal/pages/macro_rates"
	"github.com/andrewrong/optdesk/internal/pages/market_page"
	"github.com/andrewrong/optdesk/internal/pages/option_candidates_page"
	"github.com/andrewrong/optdesk/internal/pages/option_data_page"
	"github.com/andrewrong/optdesk/internal/pages/report_page"
	"github.com/andrewrong/optdesk/internal/pages/risks_block"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/reconcile"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/alphavantage"
	"github.com/andrewrong/optdesk/internal/sources/fred"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/alert"
	"github.com/andrewrong/optdesk/internal/store/candidate"
	eventstore "github.com/andrewrong/optdesk/internal/store/events"
	"github.com/andrewrong/optdesk/internal/store/instrument"
	"github.com/andrewrong/optdesk/internal/store/ops"
	"github.com/andrewrong/optdesk/internal/store/reading"
	"github.com/andrewrong/optdesk/internal/store/series"
	"github.com/andrewrong/optdesk/internal/store/sessionday"
	"github.com/andrewrong/optdesk/internal/store/setup"
	"github.com/andrewrong/optdesk/internal/store/snapshot"
	"github.com/jonboulle/clockwork"
)

// deps holds the constructors the subcommands call; a test fills it with fakes.
type deps struct {
	newClock                clock.NewClockFunc
	newCalendar             clock.NewCalendarFunc
	newCommandSessionSource clock.NewCommandSessionSourceFunc
	newDiffer               clock.NewSessionDayDifferFunc
	newCalendarRefresh      jobs.NewCalendarRefreshFunc
	openSessionDay          store.OpenSessionDayFunc
	openSeries              store.OpenSeriesFunc
	openLedger              store.OpenLedgerFunc
	setup                   store.SetupFunc
	newFRED                 sources.NewFREDFunc
	newFredSeries           jobs.NewFredSeriesFunc
	newSeriesHistory        jobs.NewSeriesHistoryFunc
	newRunner               runner.NewWithRuleFunc
	newBackfiller           runner.NewBackfillerWithRuleFunc
	dialBroker              broker.DialFunc
	openInstrument          store.OpenInstrumentFunc
	openOptionEod           store.OpenOptionEodFunc
	openQuoteSnapshot       store.OpenQuoteSnapshotFunc
	openOptionCatalog       store.OpenOptionEodCatalogFunc
	openOptionTerms         store.OpenOptionTermsFunc
	openInstrumentName      store.OpenInstrumentNameFunc
	newOptionEod            jobs.NewOptionEodFunc
	newQuoteSnapshot        jobs.NewQuoteSnapshotFunc
	openReading             store.OpenReadingFunc
	openReadingView         store.OpenReadingViewFunc
	reportPage              func(market, slot string, zone *time.Location, risks report_page.RiskBlockFunc) pages.Page
	alertRules              compute.DefaultAlertRulesFunc
	evaluateAlerts          compute.EvaluateAlertsFunc
	riskBlock               riskBlockFunc
	newComputeFigures       jobs.NewComputeFiguresEaseFunc
	computeFigures          compute.Figures
	computeRules            compute.Rules
	computeUnits            compute.Units
	computeSeriesOps        compute.SeriesOps
	computeEase             compute.Ease
	pageList                []pages.Page
	newHandler              pages.NewHandlerFunc
	migrateUp               func(ctx context.Context, adminDSN string, fsys fs.FS) error
	ownerDSN                func(adminDSN string) (string, error)
	readEnv                 store.ReadEnvFunc
	macroPage               func(zone *time.Location, buildID string, read compute.ReadingFunc) pages.Page
	marketPage              func(zone *time.Location, runID, buildID, scope string, readings compute.ReadingFunc) pages.Page
	optionDataPage          func(zone *time.Location, buildID string, in sitePageInput) pages.Page
	openCandidate           openCandidateFunc
	optionCandidatesPage    func(zone *time.Location, runID, buildID, scope string, cands store.CandidateReader) pages.Page
	openFence               store.OpenFenceFunc
	openSchedulerLedger     store.OpenSchedulerLedgerFunc
	newScheduler            runner.NewSchedulerFunc
	newKeyRunner            runner.NewKeyRunnerFunc
	newReconciler           runner.NewReconcilerFunc
	newGapSyncer            func(store.OpsLedger, store.SchedulerLedger, clockwork.Clock) runner.GapSyncer
	newFwClock              func() clockwork.Clock
	notifyContext           func(context.Context) (context.Context, context.CancelFunc)
	newNotifier             notify.NewNotifierFunc
	newTelegramSender       notify.NewTelegramSenderFunc
	newDryRunSender         notify.NewDryRunSenderFunc
	openSent                store.OpenSentFunc
	newAlertNotifier        notify.NewAlertNotifierFunc
	loadAlertRules          compute.LoadAlertRulesFunc
	newEarningsRefresh      jobs.NewEarningsRefreshFunc
	newAlphaVantage         sources.NewEarningsFunc
	openEarningsDates       store.OpenEarningsDateStoreFunc
	// readFile reads watch.toml; nil means os.ReadFile.
	readFile func(string) ([]byte, error)
}

// calendarArgs is what follows the uv program in the command of calendar_refresh, that is
// `run --no-sync --project pyside python -m optdesk.calendar`; calendarcli adds the
// calendar, the days and the output file. The program is the line OT_CALENDAR_UV, or uv on PATH.
var calendarArgs = []string{"run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.calendar"}

// envCalendarUV names the line of .env that sets the path of uv.
const envCalendarUV = "OT_CALENDAR_UV"

// defaultReadEnv is the reader used when deps.readEnv is nil (a test deps literal without newDeps).
var defaultReadEnv store.ReadEnvFunc = setup.ReadEnv

// newDeps returns the real constructors.
func newDeps() deps {
	stats := gonumstats.New()
	sops := own.NewSeriesOps()
	rl := rules.New()
	return deps{
		newClock:                realclock.NewReal,
		newCalendar:             calendar.NewCalendar,
		newCommandSessionSource: calendarcli.New,
		newDiffer:               calendar.NewDiffer,
		newCalendarRefresh:      calendarrefresh.New,
		openSessionDay:          sessionday.Open,
		openSeries:              series.Open,
		openLedger:              ops.Open,
		setup:                   setup.Setup,
		newFRED:                 fred.New,
		newFredSeries:           fredseries.New,
		newSeriesHistory:        serieshistory.New,
		newRunner:               reconcile.NewWithRule,
		newBackfiller:           reconcile.NewBackfillerWithRule,
		dialBroker:              grpcbroker.Dial,
		openInstrument:          instrument.Open,
		openOptionEod:           snapshot.OpenOptionEod,
		openQuoteSnapshot:       snapshot.OpenQuoteSnapshot,
		openOptionCatalog:       snapshot.OpenOptionEodCatalog,
		openOptionTerms:         instrument.OpenOptionTerms,
		openInstrumentName:      instrument.OpenInstrumentNames,
		newOptionEod:            optioneod.New,
		newQuoteSnapshot:        quotesnapshot.New,
		openReading:             reading.Open,
		openReadingView:         reading.OpenView,
		reportPage:              newReportPage,
		alertRules:              alertrules.DefaultRules,
		evaluateAlerts:          alertrules.Evaluate,
		riskBlock:               newRiskBlock,
		newComputeFigures:       computefigures.NewWithEase,
		computeFigures:          figures.New(stats, own.NewRolling(stats), sops, rl),
		computeRules:            rl,
		computeUnits:            own.NewUnits(),
		computeSeriesOps:        sops,
		computeEase:             ease.New(rl),
		pageList:                []pages.Page{macro_rates.Page()},
		newHandler:              embed.Handler,
		migrateUp:               setup.MigrateUp,
		ownerDSN:                setup.OwnerDSN,
		readEnv:                 setup.ReadEnv,
		macroPage:               newMacroPage,
		marketPage:              newMarketPageOf,
		optionDataPage:          newOptionDataPage,
		openCandidate:           openCandidateStore,
		optionCandidatesPage:    newOptionCandidatesPage,
		openFence:               ops.OpenFence,
		openSchedulerLedger:     ops.OpenSchedulerLedger,
		newScheduler:            sched.New,
		newKeyRunner:            reconcile.NewKeyRunner,
		newReconciler:           reconcile.NewReconciler,
		newGapSyncer:            sched.NewGapSyncer,
		newFwClock:              clockwork.NewRealClock,
		notifyContext:           stopOnSignal,
		newNotifier:             core.New,
		newTelegramSender:       telegram.New,
		newDryRunSender:         dryrun.New,
		openSent:                alert.Open,
		newAlertNotifier:        core.NewAlerts,
		loadAlertRules:          alertrules.Load,
		newEarningsRefresh:      earningsrefresh.New,
		newAlphaVantage:         alphavantage.New,
		openEarningsDates:       eventstore.OpenEarningsDate,
		readFile:                os.ReadFile,
	}
}

// newMacroPage builds the page macro_page in zone; the run id is a label inside the page JSON only.
func newMacroPage(zone *time.Location, buildID string, read compute.ReadingFunc) pages.Page {
	cfg := macro_page.Config{
		RunID:   "ot-page",
		BuildID: buildID,
		Scope:   "macro",
		Zone:    zone,
		Read:    read,
		Indicators: []macro_page.Indicator{
			// single source moves to the job manifest in a later design
			{SeriesID: "DGS10", Name: "10年期美债收益率", SourceText: "FRED"},
		},
	}
	return macro_page.NewPage(cfg)
}

// newMarketPageOf builds the page market_page in zone from the plain arguments; the one place that names market_page.Config.
func newMarketPageOf(zone *time.Location, runID, buildID, scope string, readings compute.ReadingFunc) pages.Page {
	return market_page.NewPage(market_page.Config{RunID: runID, BuildID: buildID, Scope: scope, Zone: zone, Readings: readings})
}

// newOptionCandidatesPage builds the page option_candidates_page in zone from the plain arguments; the one
// place that names option_candidates_page.Config.
func newOptionCandidatesPage(zone *time.Location, runID, buildID, scope string, cands store.CandidateReader) pages.Page {
	return option_candidates_page.NewPage(option_candidates_page.Config{
		RunID: runID, BuildID: buildID, Scope: scope, Zone: zone, Candidates: cands,
	})
}

// openCandidateStore opens the candidate store; a failed open is a nil interface, not a typed nil.
func openCandidateStore(ctx context.Context, dsn string) (candidateStore, error) {
	s, err := candidate.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// newOptionDataPage builds the page option_data_page of one underlying in zone; the symbol and the notes come
// from the site.
func newOptionDataPage(zone *time.Location, buildID string, in sitePageInput) pages.Page {
	choices := make([]option_data_page.Choice, 0, len(in.Choices))
	for _, c := range in.Choices {
		choices = append(choices, option_data_page.Choice{ID: c.ID, Label: c.Label, Href: c.Href})
	}
	return option_data_page.NewPage(option_data_page.Config{
		RunID:        "ot-page",
		BuildID:      buildID,
		Scope:        "option",
		Zone:         zone,
		UnderlyingID: in.UnderlyingID,
		Label:        in.Label,
		Symbol:       in.Symbol,
		Notes:        in.Notes,
		Eod:          in.Eod,
		Quotes:       in.Quotes,
		Terms:        in.Terms,
		Choices:      choices,
	})
}

// newAssets returns the handler of /assets/* of the web bundle; the one place wire.go calls embed.Assets.
func newAssets() http.Handler { return embed.Assets() }

// newReportPage builds the page report_page of one market and slot in zone; risks gives its risks block.
func newReportPage(market, slot string, zone *time.Location, risks report_page.RiskBlockFunc) pages.Page {
	return report_page.NewPage(report_page.Config{Market: market, Slot: slot, Zone: zone, Risks: risks})
}

// newRiskBlock is risks_block.Build over the rules, the evaluation and the winner reader.
func newRiskBlock(ctx context.Context, rules compute.AlertRules, eval compute.EvaluateAlertsFunc, read compute.ReadingFunc,
	today reportDay, at time.Time) ([]byte, error) {
	return risks_block.Build(ctx, risks_block.Input{Rules: rules, Evaluate: eval, Read: read, Today: today, GeneratedAt: at})
}
