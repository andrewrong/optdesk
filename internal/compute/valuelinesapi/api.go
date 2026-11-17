// Package valuelinesapi declares the valuation lines calculator: the types, the constants and
// the Calculator that is the only place these prices are computed. Declarations only, decimal
// text everywhere, no float, no clock, no I/O; plain strings so that no other package's type
// is needed.
package valuelinesapi

import "errors"

// The R market columns of Input.Market.
const (
	MarketUS   = "US"
	MarketJP   = "JP"
	MarketHKCN = "HKCN"
)

// RStepWarn is the largest move of an R that passes without a warning.
const RStepWarn = "0.05"

// RRow is one bucket of the R table: the bucket ends at YieldUpTo, inclusive; "" is the top
// bucket (no upper bound).
type RRow struct{ YieldUpTo, US, JP, HKCN string }

// MarginRow is one bucket of the safety margin table: the row with the greatest PFrom at or
// below P is used. Margin, fair half width, heavy discount, trim premium, as fractions.
type MarginRow struct{ PFrom, M, W, H, S string }

// RValues is the R of each market column.
type RValues struct{ US, JP, HKCN string }

// Input is what Lines needs.
type Input struct {
	V, P   string
	Market string      // Market*: the column of R to use
	R      RValues     // the newest R version
	Margin []MarginRow // ascending PFrom
	Close  string      // "" when no close price
}

// Flags: every flag compares the close with its own line and with nothing else.
// InBuy is close <= BuyHigh; BuyLow is the second batch price, not a floor.
type Flags struct{ AtHeavy, InBuy, InFair, AtTrim, AtClear, AtCore bool }

// Lines is the result of Lines.
type Lines struct {
	C                                         string // fair price V*R
	R                                         string // the R of the market column that was used
	Heavy, BuyLow, BuyHigh, FairLow, FairHigh string
	Trim, Clear, Core                         string // C*(1+s), V*1.2, V*1.5
	M, W, H, S                                string // the margin row used
	Zone                                      string // "", core, clear, trim, fair, heavy, buy, between
	ZoneNote                                  string // "", no_close, no_zone_market
	Flags                                     *Flags // nil when Zone is empty
}

// Mark is one line or the close on the axis; Pos is a fraction of the axis from 0 to 1, as text.
type Mark struct{ Name, Value, Pos string }

// Band is the price axis of the company page: the positions are fractions of the axis from 0 to 1, as text.
type Band struct {
	Min, Max string
	Marks    []Mark // every line of Lines, and the close when given
}

// Problem is one failed check of a review.
type Problem struct{ Field, Text string }

// CheckInput is the review as typed: every field text, "" when empty.
type CheckInput struct {
	Type, VersionDate, V, P string
	Margin                  []MarginRow
}

// Calculator is the one place that computes these numbers; every page calls it.
type Calculator interface {
	// Lines: V, P and the R of the market column to Lines. Heavy C*(1-h); BuyHigh C*(1-m); BuyLow C*(1-m-0.05);
	// FairLow and FairHigh C*(1-w) and C*(1+w). ErrBadInput when V is not above 0, P is outside 1..5, or Margin has no row for P.
	Lines(in Input) (Lines, error)
	// Band: the axis for Lines and the close; the lines and the close sit inside Min and Max.
	Band(l Lines, close string) Band
	// ChangePercent: (to-from)/from*100 as decimal text; ErrBadInput when from is 0.
	ChangePercent(from, to string) (string, error)
	// ProposeR: the R values of the bucket of the 10-year yield (a yield on an edge takes the lower bucket).
	ProposeR(rows []RRow, yield string) (RValues, error)
	// RStepWarnings: one text per market whose R moves by more than RStepWarn.
	RStepWarnings(previous, next RValues) []string
	// Check: the checks of Decision 12 for a review that is to become effective.
	Check(in CheckInput) []Problem
	// RoundText: round a decimal text half away from zero to places digits; the only rounding of the system.
	RoundText(value string, places int) string
}

// ErrBadInput is returned for an input outside the ranges Calculator names.
var ErrBadInput = errors.New("valuelines: bad input")
