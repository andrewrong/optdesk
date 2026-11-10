package main

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/household_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/pages-real-data.md#1 ot-household-route.

// siteHouseholdRoute is the address of the page household_page under the site root.
const siteHouseholdRoute = "household"

// householdName is the name of the page household_page, which pageNamed selects for ot page.
const householdName = "household_page"

func init() {
	registerPage(pageFamily{name: householdName, route: siteHouseholdRoute, order: 60, build: buildHousehold, named: householdPageNamed})
}

// householdConfig is the config of household_page from the readers of the site; a reader that is nil gives
// the gap page no_reader (the demo profile has no session reader).
func householdConfig(a *app, rd siteReaders) household_page.Config {
	return household_page.Config{
		RunID: "ot-serve", BuildID: buildIDOf(version), Zone: a.profile.Zone, Aliases: siteAliases,
		Account: rd.noticeAccount, Ledger: rd.optionLedger, Sessions: rd.sessions, Ledgers: rd.ledgers,
		Figures: pagefig.New, Marks: rd.marks, Holdings: pagefig.NewHoldings,
	}
}

// buildHousehold builds household_page per request for the day of dayFlag or today and the account of the
// query "account" (the first alias when absent); an alias not in the list gives the gap page no_data.
func buildHousehold(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(householdName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	alias := queryOf(ctx).Get(siteAccountQuery)
	if alias == "" {
		alias = siteAliases[0]
	}
	return household_page.Build(ctx, householdConfig(a, rd), alias, day, clk.Now().UTC())
}

// householdPageNamed returns household_page of the main account for ot page and a close function: the stores
// opened once as the site opens them (no reader with a missing line of .env, the gap no_reader).
func householdPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(householdName + " needs the owner's zone: " + zoneEnvName + " is not set")
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
	cfg := householdConfig(a, rd)
	cfg.RunID = "ot-page"
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return household_page.Build(ctx, cfg, siteAliases[0], day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: householdName, Schema: "page.household_page.v1"}, Build: build}, closeFn, nil
}
