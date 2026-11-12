package main

import (
	"context"
	"errors"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_boards"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/option-page-builders.md#6-stock-option-page-route.

// siteStockRoute is the prefix of the one-stock option page: options/stock and options/stock/<key>.
const siteStockRoute = "options/stock"

func init() {
	// No name and no named: the page needs a key, so ot page --name does not know it.
	registerPage(pageFamily{route: siteStockRoute, buildKey: stockBuild})
}

// stockBuild builds stock_option_page per request for the company key (the issuer of a stored row; "" takes the
// first ok row of the data day) and the day of dayFlag or today, from the candidate, option_eod and name readers
// of the site and the reading reader of the site (rd.view, for vol_view), with the overlay reader of the open and
// refresh rows (rd.openQuotes) and the state of the refresh button of its stock; a nil reader gives the page its own gap. A nil zone is an error.
func stockBuild(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders, key string) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("stock_option_page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return option_boards.NewStockPage(option_boards.StockConfig{
		RunID: "ot-serve", BuildID: buildIDOf(version), Scope: "stock", Zone: a.profile.Zone,
		Candidates: rd.cands, Eod: rd.eod, Names: rd.names, Readings: rd.view, Key: key,
		Quotes: rd.openQuotes(), Refresh: rd.live.state(ctx, stockSymbols(ctx, rd, day, key)),
	}).Build(ctx, nil, day, clk.Now().UTC())
}

// stockSymbols is the ticker of the company the page shows: the underlying of the first ok short put whose ticker
// or issuer is key ("" takes the first ok row). At most one symbol.
func stockSymbols(ctx context.Context, rd siteReaders, day contracts.Day, key string) []string {
	upper := strings.ToUpper(key)
	out := cardSymbols(ctx, rd, day, func(c store.Candidate, ticker string) bool {
		return key == "" || ticker == upper || c.Issuer == key
	})
	if len(out) > 1 {
		out = out[:1]
	}
	return out
}
