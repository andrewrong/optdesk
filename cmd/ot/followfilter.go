package main

import (
	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// The four job groups of docs/design/underlyings.md Decision 6.
const (
	groupBars         = "bars"
	groupDividends    = "dividends"
	groupOptions      = "options"
	groupFundamentals = "fundamentals"
)

// followedBy returns the groups that follow the name of r, in the order bars, dividends, options, fundamentals
// (Decision 6). held is whether the owner holds shares of it. Pure: no store, no broker.
func followedBy(r underlying.Row, held bool) []string {
	var groups []string
	switch r.Status {
	case underlying.StatusActive:
		groups = append(groups, groupBars)
		if r.Issuer != "" {
			groups = append(groups, groupDividends)
		}
		if r.Options {
			groups = append(groups, groupOptions)
		}
		if r.Fundamentals && r.Issuer != "" {
			groups = append(groups, groupFundamentals)
		}
	case underlying.StatusPaused, underlying.StatusRetired:
		if held {
			groups = append(groups, groupBars)
			if r.Issuer != "" {
				groups = append(groups, groupDividends)
			}
		}
	}
	return groups
}

// watchEntry builds the jobs.WatchEntry of one row (Decision 3). An empty Issuer stays empty.
func watchEntry(r underlying.Row) jobs.WatchEntry {
	return jobs.WatchEntry{
		Issuer:             r.Issuer,
		Market:             r.Market,
		ReportDateOverride: r.ReportDateOverride,
		BuyPrice:           r.BuyPrice,
		SalePrice:          r.SalePrice,
		Contract: broker.Contract{
			SecType:  "STK", // stock and etf both
			Symbol:   r.Symbol,
			Exchange: r.Exchange,
			Currency: r.Currency,
			ConID:    r.ConID,
		},
	}
}
