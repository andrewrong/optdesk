package main

import (
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// newValueLines returns the valuation line calculator as the api type, so the Config of option_candidates_page
// gets the interface and not the concrete valuelines.Calc.
func newValueLines() valuelinesapi.Calculator { return valuelines.New() }
