// Package pepbapi declares the PE and PB reading types and the function type that forms
// them. Declarations only, decimal text, no clock, no I/O.
package pepbapi

import "github.com/andrewrong/optdesk/internal/contracts"

// MinDays is the fewest stored days with a ratio under which no percentile is given.
const MinDays = 250

// StalePriceDays is how old the newest bar may be before no figure is given.
const StalePriceDays = 5

// The states of a Reading.
const (
	StateOK      = "ok"
	StateMissing = "missing_input" // Reason says which input
	StateLoss    = "loss"          // TTM net income at or below zero (PE) or equity at or below zero (PB)
)

// Reason codes, stable strings: no_price, price_stale, no_shares, no_income, no_annual,
// ttm_piece_missing, loss, no_equity, negative_equity, non_usd, share_count_jump,
// share_count_mismatch; of a percentile and the price points: too_few_days.

// Point is one stored day of the chart: Text one decimal, Value the same number for the chart.
type Point struct {
	Day   contracts.Day
	Text  string
	Value float64
}

// AtPercentile is the ratio at a rank of the window and the price that puts today's ratio there.
type AtPercentile struct {
	Percentile int    // 20 or 50
	Ratio      string // one decimal
	Price      string // two decimals
	Text       string // names the window, ends "(history, not a buy price)"
}

// Reading is one ratio of one company on the newest stored day.
type Reading struct {
	Kind, State, Reason string        // Kind is "pe" or "pb"
	Day                 contracts.Day // the bar day the ratio is for
	Ratio               string        // one decimal; empty unless StateOK
	BasisEnd            contracts.Day // period end of the newest piece of the ratio's basis
	BasisFiled          contracts.Day // filed day of that piece
	Shares              string        // the share count used, whole shares
	SharesFiled         contracts.Day // its filed day
	Percentile          string        // integer text, empty when PercentileReason is set
	PercentileReason    string
	At                  []AtPercentile // empty when PercentileReason is set
	WindowDays          int            // stored days that had a ratio
	WindowFrom          contracts.Day  // first of them
	History             []Point        // at most 120, oldest first
	Text                string         // the sentence of the card, or the reason in words
}

// Input holds what is stored: every fact of the company filed on or before Today and its
// `trades` bars, oldest first.
type Input struct {
	Facts []contracts.FundamentalFact
	Bars  []contracts.PriceBar
	Today contracts.Day
}

// Result is the PE and the PB reading.
type Result struct{ PE, PB Reading }

// BuildFunc forms both readings. Implementation: internal/compute/pepb (New takes a compute.Stats).
type BuildFunc func(in Input) Result
