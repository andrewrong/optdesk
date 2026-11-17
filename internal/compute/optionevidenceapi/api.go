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
	IVP              string // name-level fraction, "" when none
	VolEdge          string // name-level fraction, "" when none
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

	// The percent figures: each is its fraction input (Skew, Row.ATMIV, Row.Forecast, Row.ForecastErr,
	// Row.IVP, Row.VolEdge) times 100 at four decimals, "" when the input is empty or not a number.
	SkewPct        string
	ATMIVPct       string
	ForecastPct    string
	ForecastErrPct string
	IVPPct         string
	VolEdgePct     string
}

// Evaluator is the only place these figures are computed. It writes nothing and reads no clock; a
// spot, strike, bid or DTE that is not a usable number is ErrBadInput.
type Evaluator interface {
	Evaluate(r Row) (Evidence, error)
}

// ChainQuote is one listed option of the chain of one underlying on one day: its expiry, its right
// ("P" or "C", as the terms store it), its strike and its implied volatility as decimal text (empty
// when the row has none), and the Source of the quote ("option_eod" or "option_open").
type ChainQuote struct {
	Expiry contracts.Day
	Right  string
	Strike string
	IV     string
	Source string
}

// ATMPicker chooses the at-the-money row of one expiry. Among the rows of expiry whose Right is "P",
// whose Source is source and whose IV is a number above 0, it takes the strike nearest to spot within
// 10% of spot (a trial value), the lower strike on a tie, and returns that row's IV and strike; with no
// such row, or a spot that is not a number above 0, both are empty. It writes nothing and reads no clock.
type ATMPicker interface {
	ATM(chain []ChainQuote, expiry contracts.Day, spot, source string) (iv, strike string)
}

// ErrBadInput: a spot, strike, bid or DTE that is not a usable number.
var ErrBadInput = errors.New("optionevidenceapi: bad input")
