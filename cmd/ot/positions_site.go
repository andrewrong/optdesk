package main

import (
	"context"
	"errors"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/positions_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/pages-real-data.md#8 ot-positions-route.

// sitePositionsRoute is the address of the page positions_page under the site root.
const sitePositionsRoute = "positions"

// positionsName is the name of the page positions_page, which pageNamed selects for ot page.
const positionsName = "positions_page"

func init() {
	registerPage(pageFamily{name: positionsName, route: sitePositionsRoute, order: 90, build: buildPositionsPage, named: positionsPageNamedOf})
}

// positionsPageConfig is the config of positions_page from the readers of the site; a reader that is nil gives
// the gap page no_reader (the demo profile has no session reader).
func positionsPageConfig(a *app, rd siteReaders) positions_page.Config {
	return positions_page.Config{
		RunID: "ot-serve", BuildID: buildIDOf(version), Zone: a.profile.Zone, Aliases: rd.aliasList(),
		Account: rd.noticeAccount, Ledger: rd.optionLedger, Sessions: rd.sessions, Ledgers: rd.ledgers,
		Marks: rd.marks, Figures: pagefig.New, Holdings: pagefig.NewHoldings,
		Statement: pageStatementAliases(a),
	}
}

// buildPositionsPage builds positions_page per request for the day of dayFlag or today and the account of the
// query "account" (the first alias when absent); an alias not in the list gives the gap page no_data.
func buildPositionsPage(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(positionsName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	alias := queryOf(ctx).Get(siteAccountQuery)
	if alias == "" {
		alias = rd.aliasList()[0]
	}
	return positions_page.Build(ctx, positionsPageConfig(a, rd), alias, day, clk.Now().UTC())
}

// positionsPageNamedOf returns positions_page of the main account for ot page and a close function: the stores
// opened once as the site opens them (no reader with a missing line of .env, the gap no_reader).
func positionsPageNamedOf(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(positionsName + " needs the owner's zone: " + zoneEnvName + " is not set")
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
	cfg := positionsPageConfig(a, rd)
	cfg.RunID = "ot-page"
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return positions_page.Build(ctx, cfg, rd.aliasList()[0], day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: positionsName, Schema: "page.positions_page.v3"}, Build: build}, closeFn, nil
}
