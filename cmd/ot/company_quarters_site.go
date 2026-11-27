package main

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/companydata"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/company_quarters_page"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/data-collection.md#8.

// siteCompanyQuartersRoute is the address of the page company_quarters_page under the site root.
const siteCompanyQuartersRoute = "company-quarters"

// companyQuartersName is the name of the page for ot page.
const companyQuartersName = "company_quarters_page"

// companyQuartersQuery is the query parameter that names the company, as the page's own links write it.
const companyQuartersQuery = "c"

func init() {
	registerPage(pageFamily{name: companyQuartersName, route: siteCompanyQuartersRoute, order: 180, build: buildCompanyQuarters, named: companyQuartersPageNamed})
}

// buildCompanyQuarters builds company_quarters_page per request; the company is the query parameter c.
func buildCompanyQuarters(ctx context.Context, _ *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	in := company_quarters_page.Input{
		Now: clk.Now().UTC(), Today: day, Key: queryOf(ctx).Get(companyQuartersQuery),
		Underlyings: rd.underlying, Facts: companyFacts(rd), Calc: companydata.Calc{},
	}
	// a reader the site has not opened is an empty one: the page shows its gaps
	if in.Underlyings == nil {
		in.Underlyings = noSiteRows{}
	}
	if in.Facts == nil {
		in.Facts = noSiteRows{}
	}
	return company_quarters_page.Build(ctx, in)
}

// companyQuartersPageNamed gives the page to ot page: no company is chosen, so it lists the companies.
func companyQuartersPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	rd, closeFn, err := companyNamedReaders(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return company_quarters_page.Build(ctx, company_quarters_page.Input{
			Now: now.UTC(), Today: day, Underlyings: rd.underlying, Facts: companyFacts(rd), Calc: companydata.Calc{},
		})
	}
	return pages.Page{Meta: pages.Meta{Name: companyQuartersName, Schema: "page.company_quarters_page.v1"}, Build: build}, closeFn, nil
}
