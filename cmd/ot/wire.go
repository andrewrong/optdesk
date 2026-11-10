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
	"github.com/andrewrong/optdesk/internal/compute/optioncand"
	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/optionnotice"
	"github.com/andrewrong/optdesk/internal/compute/optionresult"
	"github.com/andrewrong/optdesk/internal/compute/optionstruct"
	"github.com/andrewrong/optdesk/internal/compute/optiontrack"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/ruleparam"
	"github.com/andrewrong/optdesk/internal/compute/rules"
	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
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
	"github.com/andrewrong/optdesk/internal/pages/option_notices_page"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
	"github.com/andrewrong/optdesk/internal/pages/option_tracking_page"
	"github.com/andrewrong/optdesk/internal/pages/report_page"
	"github.com/andrewrong/optdesk/internal/pages/risks_block"
	"github.com/andrewrong/optdesk/internal/pages/settings_page"
	"github.com/andrewrong/optdesk/internal/pages/structures_page"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/reconcile"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/alphavantage"
	"github.com/andrewrong/optdesk/internal/sources/fred"
	"github.com/andrewrong/optdesk/internal/sources/jquants"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/account"
	"github.com/andrewrong/optdesk/internal/store/alert"
	"github.com/andrewrong/optdesk/internal/store/candidate"
	eventstore "github.com/andrewrong/optdesk/internal/store/events"
	"github.com/andrewrong/optdesk/internal/store/instrument"
	"github.com/andrewrong/optdesk/internal/store/ops"
	optionledgerstore "github.com/andrewrong/optdesk/internal/store/optionledger"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/store/ownersetpg"
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
	newReference            broker.NewReferenceFunc
	openInstrument          store.OpenInstrumentFunc
	openOptionEod           store.OpenOptionEodFunc
	openQuoteSnapshot       store.OpenQuoteSnapshotFunc
	openOptionCatalog       store.OpenOptionEodCatalogFunc
	openOptionTerms         store.OpenOptionTermsFunc
	openInstrumentName      store.OpenInstrumentNameFunc
	newOptionEod            jobs.NewOptionEodFunc
	newOptionEodJP          jobs.NewOptionEodFunc
	newOptionEodWatch       jobs.NewOptionEodWatchFunc
	openCandidateRW         openCandidateRWFunc
	newOptionCandidates     jobs.NewOptionCandidatesJobFunc
	newCandidateRule        compute.NewOptionCandidatesFunc
	newQuoteSnapshot        jobs.NewQuoteSnapshotFunc
	dialAccount             dialAccountFunc
	openAccount             store.OpenAccountFunc
	newAccountCapture       newAccountCaptureFunc
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
	structuresListPage      func(c structures_page.Config, alias string) pages.Page
	structureDetailPage     func(c structures_page.Config, alias, structureID string) pages.Page
	optionResultPage        func(c option_result_page.Config, alias string) pages.Page
	newOptionLedger         compute.NewOptionLedgerFunc
	newOptionStructures     compute.NewOptionStructuresFunc
	newOptionResults        compute.NewOptionResultsFunc
	newRuleParams           settingsapi.NewRuleParamsFunc
	openOwnerSet            ownerset.OpenFunc
	settingsPage            func(c settings_page.Config) pages.Page
	optionCandidatesPage    func(zone *time.Location, runID, buildID, scope string, cands store.CandidateReader) pages.Page
	optionNoticesPage       optionNoticesPageFunc
	optionTrackingPage      func(zone *time.Location, runID, buildID, scope string, r trackingReaders) pages.Page
	openOptionLedgerRO      store.OpenOptionLedgerFunc
	newOptionNotices        compute.NewOptionNoticesFunc
	noticeParams            compute.DefaultNoticeParamsFunc
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
	newMorningNotifier      notify.NewMorningNotifierFunc
	openGapHistory          gaphistory.OpenFunc
	openGapReader           store.OpenGapReaderFunc
	loadAlertRules          compute.LoadAlertRulesFunc
	newEarningsRefresh      jobs.NewEarningsRefreshFunc
	newAlphaVantage         sources.NewEarningsFunc
	newJQuants              sources.NewJQuantsFunc
	openEarningsDates       store.OpenEarningsDateStoreFunc
	// readFile reads watch.toml; nil means os.ReadFile.
	readFile func(string) ([]byte, error)
	// ext holds the deps of each command by name, filled from extDeps; read it with depsOf.
	ext map[string]any
}

// calendarArgs is what follows the uv program in the command of calendar_refresh, that is
// `run --no-sync --project pyside python -m optdesk.calendar`; calendarcli adds the
// calendar, the days and the output file. The program is the line OT_CALENDAR_UV, or uv on PATH.
var calendarArgs = []string{"run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.calendar"}

// envCalendarUV names the line of .env that sets the path of uv.
const envCalendarUV = "OT_CALENDAR_UV"

// defaultReadEnv is the reader used when deps.readEnv is nil (a test deps literal without newDeps).
var defaultReadEnv store.ReadEnvFunc = setup.ReadEnv

// newDeps returns the real constructors and the deps of each command from extDeps.
func newDeps() deps {
	d := newBaseDeps()
	d.ext = newExt()
	d.openCandidateRW = openCandidateWriter
	// run-once and the scheduler build option_eod over the watch list; newOptionEod (job.toml, SPY) is what the dry run builds.
	d.newOptionEodWatch, d.newOptionEod = optioneod.NewWithWatch, optioneod.New
	d.newOptionCandidates, d.newCandidateRule = optioncandidates.New, optioncand.New
	d.dialAccount, d.openAccount, d.newAccountCapture = dialAccountClient, account.Open, accountcapture.New
	d.optionNoticesPage, d.newOptionNotices, d.noticeParams = newOptionNoticesPage, optionnotice.New, optionnotice.DefaultParams
	d.newRuleParams = ruleparam.New
	d.openOwnerSet = ownersetpg.Open
	d.openOptionLedgerRO = openOptionLedgerStore
	d.settingsPage = settings_page.NewPage
	return d
}

// newBaseDeps returns the real constructors of the fields of deps other than ext.
func newBaseDeps() deps {
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
		newReference:            grpcbroker.DialReference,
		openInstrument:          instrument.Open,
		openOptionEod:           snapshot.OpenOptionEod,
		openQuoteSnapshot:       snapshot.OpenQuoteSnapshot,
		openOptionCatalog:       snapshot.OpenOptionEodCatalog,
		openOptionTerms:         instrument.OpenOptionTerms,
		openInstrumentName:      instrument.OpenInstrumentNames,
		newOptionEodJP:          optioneod.NewJP,
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
		structuresListPage:      structures_page.NewListPage,
		structureDetailPage:     structures_page.NewDetailPage,
		optionResultPage:        option_result_page.NewPage,
		newOptionLedger:         optionledger.New,
		newOptionStructures:     optionstruct.New,
		newOptionResults:        optionresult.New,
		optionCandidatesPage:    newOptionCandidatesPage,
		optionTrackingPage:      newOptionTrackingPage,
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
		newMorningNotifier:      core.NewMorningNotifier,
		openGapHistory:          ops.OpenGapHistory,
		openGapReader:           ops.OpenGapReader,
		loadAlertRules:          alertrules.Load,
		newEarningsRefresh:      earningsrefresh.New,
		newAlphaVantage:         alphavantage.New,
		newJQuants:              jquants.New,
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

// newOptionTrackingPage builds the page option_tracking_page in zone from the plain arguments; the one place
// that names option_tracking_page.Config.
func newOptionTrackingPage(zone *time.Location, runID, buildID, scope string, r trackingReaders) pages.Page {
	return option_tracking_page.NewPage(option_tracking_page.Config{
		RunID: runID, BuildID: buildID, Scope: scope, Zone: zone,
		Ledger: r.Ledger, Account: r.Account, Candidates: r.Candidates, Eod: r.Eod, Catalog: r.Catalog,
		Names: r.Names, Terms: r.Terms, Sessions: r.Sessions,
		Tracker: optiontrack.New(), Replay: optionledger.New(),
		Params:     trackapi.TrackParams{MarkMaxAgeSessions: 3, RecentDays: 30},
		CalendarID: "XNYS",
	})
}

// openOptionLedgerStore opens the option ledger store; a failed open is a nil interface, not a typed nil.
func openOptionLedgerStore(ctx context.Context, dsn string) (store.OptionLedgerStore, error) {
	s, err := optionledgerstore.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// optionNoticesPageFunc builds option_notices_page from plain arguments.
type optionNoticesPageFunc func(zone *time.Location, runID, buildID, scope, main string, aliases []string, acct store.AccountReader,
	cands store.CandidateReader, earn store.EarningsDateReader, rule compute.OptionNotices, params compute.NoticeParams) pages.Page

// newOptionNoticesPage builds the page option_notices_page in zone from the plain arguments; the one place
// that names option_notices_page.Config.
func newOptionNoticesPage(zone *time.Location, runID, buildID, scope, main string, aliases []string, acct store.AccountReader,
	cands store.CandidateReader, earn store.EarningsDateReader, rule compute.OptionNotices, params compute.NoticeParams) pages.Page {
	return option_notices_page.NewPage(option_notices_page.Config{
		RunID: runID, BuildID: buildID, Scope: scope, Zone: zone, Main: main, Aliases: aliases,
		Account: acct, Candidates: cands, Earnings: earn, Rule: rule, Params: params,
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

// openCandidateWriter opens the candidate store for the job; a failed open is a nil interface, not a typed nil.
func openCandidateWriter(ctx context.Context, dsn string) (candidateRW, error) {
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
