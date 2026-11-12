package main

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/home_companies"
	"github.com/andrewrong/optdesk/internal/pages/home_household"
	"github.com/andrewrong/optdesk/internal/pages/home_macro"
	"github.com/andrewrong/optdesk/internal/pages/home_market"
	"github.com/andrewrong/optdesk/internal/pages/home_options"
	"github.com/andrewrong/optdesk/internal/pages/home_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/pages-real-data.md#17 ot-home-route.

// siteHomeRoute is the address of the page home_page under the site root; `/` redirects to it.
const siteHomeRoute = "home"

// homeName is the name of the page home_page, which pageNamed selects for ot page.
const homeName = "home_page"

func init() {
	registerPage(pageFamily{name: homeName, route: siteHomeRoute, order: 100, build: buildHome, named: homePageNamed})
}

// homeReadings is the reading function of the market and macro members: the winner readings of the view of
// the site (the demo view with the demo profile); with no view it answers no row, so each figure is a gap.
func homeReadings(a *app, rd siteReaders) compute.ReadingFunc {
	if a.profile.Name == "demo" && rd.view == nil {
		return demoSiteMarketReadings
	}
	if rd.view == nil {
		return func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
			return nil, nil
		}
	}
	return rd.view.WinnerReadings
}

// noSeries is the series reader of a site with none: no point, so the market member shows a gap per index.
type noSeries struct{}

func (noSeries) SeriesPoints(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SeriesPoint, error) {
	return nil, nil
}

// homeBuild builds the five members and composes the page for day.
func homeBuild(ctx context.Context, a *app, runID string, rd siteReaders, day contracts.Day, now time.Time) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(homeName + ": no zone")
	}
	const scope = "home"
	build := buildIDOf(version)
	zone := a.profile.Zone
	read := homeReadings(a, rd)
	rows := []home_companies.Row{}
	subjects, err := companiesSubjects(ctx, a, rd)
	if err != nil {
		return nil, err
	}
	held := map[string]bool{}
	if a.profile.Name != "demo" {
		if held, err = heldSymbols(ctx, rd, now); err != nil {
			return nil, err
		}
	}
	for _, s := range subjects {
		if a.profile.Name != "demo" && !held[s.Symbol] {
			continue
		}
		rows = append(rows, home_companies.Row{Issuer: s.Issuer, Market: s.Market, Symbol: s.Symbol, HandPrice: s.HandPrice})
	}
	hh, err := home_household.Build(ctx, home_household.Config{
		RunID: runID, BuildID: build, Scope: scope, Zone: zone, Alias: rd.aliasList()[0],
		Account: rd.noticeAccount, Sessions: rd.sessions, Figures: pagefig.New,
	}, day, now)
	if err != nil {
		return nil, err
	}
	cc := home_companies.Config{Zone: zone, Today: func() contracts.Day { return day }, RunID: runID, BuildID: build, Scope: scope, Rows: rows}
	if rd.view != nil {
		cc.Read = rd.view
	}
	if rd.earnings != nil {
		cc.Earnings = rd.earnings
	}
	co, err := home_companies.Build(ctx, cc, now)
	if err != nil {
		return nil, err
	}
	ma, err := home_macro.Build(ctx, home_macro.Config{RunID: runID, BuildID: build, Scope: scope, Zone: zone, Read: read}, day, now)
	if err != nil {
		return nil, err
	}
	series := rd.series
	if series == nil {
		series = noSeries{}
	}
	mk, err := home_market.Build(ctx, home_market.Config{RunID: runID, BuildID: build, Scope: scope, Zone: zone, Readings: read}, series, day, now)
	if err != nil {
		return nil, err
	}
	op, err := home_options.Build(ctx, home_options.Config{
		RunID: runID, BuildID: build, Scope: scope, Zone: zone,
		Ledger: rd.optionLedger, Replay: rd.ledgers, Results: rd.results,
	}, day, now)
	if err != nil {
		return nil, err
	}
	return home_page.Compose(home_page.Input{
		RunID: runID, BuildID: build, Scope: scope, Zone: zone,
		Title: "Today", Say: "Five cards, one page.",
		CardLinks: home_page.CardLinks{
			Macro: "/macro", Market: "/" + siteMarketRoute, Household: "/" + siteHouseholdRoute,
			Companies: "/" + siteCompaniesRoute, Options: "/" + siteOptionRoute,
		},
		ReportLinks: []home_page.Link{},
		Household:   hh, Companies: co, Macro: ma, Market: mk, Options: op,
	}, now)
}

// buildHome builds home_page per request for the day of dayFlag or today.
func buildHome(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return homeBuild(ctx, a, "ot-serve", rd, day, clk.Now().UTC())
}

// homePageNamed returns home_page for ot page and a close function: the stores opened once as the site opens
// them (no reader with a missing line of .env, the gap pages of the members).
func homePageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(homeName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	rd := siteReaders{}
	closeFn := func() {}
	open := a.profile.Name == "demo"
	if !open {
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		open = env[pagesDSNName] != ""
	}
	if open {
		var err error
		if rd, closeFn, err = openSiteReaders(ctx, a); err != nil {
			return pages.Page{}, nil, err
		}
	}
	build := func(ctx context.Context, ser store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		r := rd
		if ser != nil {
			r.series = ser
		}
		return homeBuild(ctx, a, "ot-page", r, day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: homeName, Schema: "page.home_page.v3"}, Build: build}, closeFn, nil
}
