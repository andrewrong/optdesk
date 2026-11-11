package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/pagefig"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/risk_page"
)

// Design: docs/design/pages-real-data.md#10 ot-risk-route.

// siteRiskRoute is the address of the page household_risk under the site root.
const siteRiskRoute = "risk"

// riskName is the name of the page household_risk, which pageNamed selects for ot page.
const riskName = "household_risk"

func init() {
	registerPage(pageFamily{name: riskName, route: siteRiskRoute, order: 80, build: buildRisk, named: riskPageNamed})
}

// riskConfig is the config of household_risk from the readers of the site; a reader that is nil gives the
// gap page no_reader.
func riskConfig(a *app, rd siteReaders) risk_page.Config {
	return risk_page.Config{
		RunID: "ot-serve", BuildID: buildIDOf(version), Zone: a.profile.Zone, Aliases: rd.aliasList(),
		Account: rd.noticeAccount, Ledger: rd.optionLedger, Sessions: rd.sessions, Ledgers: rd.ledgers,
		Figures: pagefig.New, Holdings: pagefig.NewHoldings,
		Structures: rd.structures, Eod: rd.eod, Terms: rd.terms,
	}
}

// buildRisk builds household_risk per request for the day of dayFlag or today and the account of the query
// "account" (the first alias when absent).
func buildRisk(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	if a.profile.Zone == nil {
		return nil, errors.New(riskName + ": no zone")
	}
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	alias := queryOf(ctx).Get(siteAccountQuery)
	if alias == "" {
		alias = rd.aliasList()[0]
	}
	return risk_page.NewPage(riskConfig(a, rd), alias).Build(ctx, nil, day, clk.Now().UTC())
}

// riskPageNamed returns household_risk of the main account for ot page and a close function, opening the
// site readers as householdPageNamed does.
func riskPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	if a.profile.Zone == nil {
		return pages.Page{}, nil, errors.New(riskName + " needs the owner's zone: " + zoneEnvName + " is not set")
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
	cfg := riskConfig(a, rd)
	cfg.RunID = "ot-page"
	return risk_page.NewPage(cfg, rd.aliasList()[0]), closeFn, nil
}
