// Package spreadlegsapi declares the figure that gives each option spread its net credit and all
// its break-even prices (docs/design/spread-legs.md, Interface). Declarations only; the
// implementation is internal/compute/spreadlegs.
package spreadlegsapi

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// SpreadIn is the structures the matcher gave and the data day.
type SpreadIn struct {
	Structures []compute.Structure
	AsOf       contracts.Day // the data day, injected: the figure reads no clock
}

// SpreadFigures are the figures of one structure.
// NetCredit is the sum over the legs of -Quantity x OpenPrice x Multiplier, decimal text with 2
// places half-even, positive a credit and negative a debit; its Gap is missing_input or
// unsupported_currency. BreakEvens are the prices where the expiry payoff is zero, ascending,
// decimal text with 4 places; BreakEvenGap is "" when BreakEvens is set, else no_rule,
// missing_input or unsupported_currency.
type SpreadFigures struct {
	ID           string
	Expiry       contracts.Day // the shortest expiry of the legs
	DaysToExpiry int           // calendar days from AsOf to Expiry, 0 or more
	NetCredit    compute.FigureValue
	BreakEvens   []string
	BreakEvenGap string
}

// SpreadResult holds one row per input structure, in display order: DaysToExpiry ascending, then
// underlying, then id.
type SpreadResult struct{ Rows []SpreadFigures }

// SpreadFigure is pure: no I/O, no clock, exact decimals.
type SpreadFigure interface {
	Spreads(SpreadIn) SpreadResult
}

// NewSpreadFunc builds a SpreadFigure.
type NewSpreadFunc func() SpreadFigure
