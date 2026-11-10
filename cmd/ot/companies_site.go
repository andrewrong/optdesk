package main

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/companies_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/valuation.md#companies-route-band.

// siteCompaniesRoute is the address of the page companies_page under the site root.
const siteCompaniesRoute = "companies"

// companiesName is the name of the page companies_page, which pageNamed selects for ot page.
const companiesName = "companies_page"

func init() {
	registerPage(pageFamily{name: companiesName, route: siteCompaniesRoute, order: 50, build: buildCompanies, named: companiesPageNamed})
}

// companiesDemoSubjects are the two fixed rows of the demo profile, each with a hand price.
var companiesDemoSubjects = []companies_page.Subject{
	{Issuer: "cik:0000000001", Market: "US", Symbol: "DEMOA", HandPrice: "85.5"},
	{Issuer: "cik:0000000002", Market: "US", Symbol: "DEMOB", HandPrice: "42"},
}

// companiesSubjects are the rows of the page: the demo rows with the demo profile, else one per entry of
// watch.toml (a broken file is an error, a missing one gives none).
func companiesSubjects(a *app) ([]companies_page.Subject, error) {
	if a.profile.Name == "demo" {
		return companiesDemoSubjects, nil
	}
	entries, err := loadWatch(a)
	if err != nil {
		return nil, err
	}
	out := make([]companies_page.Subject, 0, len(entries))
	for _, e := range entries {
		if e.Issuer == "" {
			continue
		}
		out = append(out, companies_page.Subject{Issuer: e.Issuer, Market: e.Market, Symbol: e.Contract.Symbol, HandPrice: e.BuyPrice})
	}
	return out, nil
}

// buildCompaniesAt builds companies_page for day from the view reader read (nil gives a gap). Only the price
// line is filled, with the hand price; no compute function is set, so no row has a band.
func buildCompaniesAt(ctx context.Context, a *app, read store.ReadingViewReader, day contracts.Day, now time.Time) ([]byte, error) {
	subjects, err := companiesSubjects(a)
	if err != nil {
		return nil, err
	}
	cfg := companies_page.Config{
		Zone: a.profile.Zone, Today: func() contracts.Day { return day }, Subjects: subjects,
	}
	if read != nil {
		cfg.Read = read
	}
	return companies_page.Build(ctx, cfg, now)
}

// buildCompanies builds companies_page per request for the day of dayFlag or today, from the view reader of the site.
func buildCompanies(ctx context.Context, a *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return buildCompaniesAt(ctx, a, rd.view, day, clk.Now().UTC())
}

// companiesPageNamed returns companies_page for ot page and a close function: the reading view opened once with
// the pages line of .env, no reader with a missing line, or the demo view with the demo profile.
func companiesPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	var read store.ReadingViewReader
	closeFn := func() {}
	switch a.profile.Name {
	case "demo":
		read = demoView{}
	default:
		env, err := a.envFile()
		if err != nil {
			return pages.Page{}, nil, err
		}
		if dsn := env[pagesDSNName]; dsn != "" {
			v, err := a.deps.openReadingView(ctx, dsn)
			if err != nil {
				return pages.Page{}, nil, fmt.Errorf("open reading view with %s failed", pagesDSNName)
			}
			read, closeFn = v, func() { _ = v.Close() }
		}
	}
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return buildCompaniesAt(ctx, a, read, day, now)
	}
	return pages.Page{Meta: pages.Meta{Name: companiesName}, Build: build}, closeFn, nil
}
