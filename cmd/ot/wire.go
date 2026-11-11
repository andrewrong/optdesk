package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"sync"
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
	"github.com/andrewrong/optdesk/internal/compute/settingsform"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/accountcapture"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/calendarrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/dividendcapture"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/jobs/fxrefresh"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	optionledgerjob "github.com/andrewrong/optdesk/internal/jobs/optionledger"
	"github.com/andrewrong/optdesk/internal/jobs/quotesnapshot"
	"github.com/andrewrong/optdesk/internal/jobs/serieshistory"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/notify/core"
	"github.com/andrewrong/optdesk/internal/notify/dryrun"
	"github.com/andrewrong/optdesk/internal/notify/telegram"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/companies_page"
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
	"github.com/andrewrong/optdesk/internal/sources/flex"
	"github.com/andrewrong/optdesk/internal/sources/fred"
	"github.com/andrewrong/optdesk/internal/sources/jquants"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/account"
	"github.com/andrewrong/optdesk/internal/store/alert"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/barpg"
	"github.com/andrewrong/optdesk/internal/store/candidate"
	"github.com/andrewrong/optdesk/internal/store/dividend"
	eventstore "github.com/andrewrong/optdesk/internal/store/events"
	"github.com/andrewrong/optdesk/internal/store/instrument"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/store/ops"
	optionledgerstore "github.com/andrewrong/optdesk/internal/store/optionledger"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/store/ownersetpg"
	"github.com/andrewrong/optdesk/internal/store/reading"
	"github.com/andrewrong/optdesk/internal/store/series"
	"github.com/andrewrong/optdesk/internal/store/sessionday"
	"github.com/andrewrong/optdesk/internal/store/setup"
	"github.com/andrewrong/optdesk/internal/store/snapshot"
	"github.com/andrewrong/optdesk/internal/symbol"
	"github.com/andrewrong/optdesk/internal/symbol/table"
	"github.com/jonboulle/clockwork"
)

// deps holds the constructors the subcommands call; a test fills it with fakes.
type deps struct {
	newClock                clock.NewClockFunc
	newCalendar             clock.NewCalendarFunc
	newCommandSessionSource clock.NewCommandSessionSourceFunc
	newDiffer               clock.NewSessionDayDifferFunc
	newCalendarRefresh      calendarrefresh.NewCalendarRefreshFunc
	openSessionDay          store.OpenSessionDayFunc
	openSeries              store.OpenSeriesFunc
	openLedger              store.OpenLedgerFunc
	setup                   store.SetupFunc
	newFRED                 sources.NewFREDFunc
	newFredSeries           fredseries.NewFredSeriesFunc
	newSeriesHistory        serieshistory.NewSeriesHistoryFunc
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
	openIndexIDs            computefigures.OpenIndexIDsFunc
	newOptionEod            optioneod.NewOptionEodSymbolsFunc
	newOptionEodJP          optioneod.NewOptionEodFunc
	newOptionEodJPWatch     optioneod.NewOptionEodWatchFunc
	newOptionEodWatch       optioneod.NewOptionEodWatchSymbolsFunc
	openCandidateRW         openCandidateRWFunc
	newOptionCandidates     optioncandidates.NewOptionCandidatesJobFunc
	newCandidateRule        compute.NewOptionCandidatesFunc
	newQuoteSnapshot        quotesnapshot.NewQuoteSnapshotFunc
	dialAccount             dialAccountFunc
	openAccount             store.OpenAccountFunc
	newAccountCapture       newAccountCaptureFunc
	openMark                mark.OpenFunc
	newPositionMarks        newPositionMarksFunc
	openReading             store.OpenReadingFunc
	openReadingView         store.OpenReadingViewFunc
	reportPage              func(market, slot string, zone *time.Location, risks report_page.RiskBlockFunc) pages.Page
	reportPageBlocks        func(cfg report_page.Config) pages.Page // the report with its three block sources; nil falls back to reportPage (risks only)
	alertRules              compute.DefaultAlertRulesFunc
	evaluateAlerts          compute.EvaluateAlertsFunc
	riskBlock               riskBlockFunc
	newComputeFigures       computefigures.NewComputeFiguresEaseFunc
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
	newSettingsForm         settingsapi.NewSettingsFormFunc
	openOwnerSet            ownerset.OpenFunc
	settingsPage            func(c settings_page.Config) pages.Page
	optionCandidatesPage    func(zone *time.Location, runID, buildID, scope string, cands store.CandidateReader, names store.InstrumentNameReader) pages.Page
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
	newKeyGapSyncer         runner.NewKeyGapSyncerFunc
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
	newEarningsRefresh      earningsrefresh.NewEarningsRefreshSymbolsFunc
	symbols                 symbol.Translator
	newAlphaVantage         sources.NewEarningsFunc
	newJQuants              sources.NewJQuantsFunc
	openEarningsDates       store.OpenEarningsDateStoreFunc
	openDividend            dividend.OpenFunc
	newDividendCapture      dividendcapture.NewDividendCaptureFunc
	openOptionLedgerStore   store.OpenOptionLedgerFunc
	newOptionLedgerJob      optionledgerjob.NewOptionLedgerJobFunc
	loadFlexFile            sources.LoadFlexFileFunc
	newFlexSource           sources.NewFlexFunc
	importOptionTrader      optionledgerjob.ImportOptionTraderFunc
	// option_eod with the held short calls and option_candidates with the dividend reader (Decisions 11 and 12).
	newOptionEodHeld         optioneod.NewOptionEodHeldSymbolsFunc
	newOptionCandidatesCalls optioncandidates.NewOptionCandidatesCallsFunc
	// bars_refresh and fx_refresh: the bar store, the two constructors and the history client.
	openBar        bar.OpenBarFunc
	newBarsRefresh barsrefresh.NewBarsRefreshFunc
	newFxRefresh   fxrefresh.NewFxRefreshFunc
	newHistory     broker.NewHistoryFunc
	// readFile reads watch.toml; nil means os.ReadFile.
	readFile func(string) ([]byte, error)
	// ext holds the deps of each command by name, filled from extDeps; read it with depsOf.
	ext map[string]any
}

// calendarArgs is what follows the uv program in the command of calendar_refresh, that is
// `run --locked --project pyside python -m optdesk.calendar`; calendarcli adds the
// calendar, the days and the output file. The program is the line OT_CALENDAR_UV, or uv on PATH.
var calendarArgs = []string{"run", "--locked", "--project", "pyside", "python", "-m", "optdesk.calendar"}

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
	d.newOptionEodWatch, d.newOptionEod = optioneod.NewWithWatchSymbols, optioneod.NewWithSymbols
	// option_eod_jp is built over the watch list too: each JP stock entry adds a key (the dry run included).
	d.newOptionEodJPWatch = optioneod.NewJPWithWatch
	d.symbols = table.New()
	d.newOptionCandidates, d.newCandidateRule = optioncandidates.New, optioncand.New
	d.newOptionEodHeld, d.newOptionCandidatesCalls = optioneod.NewWithHeldSymbols, optioncandidates.NewWithCalls
	d.dialAccount, d.openAccount, d.newAccountCapture = dialAccountClient, account.Open, accountcapture.New
	realMarkDeps(&d) // position_marks
	d.optionNoticesPage, d.newOptionNotices, d.noticeParams = newOptionNoticesPage, optionnotice.New, optionnotice.DefaultParams
	d.openOptionLedgerStore, d.newOptionLedgerJob, d.importOptionTrader = openOptionLedgerStore, optionledgerjob.New, optionledgerjob.ImportOptionTrader
	d.loadFlexFile, d.newFlexSource = flex.Load, flex.New
	d.newRuleParams = ruleparam.New
	d.newSettingsForm = settingsform.New
	d.openDividend, d.newDividendCapture = eventstore.OpenDividend, dividendcapture.New
	d.openOwnerSet = ownersetpg.Open
	d.openBar, d.newBarsRefresh, d.newFxRefresh, d.newHistory = openBarStore, barsrefresh.New, fxrefresh.New, grpcbroker.DialHistory
	d.openOptionLedgerRO = openOptionLedgerStore
	d.settingsPage, d.reportPageBlocks = settings_page.NewPage, report_page.NewPage
	return d
}

// newBaseDeps returns the real constructors of the fields of deps other than ext.
func newBaseDeps() deps {
	stats, sops := gonumstats.New(), own.NewSeriesOps()
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
		openIndexIDs:            openIndexIDsDefault,
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
		newKeyGapSyncer:         sched.NewKeyGapSyncer,
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
		newEarningsRefresh:      earningsrefresh.NewWithSymbols,
		newAlphaVantage:         alphavantage.New,
		newJQuants:              jquants.New,
		openEarningsDates:       eventstore.OpenEarningsDate,
		readFile:                os.ReadFile,
	}
}

// openBarStore is barpg.Open as the bar.OpenBarFunc; a failed open is a nil interface, not a typed nil.
func openBarStore(ctx context.Context, dsn string) (bar.BarStore, error) {
	s, err := barpg.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
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
			{SeriesID: "DGS10", Name: "10-year Treasury yield", SourceText: "FRED"},
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
func newOptionCandidatesPage(zone *time.Location, runID, buildID, scope string, cands store.CandidateReader, names store.InstrumentNameReader) pages.Page {
	return option_candidates_page.NewPage(option_candidates_page.Config{
		RunID: runID, BuildID: buildID, Scope: scope, Zone: zone, Candidates: cands, Names: names,
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

// barsOff is the reason bars_refresh, fx_refresh and the HV step are left out for want of their line, or "" when set.
// It names the line, never a value.
func (c runOnceConfig) barsOff() string {
	if c.barDSN == "" {
		return reasonNoBarDSN
	}
	return ""
}

// noBarStore stands in for the bar store when OT_PG_DSN_BAR is empty: the jobs are left out, so it is never called.
type noBarStore struct{}

func (noBarStore) InsertPriceBars(context.Context, []contracts.PriceBar) (int, error) {
	return 0, errors.New(reasonNoBarDSN)
}

func (noBarStore) PriceBars(context.Context, int64, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
	return nil, errors.New(reasonNoBarDSN)
}

func (noBarStore) InsertFxRates(context.Context, []contracts.FxRate) (int, error) {
	return 0, errors.New(reasonNoBarDSN)
}

func (noBarStore) FxRates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.FxRate, error) {
	return nil, errors.New(reasonNoBarDSN)
}

func (noBarStore) LatestFxRate(context.Context, string, contracts.Day) (contracts.FxRate, bool, error) {
	return contracts.FxRate{}, false, errors.New(reasonNoBarDSN)
}

func (noBarStore) Close() error { return nil }

// unreachableHistory stands in for the history client when the broker is down: every call answers retryable.
type unreachableHistory struct{}

func (unreachableHistory) Bars(context.Context, broker.BarQuery) ([]broker.Bar, error) {
	return nil, brokerDownError{}
}

func (unreachableHistory) Close() error { return nil }

// barsRefresh builds bars_refresh over the broker, the history client, the instrument store opened for option_eod,
// the bar store and the watch list.
func (f jobFactory) barsRefresh(b broker.Broker, h broker.HistoryClient) (contracts.Job, error) {
	j, err := f.d.newBarsRefresh(b, h, f.inst, f.bars, f.bars, f.watch, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build bars_refresh job failed: %w", err)
	}
	return j, nil
}

// fxRefresh builds fx_refresh over the broker, the history client and the bar store.
func (f jobFactory) fxRefresh(b broker.Broker, h broker.HistoryClient) (contracts.Job, error) {
	j, err := f.d.newFxRefresh(b, h, f.bars, f.bars, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build fx_refresh job failed: %w", err)
	}
	return j, nil
}

// barJobs builds bars_refresh and fx_refresh beside option_eod and opens the bar reader of the HV step. It is empty for
// a deps without the constructors. Without OT_PG_DSN_BAR the jobs hold a stand-in store, with the broker down at start
// (or a failed history dial) a stand-in history client; reason is then not empty and names the line or the broker,
// the caller leaves the jobs out and the HV step is not given a reader. Nothing else is touched. What it opens goes
// to extra. No error holds a DSN or a path.
func (a *app) barJobs(
	ctx context.Context, cfg runOnceConfig, f *jobFactory, brk broker.Broker, brokerUp bool, extra *closerSet,
) (js []contracts.Job, reason string, err error) {
	if !cfg.barsWired {
		return nil, "", nil
	}
	d := f.d
	f.bars, f.hist = noBarStore{}, unreachableHistory{}
	reason = cfg.barsOff()
	if reason == "" {
		s, err := d.openBar(ctx, cfg.barDSN)
		if err != nil {
			return nil, "", fmt.Errorf("open bar store with %s failed", bar.DSNEnvBar)
		}
		extra.addCloser(s)
		f.bars = s
		reason = reasonBrokerDown
		if brokerUp {
			if h, err := d.newHistory(ctx, cfg.socketPath, broker.Options{}); err == nil {
				extra.addCloser(h)
				f.hist, reason = h, ""
			}
		}
	}
	if reason == "" {
		rd, err := d.openBar(ctx, cfg.pagesDSN)
		if err != nil {
			return nil, "", fmt.Errorf("open bar reader with %s failed", store.DSNEnvPages)
		}
		extra.addCloser(rd)
		f.compute.PriceBars = rd
	} else {
		a.log.Warn("HV step left out", "component", "ot", "step", "market.hv30_own, market.rv20", "status", reason)
	}
	bj, err := f.barsRefresh(brk, f.hist)
	if err != nil {
		return nil, "", err
	}
	fj, err := f.fxRefresh(brk, f.hist)
	if err != nil {
		return nil, "", err
	}
	return []contracts.Job{bj, fj}, reason, nil
}

// redialers return the funcs the probe of ot scheduler uses to dial the broker and the history client anew; what
// they open is closed through extra when withWiring returns.
func redialers(a *app, cfg runOnceConfig, extra *closerSet) (
	func(context.Context) (broker.Broker, error), func(context.Context) (broker.HistoryClient, error),
) {
	dial := func(ctx context.Context) (broker.Broker, error) {
		b, err := a.deps.dialBroker(ctx, cfg.socketPath, broker.Options{})
		if err != nil {
			return nil, err
		}
		extra.add(b)
		return b, nil
	}
	dialHistory := func(ctx context.Context) (broker.HistoryClient, error) {
		h, err := a.deps.newHistory(ctx, cfg.socketPath, broker.Options{})
		if err != nil {
			return nil, err
		}
		extra.addCloser(h)
		return h, nil
	}
	return dial, dialHistory
}

// jobStep is a job that is wired beside the families without a registry entry: bars_refresh and fx_refresh,
// and position_marks. They landed before the registry, have no document of their own yet and keep their place:
// each runs before the first family whose order is above after. A new job registers a family instead.
type jobStep struct {
	after int
	run   func(ctx context.Context, a *app, jc *jobCtx, w *wiring) ([]contracts.Job, error)
}

// jobSteps: bars_refresh and fx_refresh follow option_ledger (50), position_marks follows account_capture (100).
var jobSteps = []jobStep{
	{after: 50, run: func(ctx context.Context, a *app, jc *jobCtx, w *wiring) ([]contracts.Job, error) {
		js, reason, err := a.barJobs(ctx, jc.cfg, jc.f, jc.brk, jc.brokerUp, jc.extra)
		w.bars, w.barsOff = js, reason
		return js, err
	}},
	{after: 100, run: func(ctx context.Context, a *app, jc *jobCtx, _ *wiring) ([]contracts.Job, error) {
		j, err := a.positionMarks(ctx, *jc.f, jc.brk, jc.extra)
		if j == nil || err != nil {
			return nil, err
		}
		return []contracts.Job{j}, nil
	}},
}

// lazyStore opens a store on its first use with the ctx of that call and closes it if it was opened. The open
// happens at run time, not at wiring, so a run that refuses its config before any job runs opens nothing.
type lazyStore[T io.Closer] struct {
	open func(context.Context) (T, error)
	mu   sync.Mutex
	s    T
	ok   bool
}

func (l *lazyStore[T]) get(ctx context.Context) (T, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ok {
		s, err := l.open(ctx)
		if err != nil {
			var zero T
			return zero, err
		}
		l.s, l.ok = s, true
	}
	return l.s, nil
}

func (l *lazyStore[T]) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ok {
		return nil
	}
	l.ok = false
	return l.s.Close()
}

// lazyAccountReader is the account reader of option_eod: the store of OT_PG_DSN_ACCOUNT, opened on the first read.
type lazyAccountReader struct{ lazyStore[store.AccountStore] }

func (r *lazyAccountReader) Positions(ctx context.Context, alias, day string) ([]store.Position, error) {
	s, err := r.get(ctx)
	if err != nil {
		return nil, err
	}
	return s.Positions(ctx, alias, day)
}

func (r *lazyAccountReader) Values(ctx context.Context, alias, day string) ([]store.AccountValue, error) {
	s, err := r.get(ctx)
	if err != nil {
		return nil, err
	}
	return s.Values(ctx, alias, day)
}

func (r *lazyAccountReader) Executions(ctx context.Context, alias, from, to string) ([]store.Execution, error) {
	s, err := r.get(ctx)
	if err != nil {
		return nil, err
	}
	return s.Executions(ctx, alias, from, to)
}

// lazyDividendReader is the dividend reader of option_candidates: the store of OT_PG_DSN_EVENTS, opened on the first read.
type lazyDividendReader struct{ lazyStore[dividend.Store] }

func (r *lazyDividendReader) Dividends(ctx context.Context, id int, from, to contracts.Day) ([]contracts.DividendExpectation, error) {
	s, err := r.get(ctx)
	if err != nil {
		return nil, err
	}
	return s.Dividends(ctx, id, from, to)
}

// wireHeldCalls gives the held short calls to option_eod and the dividend reader to option_candidates, by wrapping
// the constructors of the deps copy of f (option-candidates Decisions 11 and 12):
//   - with OT_PG_DSN_ACCOUNT set, option_eod is built by NewWithHeldSymbols over a reader of the account store;
//   - with OT_PG_DSN_EVENTS set, option_candidates is built by NewWithCalls over a reader of the dividend store;
//     without it the puts job stays as it was and one line names OT_PG_DSN_EVENTS.
//
// Both stores open on first read and are closed through extra. No error or line holds a DSN.
func wireHeldCalls(cfg runOnceConfig, f *jobFactory, extra *closerSet, log *slog.Logger) {
	if f.d.newOptionEodHeld != nil && f.d.openAccount != nil && cfg.accountDSN != "" {
		rd := &lazyAccountReader{lazyStore[store.AccountStore]{open: func(ctx context.Context) (store.AccountStore, error) {
			s, err := f.d.openAccount(ctx, cfg.accountDSN)
			if err != nil {
				return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
			}
			return s, nil
		}}}
		extra.addCloser(rd)
		held := f.d.newOptionEodHeld
		f.d.newOptionEodWatch = func(b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter, watch []jobs.WatchEntry,
			clk clock.Clock, tr symbol.Translator) (contracts.Job, error) {
			return held(b, r, w, watch, rd, clk, tr)
		}
	}
	if f.d.newOptionCandidatesCalls == nil || f.d.openDividend == nil || cfg.candidatesOff() != "" {
		return
	}
	if cfg.eventsDSN == "" {
		log.Warn("covered calls left out", "component", "ot", "job", jobOptionCandidates, "status", reasonNoEventsDSN)
		return
	}
	rd := &lazyDividendReader{lazyStore[dividend.Store]{open: func(ctx context.Context) (dividend.Store, error) {
		s, err := f.d.openDividend(ctx, cfg.eventsDSN)
		if err != nil {
			return nil, fmt.Errorf("open dividend store with %s failed", store.DSNEnvEvents)
		}
		return s, nil
	}}}
	extra.addCloser(rd)
	calls := f.d.newOptionCandidatesCalls
	f.d.newOptionCandidates = func(st optioncandidates.CandidateStores, rule compute.OptionCandidates, watch []jobs.WatchEntry,
		clk clock.Clock) (contracts.Job, error) {
		return calls(st, rd, rule, watch, clk)
	}
}

// secBaseURL is the SEC host of the company facts; the source appends the path itself.
const secBaseURL = "https://data.sec.gov"

// newSEC builds the SEC source with mk, or answers nil, nil when userAgent is empty (the fundamentals job is then
// left out). The user agent comes from the environment through the caller; no error carries it.
func newSEC(mk sources.NewSECFunc, userAgent string, clk clock.Clock) (sources.FactSource, error) {
	if userAgent == "" || mk == nil {
		return nil, nil
	}
	src, err := mk(sources.SECConfig{BaseURL: secBaseURL, UserAgent: userAgent, Clock: clk})
	if err != nil {
		return nil, fmt.Errorf("build SEC source failed: %w", err)
	}
	return src, nil
}

// newCompaniesPage returns base with the fields entry 24 leaves nil set: the facts, the series, the seven compute
// functions, and Spot and Offers through the instrument id of the symbol (Decision 13). A nil reader leaves its
// field unset, which the page shows as a gap. Held, Text and Override stay as base has them.
func newCompaniesPage(base companies_page.Config, in companiesInputs, fn valuationFuncs) companies_page.Config {
	cfg := base
	if in.Facts != nil {
		cfg.Facts = in.Facts
	}
	if in.Series != nil {
		cfg.Series = in.Series
	}
	cfg.StandardOf, cfg.BuildFilings, cfg.Band, cfg.ImpliedGrowth = fn.StandardOf, fn.BuildFilings, fn.Band, fn.ImpliedGrowth
	cfg.ReturnAtPrice, cfg.Questions, cfg.Diagnosis = fn.ReturnAtPrice, fn.Questions, fn.Diagnosis
	if in.Catalog != nil && in.Names != nil {
		res := &symbolIDs{catalog: in.Catalog, names: in.Names}
		if in.Quotes != nil {
			cfg.Spot = spotOf(res, in.Quotes)
		}
		if in.Eod != nil && in.Terms != nil {
			cfg.Offers = offersOf(res, in.Eod, in.Terms)
		}
	}
	return cfg
}
