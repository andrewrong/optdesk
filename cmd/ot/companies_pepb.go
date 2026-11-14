package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/compute/gonumstats"
	"github.com/andrewrong/optdesk/internal/compute/pepb"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/companies_page"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// Design: docs/design/company-pepb.md#6 company-pepb-wire.

// pepbBasis is the bar basis of the PE and PB history: the traded close, not the adjusted last.
const pepbBasis = "trades"

// barsOf returns the Bars function of the page: the stored trades bars of a symbol over the whole span asked.
// A symbol with no instrument id or a nil reader gives no bars, which the page shows as the Gap no_data.
func barsOf(res *symbolIDs, bars bar.PriceBarReader) func(context.Context, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
	return func(ctx context.Context, symbol string, from, to contracts.Day) ([]contracts.PriceBar, error) {
		if bars == nil {
			return nil, nil
		}
		id, ok, err := res.ID(ctx, symbol)
		if err != nil || !ok {
			return nil, err
		}
		return bars.PriceBars(ctx, id, pepbBasis, from, to)
	}
}

// withCompaniesPepb sets Pepb and Bars of cfg when the inputs have the bar reader, the catalog and the name
// reader; otherwise the page has no pepb property.
func withCompaniesPepb(cfg companies_page.Config, in companiesInputs) companies_page.Config {
	if in.Bars == nil || in.Catalog == nil || in.Names == nil {
		return cfg
	}
	cfg.Pepb = pepb.New(gonumstats.New())
	cfg.Bars = barsOf(&symbolIDs{catalog: in.Catalog, names: in.Names}, in.Bars)
	return cfg
}
