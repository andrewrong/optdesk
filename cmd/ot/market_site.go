package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
)

// siteMarketRoute is the address of the page market_page under the site root. It is not an item of the
// bar of the site: it is reached by its address (docs/design/market-page-producer.md, Decision 8).
const siteMarketRoute = "market"

// buildMarket builds market_page per request for the day of dayFlag or today, from the winner readings of
// the reader of the site (rd.view, opened once per process) and from its series reader; the demo profile
// reads fixed demo readings instead and opens nothing. A nil zone is an error.
func buildMarket(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New("market_page: no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	readings := rd.view.WinnerReadings
	if a.profile.Name == "demo" {
		readings = demoSiteMarketReadings
	}
	return newSiteMarketPage(a, readings).Build(ctx, rd.series, day, clk.Now().UTC())
}

// newSiteMarketPage calls deps.marketPage with the plain arguments (run id "ot-serve", scope "market").
func newSiteMarketPage(a *app, readings compute.ReadingFunc) pages.Page {
	return a.deps.marketPage(a.profile.Zone, "ot-serve", buildIDOf(version), "market", readings)
}

// demoSiteFigures are the five figures of market_page with the value of their one demo reading.
var demoSiteFigures = map[string]string{
	"market.vix_pctile_3y": "0.5", "market.vxn_pctile_3y": "0.5", "market.rvx_pctile_3y": "0.5",
	"market.sp500_vs_200d": "0.031200", "market.vix_term_ratio": "0.950000",
}

// demoSiteMarketReadings is the ReadingFunc of the market route in the demo profile: one fixed ok
// reading of day to, carrying the parameter data_day, for each of the five figures; others have no row.
func demoSiteMarketReadings(_ context.Context, name, subjectKind, subject string, from, to contracts.Day) ([]contracts.ReadingRecord, error) {
	v, ok := demoSiteFigures[name]
	if !ok || to < from {
		return nil, nil
	}
	d, err := time.Parse(dayLayout, string(to))
	if err != nil {
		return nil, err
	}
	rec := contracts.ReadingRecord{
		Name: name, SubjectKind: subjectKind, Subject: subject,
		Day: types.SerializableDate{Time: d}, Rev: 1, Version: 1,
		State: "ok", Unit: "decimal", Value: &v,
	}
	param, err := json.Marshal([]map[string]any{{"param": "data_day", "value_num": nil, "value_tag": string(to)}})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(param, &rec.Parameters); err != nil {
		return nil, err
	}
	return []contracts.ReadingRecord{rec}, nil
}
