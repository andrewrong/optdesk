package main

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/optionattr"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/pages/option_pnl_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/optionday"
	"github.com/andrewrong/optdesk/internal/strategy/risk/bsm"
)

// siteOptionPnlRoute is the address of the page option_pnl_page under the site root.
const siteOptionPnlRoute = "options/pnl"

// pnlName is the name of the page option_pnl_page, which pageNamed selects for ot page.
const pnlName = "option_pnl_page"

// pnlMaxRateAgeDays is the oldest DGS3MO rate, in days, the attribution may use. It is the default of the
// settings key max_rate_age_days of internal/compute/ruleparam/registry.go; the page does not read the setting.
const pnlMaxRateAgeDays = 7

// pnlReaders are the readers of option_pnl_page, each nil when its store did not open (the page then shows its
// own gap). Series is the rate reader passed to the page's Build.
type pnlReaders struct {
	Ledger   store.OptionLedgerReader
	Sessions store.SessionDayReader
	Days     optionday.DayReader
	StockIDs option_pnl_page.StockIDReader
	Bars     bar.PriceBarReader
	Series   store.SeriesReader
}

// pnlReadersOf picks the readers of the page from the readers of the site; the stock ids come from the site
// extension under siteMarketStateIDsKey (rd.names reads names by id, not ids by name).
func pnlReadersOf(rd siteReaders) pnlReaders {
	var ledger store.OptionLedgerReader
	if rd.optionLedger != nil {
		ledger = aliasedLedger{OptionLedgerReader: rd.optionLedger, configured: rd.aliasList()}
	}
	ids, _ := siteReaderOf[market_state_page.StockIDReader](rd, siteMarketStateIDsKey)
	var stocks option_pnl_page.StockIDReader
	if ids != nil {
		stocks = ids
	}
	return pnlReaders{Ledger: ledger, Sessions: rd.sessions, Days: rd.days, StockIDs: stocks, Bars: rd.bars, Series: rd.series}
}

// pnlPageFunc builds option_pnl_page from its Config.
type pnlPageFunc func(c option_pnl_page.Config) pages.Page

// pnlConfigOf is the one place that fills option_pnl_page.Config: the readers and the compute rules.
func pnlConfigOf(a *app, runID, scope string, r pnlReaders) option_pnl_page.Config {
	return option_pnl_page.Config{
		RunID: runID, BuildID: buildIDOf(version), Scope: scope, Zone: a.profile.Zone,
		Ledger: r.Ledger, Sessions: r.Sessions, Days: r.Days, StockIDs: r.StockIDs, Bars: r.Bars,
		Replay: optionledger.New(), Attributor: optionattr.New(newBSMPricer()), Booker: optionattr.NewBooker(),
		MarkMaxAgeSessions: 3, RecentDays: 30, MaxRateAgeDays: pnlMaxRateAgeDays, CalendarID: "XNYS",
	}
}

// bsmPricer adapts the Black-Scholes pricer of the strategy module to optionattrapi.Pricer: it holds the pricer
// and delegates each method, so cmd/ot is the only place that joins the two modules.
type bsmPricer struct{ p bsm.Pricer }

var _ optionattrapi.Pricer = bsmPricer{}

// newBSMPricer returns the adapter as the interface, so the argument of optionattr.New has the static type
// optionattrapi.Pricer.
func newBSMPricer() optionattrapi.Pricer { return bsmPricer{p: bsm.New()} }

// Value delegates to the held pricer; its error, with NoSolution, is returned unchanged.
func (b bsmPricer) Value(right, spot, strike, years, rate, vol string) (string, error) {
	return b.p.Value(right, spot, strike, years, rate, vol)
}

// Implied delegates to the held pricer; its error, with NoSolution, is returned unchanged.
func (b bsmPricer) Implied(right, spot, strike, years, rate, price string) (string, error) {
	return b.p.Implied(right, spot, strike, years, rate, price)
}

// newOptionPnlPage is the default of deps.optionPnlPage.
func newOptionPnlPage(c option_pnl_page.Config) pages.Page { return option_pnl_page.NewPage(c) }

func init() {
	registerPage(pageFamily{name: pnlName, route: siteOptionPnlRoute, order: 35, build: buildPnl, named: pnlPageNamed})
}

// buildPnl builds option_pnl_page per request for the day of dayFlag or today in the zone of the profile. A nil
// zone is an error.
func buildPnl(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(pnlName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	r := pnlReadersOf(rd)
	return a.deps.optionPnlPage(pnlConfigOf(a, "ot-serve", "pnl", r)).Build(ctx, r.Series, day, clk.Now().UTC())
}

// pnlPageNamed returns option_pnl_page for ot page and a close function, as trackingPageNamed does. The page
// it returns passes the series reader of the stores to Build. A nil zone is an error raised before any store is opened.
func pnlPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(pnlName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	if a.profile.Name != "demo" {
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		if env[pagesDSNName] == "" {
			return a.deps.optionPnlPage(pnlConfigOf(a, "ot-page", "pnl", pnlReaders{})), func() {}, nil
		}
	}
	rd, closeFn, err := openSiteReaders(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	r := pnlReadersOf(rd)
	p := a.deps.optionPnlPage(pnlConfigOf(a, "ot-page", "pnl", r))
	inner := p.Build
	p.Build = func(ctx context.Context, _ store.SeriesReader, day contracts.Day, at time.Time) ([]byte, error) {
		return inner(ctx, r.Series, day, at)
	}
	return p, closeFn, nil
}
