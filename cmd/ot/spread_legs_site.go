package main

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegs"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/structures_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/spread-legs.md#3-legs-3-page.

// siteSpreadLegsRoute is the address of the page spread_legs_page under the site root.
const siteSpreadLegsRoute = "options/spread-legs"

// spreadLegsName is the name of the page spread_legs_page, which pageNamed selects for ot page.
const spreadLegsName = "spread_legs_page"

func init() {
	registerPage(pageFamily{name: spreadLegsName, route: siteSpreadLegsRoute, order: 95, build: buildSpreadLegs, named: spreadLegsPageNamed})
}

// spreadLegsConfig is the config of the page from the readers of the site, the same the structures pages are
// built with (site_positions.go), and the figure of the spread legs.
func spreadLegsConfig(a *app, rd siteReaders) structures_page.Config {
	return structures_page.Config{RunID: "ot-serve", BuildID: buildIDOf(version), Zone: a.profile.Zone, Aliases: rd.aliasList(),
		Ledger: rd.ledger, Account: rd.account, Eod: rd.eod, Sessions: rd.sessions, Terms: rd.terms,
		Ledgers: rd.ledgers, Structures: rd.structures, StockIDs: structuresStockIDs(rd),
		Bars: rd.bars, Dividends: rd.dividends, Spreads: spreadlegs.New}
}

// spreadLegsDemo is the page of the demo profile: fixed rows, invented names and numbers, no read.
func spreadLegsDemo(a *app, alias string, day contracts.Day, at time.Time) ([]byte, error) {
	dayAt := string(day) + "T00:00:00Z"
	if a.profile.Zone != nil {
		if d, err := time.ParseInLocation(dayLayout, string(day), a.profile.Zone); err == nil {
			dayAt = d.Format(time.RFC3339)
			at = at.In(a.profile.Zone)
		}
	}
	value := func(text string, v float64) map[string]any {
		return map[string]any{"kind": "value", "text": text, "value": v, "unit": "usd"}
	}
	leg := func(side, strike, qty, open string) map[string]any {
		return map[string]any{"side_text": side, "strike": strike, "expiry_text": "2026-12-18", "quantity": qty, "open_price": open}
	}
	spread := map[string]any{
		"symbol": "DEMOA", "href": "/options/structures/demo-put-spread", "kind_text": "Put spread",
		"kind_gloss": "Sold higher put, bought lower put", "expiry_text": "Expiry 2026-12-18 (27 days)",
		"legs":       []any{leg("Short put", "95.00", "-2", "2.40"), leg("Long put", "90.00", "+2", "1.10")},
		"net_credit": value("+260.00 USD", 260), "max_loss": value("-740.00", -740), "max_gain": value("+260.00", 260),
		"break_even": map[string]any{"kind": "values", "text": "93.70", "values": []any{93.7}},
		"reason":     nil,
	}
	page := map[string]any{
		"schema": spreadLegsName, "schema_version": 1, "run_id": "ot-serve", "build_id": buildIDOf(version), "scope": alias,
		"generated_at": at.Format(time.RFC3339), "data_times": []any{}, "run_status": "ok",
		"stale": map[string]any{"data_time": dayAt, "text": "Demo profile: invented names and numbers", "is_stale": false},
		"title": "Spread legs", "sentence": "Demo profile: these rows are fixed and nothing was read.", "note": nil,
		"groups": []any{
			map[string]any{"title": "Spreads", "structures": []any{spread}},
			map[string]any{"title": "Unidentified", "structures": []any{}},
		},
	}
	return json.Marshal(page)
}

// buildSpreadLegs builds spread_legs_page per request for the day of dayFlag or today and the account of the
// query "account" (the first alias when absent); an alias not in the list gives the gap page, and the demo
// profile answers fixed rows with no read.
func buildSpreadLegs(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(spreadLegsName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	alias := queryOf(ctx).Get(siteAccountQuery)
	if alias == "" {
		alias = rd.aliasList()[0]
	}
	if a.profile.Name == "demo" && slices.Contains(rd.aliasList(), alias) {
		return spreadLegsDemo(a, alias, day, clk.Now().UTC())
	}
	return structures_page.NewSpreadsPage(spreadLegsConfig(a, rd), alias).Build(ctx, nil, day, clk.Now().UTC())
}

// spreadLegsPageNamed returns the page of the main account for ot page and a close function: the stores opened
// once as the site opens them (no reader with a missing line of .env, the gap no_reader).
func spreadLegsPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(spreadLegsName + " needs the owner's zone: " + zoneEnvName + " is not set")
	}
	rd := siteReaders{}
	closeFn := func() {}
	demo := a.profile.Name == "demo"
	open := false
	if !demo {
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
	cfg := spreadLegsConfig(a, rd)
	cfg.RunID = "ot-page"
	alias := rd.aliasList()[0]
	build := func(ctx context.Context, s store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		if demo {
			return spreadLegsDemo(a, alias, day, now)
		}
		return structures_page.NewSpreadsPage(cfg, alias).Build(ctx, s, day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: spreadLegsName, Schema: "page.spread_legs_page.v1"}, Build: build}, closeFn, nil
}
