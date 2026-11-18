// Package optionattrapi declares the option attribution: the types and the Attributor that is the
// only place the profit of a short put is split into direction, move cost, IV re-marking and time.
// Declarations only, decimal text everywhere, no float, no clock, no I/O; an empty string is "no value".
package optionattrapi

import (
	"errors"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Pricer has the shape of stress.Pricer in internal/strategy/risk/stress; years is calendar days/365,
// rate is the continuously compounded rate as a fraction, right is "P". An error with
// NoSolution() bool true means no volatility matches the price.
type Pricer interface {
	Value(right, spot, strike, years, rate, vol string) (string, error)
	Implied(right, spot, strike, years, rate, price string) (string, error)
}

// Fill is a ledger row with action open (Side "open", a sale) or close (Side "close", a buy-back).
// The rows expire, assign and exercise are not fills.
type Fill struct {
	Day                         contracts.Day
	Side                        string
	Quantity, Price, Commission string // Commission positive, as the ledger
}

// Ref is the IBKR figures of one stored row, for comparison only; they never enter a sum.
type Ref struct{ IV, Delta, Gamma, Theta, Vega string }

// Quote is one stored row of the contract: Spot is the underlying price of the same row.
type Quote struct {
	Day            contracts.Day
	Bid, Ask, Spot string
	Ref            Ref
}

// Rate is a DGS3MO point in percent.
type Rate struct {
	Day     contracts.Day
	Percent string
}

// Lot is one short put position: the ledger's view. End is open, closed, expired, assigned or exercised.
type Lot struct {
	Currency, Strike, Multiplier string
	Expiry                       contracts.Day
	Fills                        []Fill
	End                          string
	EndDay                       contracts.Day // "" when open
	EndClose                     string        // underlying close of EndDay for expired, assigned, exercised
}

// Input is everything Attribute reads. Quotes ascend by Day. LedgerPnL is the ledger's RealizedPnL
// (UnrealizedPnL for an open lot). MaxRateAgeDays is how old a Rate may be (7).
type Input struct {
	Lot            Lot
	Quotes         []Quote
	Rates          []Rate
	LedgerPnL      string
	LedgerMarkDay  contracts.Day // the ledger's mark day for an open lot; "" when closed
	MaxRateAgeDays int
}

// Step is one pair of neighbouring quote days (or the terminal step). Blocks are seller amounts:
// positive is earned. Reason is set when the step is Unsplit.
type Step struct {
	From, To                                     contracts.Day
	CalendarDays                                 int
	MultiDay, Wide, Terminal                     bool
	Direction, MoveCost, IVRemark, Time, Unsplit string
	IVFrom, IVTo                                 string // solved from the mids; "" when not solved
	RefIVFrom, RefIVTo                           string
	Reason                                       string
}

// Result is one lot in one currency. Commission is negative (a cost); AssignedIntrinsic is positive
// (the intrinsic value moved to the shares). Difference is LedgerPnL minus (Entry + Exit + the sum
// of Steps + Commission + AssignedIntrinsic) rounded half-even to 4 places.
type Result struct {
	Currency                            string
	Steps                               []Step
	Entry, Exit, Commission             string
	Direction, MoveCost, IVRemark, Time string // sums over Steps
	Unsplit, AssignedIntrinsic          string
	LedgerPnL, Difference               string
	UnsplitSteps                        int
	MarkDay                             contracts.Day // last quote day of an open lot; "" when closed
	LateEntry                           bool
	GapDays                             []contracts.Day
	NoQuotes                            bool
	NoRate                              []contracts.Day // days that could not be split for lack of a rate; their amount is in Unsplit
	Reason                              string
}

// Attributor is the only place the split is computed.
type Attributor interface {
	Attribute(in Input) (Result, error)
}

// ErrBadInput: a lot with no open fill, a quantity, price or LedgerPnL that is not a number.
var ErrBadInput = errors.New("optionattrapi: bad input")

// Book is the sum of the lots of one currency. Lots counts the lots added; NoQuoteLots and
// UnreconciledLots are counted apart and add nothing to any sum field; their ledger P&L is in
// NoQuoteLedgerPnL and UnreconciledLedgerPnL. Total is the option side (Entry, Exit, the four step
// blocks, Unsplit and Commission) and excludes AssignedIntrinsic; for the lots added,
// LedgerPnL = Total + AssignedIntrinsic + Difference. RentMinusCost is Time + MoveCost.
type Book struct {
	Currency                                     string
	Lots, NoQuoteLots, UnreconciledLots          int
	Entry, Exit, Commission                      string
	Direction, MoveCost, IVRemark, Time, Unsplit string
	RentMinusCost, Total, AssignedIntrinsic      string
	LedgerPnL, Difference                        string
	// Ledger P&L of the lots counted apart; in no sum field above.
	NoQuoteLedgerPnL, UnreconciledLedgerPnL string
	// AllLots is Lots + NoQuoteLots + UnreconciledLots: the lots given to Combine that have a ledger P&L.
	// LedgerAllPnL is LedgerPnL + NoQuoteLedgerPnL + UnreconciledLedgerPnL, exact, not rounded.
	// Neither includes a lot that was not given to Combine.
	AllLots      int
	LedgerAllPnL string
}

// Booker is the only place results are added. One Book per currency, in order of first appearance.
// The answer for no results is a non-nil empty slice. A result with NoQuotes, or with an empty
// Difference, is counted apart; an empty Exit counts as zero; sums are exact, nothing is rounded.
type Booker interface {
	Combine(rs []Result) []Book
}
