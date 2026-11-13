// Package optionfigapi declares the two figures of the short-put candidate table: the
// ITM probability and the excess yield over cash. It holds declarations only; the
// implementation is internal/compute/optionfig. Design:
// docs/design/option-candidates-figures.md#Interface.
//
// Every number is text; no float, no I/O and no clock. An empty string is "no value".
// A figure that cannot be formed has a gap and a sentence, never zero.
package optionfigapi

import "github.com/andrewrong/optdesk/internal/contracts"

// PutFigureInput is one short-put row with the cash rate the page read for its group.
// Spot, Strike, IV and AnnualYield are decimal text as stored (a missing one is the
// empty string); Rate is percent text, empty when there is none.
type PutFigureInput struct {
	// ChainDay is the session day of the chain.
	ChainDay contracts.Day
	// DTE is the days to expiry.
	DTE int
	// Spot, Strike and IV are decimal text as stored.
	Spot, Strike, IV string
	// AnnualYield is the stored annual yield, decimal text.
	AnnualYield string
	// Rate is the cash rate in percent, empty when none.
	Rate string
	// RateDay is the day of the rate point, empty when none.
	RateDay contracts.Day
	// MaxRateAgeDays is the oldest a rate may be, in days before ChainDay.
	MaxRateAgeDays int
}

// PutFigures is the two figures of one row. When a gap is set the value fields of that
// figure are empty; the gap sentence is English, the words above and below and the
// column sentences are the page's.
type PutFigures struct {
	// ItmPercent is the ITM probability (27.5%).
	ItmPercent string
	// ItmKey is the sort key of the ITM probability (2755).
	ItmKey string
	// ItmGap is empty, or missing_input or stale_input.
	ItmGap string
	// ItmWhy is the gap sentence.
	ItmWhy string
	// ExcessPercent is the excess yield over cash, unsigned (4.5%).
	ExcessPercent string
	// ExcessAbove is true when the yield is above cash.
	ExcessAbove bool
	// ExcessKey is the sort key of the excess, signed integer text.
	ExcessKey string
	// ExcessGap is empty, or missing_input or stale_input.
	ExcessGap string
	// ExcessWhy is the gap sentence.
	ExcessWhy string
}

// OptionFigures forms the two figures of a short-put row. Pure: no I/O, no clock.
type OptionFigures interface {
	// Put gives the figures of one row; a figure with no usable input is a gap.
	Put(in PutFigureInput) PutFigures
}

// NewOptionFiguresFunc constructs the implementation (internal/compute/optionfig).
type NewOptionFiguresFunc func() OptionFigures
