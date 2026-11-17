// Package optionevidenceapi declares the candidate evidence calculator: for each put candidate the
// expected move, the break-even below the buy price, the bid against a price made from realised
// volatility, the skew and the soft labels. Declarations only; decimal text everywhere, no float,
// no clock, no I/O; an empty string is "no value".
package optionevidenceapi

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Valuer is the Value half of stress.Pricer: the price of one unit of the option; years is DTE/365,
// rate and vol are fractions.
type Valuer interface {
	Value(right, spot, strike, years, rate, vol string) (string, error)
}

// Params are the numbers of the rule; the implementation's DefaultParams has the trial values.
type Params struct {
	// DeltaLow and DeltaHigh ("0.15", "0.30": trial) bound the band of the absolute delta.
	DeltaLow, DeltaHigh string
	// DaysInYear is the basis of years and of the 1-sigma (365).
	DaysInYear int
}

// Row is one put candidate with what the page read for it.
type Row struct {
	ChainDay, Expiry contracts.Day
	ExDiv            contracts.Day // "" when none
	DTE              int
	Spot, Strike     string
	Bid              string
	BuyPrice         string // "" when none
	IV, Delta        string
	Multiplier       string
	RatePercent      string // cash rate, "" when none
	Forecast         string // name-level fraction, "" when none
	ForecastErr      string // name-level fraction, "" when none
	ATMIV            string // same expiry, "" when no ATM row
	ATMStrike        string // same expiry, "" when no ATM row
	// EarningsBeforeExpiry is true when an earnings date falls before expiry.
	EarningsBeforeExpiry bool
}

// Evidence is the answer. Each figure has its own Why: empty when the figure has a value, else one of
// no_iv, no_forecast, no_rate, no_fair_price, earnings_before_expiry, no_atm, no_buy_price.
// The edge is a proxy: the bid against a price made from realised volatility.
type Evidence struct {
	BreakEven   string // strike - bid
	BelowBuy    string // buy price - break-even; positive: below the buy price
	BelowBuyPct string
	BelowBuyWhy string

	OneSigma          string
	OneSigmaPct       string
	BreakEvenInSigmas string
	MoveWhy           string

	Fair            string
	EdgePerShare    string // bid - fair
	EdgePerContract string
	EdgeWhy         string

	Skew    string
	SkewWhy string

	ForecastErr string   // repeated beside the forecast
	Labels      []string // never nil; a label changes no gate
}

// Evaluator is the only place these figures are computed. It writes nothing and reads no clock; a
// spot, strike, bid or DTE that is not a usable number is ErrBadInput.
type Evaluator interface {
	Evaluate(r Row) (Evidence, error)
}

// ErrBadInput: a spot, strike, bid or DTE that is not a usable number.
var ErrBadInput = errors.New("optionevidenceapi: bad input")
