package main

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute/companydata"
	"github.com/andrewrong/optdesk/internal/compute/companydataapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/company_data_page"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Design: docs/design/data-collection.md#8.

// siteCompanyDataRoute is the address of the page company_data_page under the site root.
const siteCompanyDataRoute = "company-data"

// companyDataName is the name of the page for ot page.
const companyDataName = "company_data_page"

func init() {
	registerPage(pageFamily{name: companyDataName, route: siteCompanyDataRoute, order: 190, build: buildCompanyData, named: companyDataPageNamed})
}

// companyFacts is the fact reader of the site as the period reader of the pages: the one siteFactsKey opened with
// the pages line (role ot_ro_pages); nil when there is none or it reads no period fields.
func companyFacts(rd siteReaders) companydataapi.PeriodFactReader {
	f, _ := siteReaderOf[store.FactReader](rd, siteFactsKey)
	p, _ := f.(companydataapi.PeriodFactReader)
	return p
}

// buildCompanyData builds company_data_page per request over the readers of the site.
func buildCompanyData(ctx context.Context, _ *app, clk clock.Clock, dayFlag string, rd siteReaders) ([]byte, error) {
	day, err := dayOrDefault(clk, dayFlag)
	if err != nil {
		return nil, err
	}
	return company_data_page.Build(ctx, company_data_page.Input{
		Now: clk.Now().UTC(), Today: day, Underlyings: rd.underlying, Bars: rd.bars, Facts: companyFacts(rd),
		Earnings: rd.earnings, Calc: companydata.Calc{},
	})
}

// companyDataPageNamed gives the page to ot page: the demo profile reads no database and shows no company.
func companyDataPageNamed(ctx context.Context, a *app) (pages.Page, func(), error) {
	rd, closeFn, err := companyNamedReaders(ctx, a)
	if err != nil {
		return pages.Page{}, nil, err
	}
	build := func(ctx context.Context, _ store.SeriesReader, day contracts.Day, now time.Time) ([]byte, error) {
		return company_data_page.Build(ctx, company_data_page.Input{
			Now: now.UTC(), Today: day, Underlyings: rd.underlying, Bars: rd.bars, Facts: companyFacts(rd),
			Earnings: rd.earnings, Calc: companydata.Calc{},
		})
	}
	return pages.Page{Meta: pages.Meta{Name: companyDataName, Schema: "page.company_data_page.v1"}, Build: build}, closeFn, nil
}

// companyNamedReaders is the readers of the two pages for ot page: with the demo profile empty ones and no store
// opened, else the site readers opened with the pages line.
func companyNamedReaders(ctx context.Context, a *app) (siteReaders, func(), error) {
	if a.profile.Name == "demo" {
		return siteReaders{
			underlying: emptyCompanyUnderlying{}, bars: emptyCompanyBars{}, earnings: emptyCompanyEarnings{},
			ext: map[string]any{siteFactsKey: store.FactReader(emptyCompanyFacts{})},
		}, func() {}, nil
	}
	return openSiteReaders(ctx, a)
}

// The demo readers hold nothing.
type (
	emptyCompanyUnderlying struct{}
	emptyCompanyBars       struct{}
	emptyCompanyEarnings   struct{}
	emptyCompanyFacts      struct{}
)

func (emptyCompanyUnderlying) Current(context.Context) ([]underlying.Row, error) { return nil, nil }
func (emptyCompanyUnderlying) History(context.Context, int) ([]underlying.Row, error) {
	return nil, nil
}

func (emptyCompanyBars) PriceBars(context.Context, int64, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
	return nil, nil
}

func (emptyCompanyEarnings) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return nil, nil
}

func (emptyCompanyFacts) FactsFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	return nil, nil
}

func (emptyCompanyFacts) FactsWithPeriodFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	return nil, nil
}
