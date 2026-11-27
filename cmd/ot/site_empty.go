package main

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// noSiteRows is the one reader of a site that has none of a store: every read answers no row, so a page shows its
// gap, not an error. It stands in for a nil reader of a page whose builder rejects nil.
type noSiteRows struct{}

func (noSiteRows) Current(context.Context) ([]underlying.Row, error)      { return nil, nil }
func (noSiteRows) History(context.Context, int) ([]underlying.Row, error) { return nil, nil }

func (noSiteRows) PriceBars(context.Context, int64, string, contracts.Day, contracts.Day) ([]contracts.PriceBar, error) {
	return nil, nil
}

func (noSiteRows) FactsWithPeriodFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	return nil, nil
}

func (noSiteRows) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return nil, nil
}

func (noSiteRows) OptionEodLatest(context.Context, contracts.Day) ([]store.OptionEodLatest, error) {
	return nil, nil
}

func (noSiteRows) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return nil, nil
}
