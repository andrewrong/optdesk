package main

import (
	"github.com/andrewrong/optdesk/internal/compute/buyprice"
	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// newValueLines returns the valuation line calculator as the api type, so the Config of option_candidates_page
// gets the interface and not the concrete valuelines.Calc.
func newValueLines() valuelinesapi.Calculator { return valuelines.New() }

// newBuyPriceResolver returns the one buy price resolver, buyprice.New over the valuation line calculator, as the
// api type. The candidates page route and the jobs use this one resolver (buy-price-source.md Decisions 9 and 11).
func newBuyPriceResolver() buypriceapi.Resolver {
	// The calculator goes through a variable of the api type: go-arch-lint's deep scan would otherwise read
	// valuelines.New() inside buyprice.New as an edge from compute/valuelines to compute/buyprice.
	var calc valuelinesapi.Calculator = valuelines.New()
	return buyprice.New(calc)
}
