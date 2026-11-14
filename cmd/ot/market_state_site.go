package main

import (
	"context"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/liquidity"
	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/market_state_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/market-state.md#Decisions (Decisions 1, 7 and 13).

// siteMarketStateRoute is the address of the page market_state_page under the site root.
const siteMarketStateRoute = "market-state"

// marketStateName is the name of the page market_state_page, which pageNamed selects for ot page.
const marketStateName = "market_state_page"

// siteMarketStateIDsKey is the key of the stock id reader that the site opener puts in siteReaders.ext.
const siteMarketStateIDsKey = "market_state_stock_ids"

func init() {
	registerPage(pageFamily{name: marketStateName, route: siteMarketStateRoute, order: 120, build: buildMarketState, named: marketStatePageNamed})
	siteOpeners[siteMarketStateIDsKey] = openMarketStateIDs
}

// openMarketStateIDs opens the stock id reader with the pages line, through the opener of the report. Without an
// opener or the line it gives no reader and goes on: the sector rows are then the gap no_reader.
func openMarketStateIDs(ctx context.Context, a *app, track func(io.Closer, error) bool) (any, bool) {
	open := reportStockIDOpener(a)
	if open == nil {
		return nil, true
	}
	env, err := a.envFile()
	if err != nil {
		return nil, false
	}
	if env[pagesDSNName] == "" {
		return nil, true
	}
	s, err := open(ctx, env[pagesDSNName])
	if !track(s, err) {
		return nil, false
	}
	return market_state_page.StockIDReader(s), true
}

// marketStateSymbols are the funds (group sector) and SMH (group industry) of the manifest of bars_refresh, in its
// order. An unreadable manifest gives none (the sector block is then a gap).
func marketStateSymbols() []market_state_page.Symbol {
	m, err := barsrefresh.DefaultManifest()
	if err != nil {
		return nil
	}
	var out []market_state_page.Symbol
	for _, s := range m.Stocks {
		switch s.Group {
		case barsrefresh.GroupSector:
			out = append(out, market_state_page.Symbol{Symbol: s.Symbol, Label: s.Label, Group: market_state_page.GroupSector})
		case barsrefresh.GroupIndustry:
			out = append(out, market_state_page.Symbol{Symbol: s.Symbol, Label: s.Label, Group: market_state_page.GroupIndustry})
		}
	}
	return out
}

// marketStateConfig is the Config of the page over the readers rd. The figure functions are the ones of the deps
// the compute_figures job is built with.
func marketStateConfig(a *app, rd siteReaders) market_state_page.Config {
	ids, _ := siteReaderOf[market_state_page.StockIDReader](rd, siteMarketStateIDsKey)
	c := market_state_page.Config{
		RunID: "ot-serve", BuildID: buildIDOf(version), Scope: "market_state", Zone: a.profile.Zone,
		Readings: homeReadings(a, rd), StockIDs: ids,
		Market: marketstate.Compute, Liquidity: liquidity.Compute,
		Figures: a.deps.computeFigures, Units: a.deps.computeUnits, Rules: a.deps.computeRules,
		MarketParams: marketstate.TrialParams(), LiquidityParams: liquidity.TrialParams(),
		Symbols: marketStateSymbols(),
	}
	if rd.bars != nil {
		c.Bars = rd.bars
	}
	return c
}

// buildMarketStateAt builds market_state_page for day. A site with no series reader reads no point.
func buildMarketStateAt(ctx context.Context, a *app, rd siteReaders, ser store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
	if ser == nil {
		ser = rd.series
	}
	if ser == nil {
		ser = noSeries{}
	}
	return market_state_page.Build(ctx, marketStateConfig(a, rd), ser, day, now)
}

// buildMarketState builds market_state_page per request for the day of dayFlag or today.
func buildMarketState(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return buildMarketStateAt(ctx, a, rd, nil, day, clk.Now().UTC())
}

// marketStatePageNamed returns market_state_page for ot page and a close function: the site readers opened once
// with the pages line of .env, no reader with a missing line or the demo profile.
func marketStatePageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	var rd siteReaders
	closeFn := func() {}
	if a.profile.Name != "demo" {
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		if env[pagesDSNName] != "" {
			r, closeAll, err := openSiteReaders(ctx, a)
			if err != nil {
				return pages.Page{}, nil, err
			}
			rd, closeFn = r, closeAll
		}
	}
	build := func(ctx context.Context, ser store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return buildMarketStateAt(ctx, a, rd, ser, day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: marketStateName, Schema: "page.market_state_page.v1"}, Build: build}, closeFn, nil
}
