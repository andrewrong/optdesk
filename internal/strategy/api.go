package strategy

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Strategy is one option strategy as a unit (docs/design/strategy-framework.md). Every method is pure: no I/O,
// no clock, exact decimal text arithmetic.
type Strategy interface {
	ID() string      // "short_put": the strategy column of a result row and the strategy of its [[need]] row
	Version() string // the rule version, "put_v1": changes whenever a rule changes
	Params() Params
	// Evaluate gives one Evaluation per candidate structure of the snapshot, with RankInUnderlying set and Rank 0; Rank sets Rank.
	Evaluate(Snapshot) []Evaluation
	// Rank sets Rank over the rows of the run and returns them in the strategy's order.
	Rank([]Evaluation) []Evaluation
	// Manage says what to do with one open position of this strategy on the day of the snapshot.
	Manage(Position, Snapshot) Action
}

// Params are the named parameters of a strategy and the hash of them.
type Params struct {
	Values map[string]string // decimal or text; every number is trial unless its manifest comment names a source
	Hash   string            // SHA-1 hex of the sorted key=value lines
}

// Snapshot is everything one Evaluate or Manage reads. Days are YYYY-MM-DD, money and rates decimal text.
type Snapshot struct {
	RunDay, ChainDay    contracts.Day
	SessionDays         []contracts.Day
	Issuer              string
	Underlying          string
	UnderlyingID        int64
	Spot                string
	Quotes              []compute.OptionQuote
	BuyPrice, SalePrice string // the owner's prices, empty when unset
	Earnings            *compute.EarningsInfo
	Dividend            *compute.DividendInfo
	Rate                string
	RateDay             contracts.Day
	Accounts            []compute.AliasFunds
	Context             map[string]string // named facts the caller knows; a strategy copies them into Facts
}

// Evaluation is one candidate structure with its checks. Key is unique within (day, strategy): the instrument
// ids of the legs joined by ":" and the alias when the row is per alias.
type Evaluation struct {
	Key              string
	Legs             []Leg
	Checks           []Check
	Status           string // ok, blocked or unknown
	Facts            map[string]string
	Rank             int // 0 when not ok
	RankInUnderlying int
}

// Leg is one option leg of an Evaluation.
type Leg struct {
	InstrumentID string
	Side         string // short or long
	Right        string // P or C
	Strike       string
	Expiry       contracts.Day
	Multiplier   string
	Quantity     int
}

// Check is the outcome of one gate.
type Check struct {
	Name   string // the gate word
	Status string // pass, block or unknown
	Reason string // the first reason code when not pass
}

// Position is one open lot of a strategy: what Manage needs; LedgerPosition fills it.
type Position struct {
	Contract   string
	Right      string
	Strike     string
	Expiry     contracts.Day
	OpenedOn   contracts.Day
	OpenPrice  string // credit per share
	Quantity   string // negative when short
	Multiplier string
}

// Action is the answer of Manage: Kind is hold or close; Reason is empty for hold and a word for close.
type Action struct{ Kind, Reason string }

// Book risk (docs/design/strategy-risk.md, Interface). All numbers are decimal text.

// RiskClass is the product class of an underlying: "equity", "broad_high" or "broad_other".
type RiskClass string

// RiskLeg is the option part of a holding; CashSettled is true when the leg is European and cash settled.
type RiskLeg struct {
	Right, Strike, Expiry, QuoteIV string
	CashSettled                    bool
}

// RiskHolding is one open line: a share line (Option nil) or one option leg.
type RiskHolding struct {
	Underlying, Currency string
	Class                RiskClass // "" when the symbol has no class
	Option               *RiskLeg
	Quantity, Multiplier string // quantity negative when short
	Spot, Mark           string // Mark "" when unknown
	MarkDay, SpotDay     contracts.Day
	MarkUsable           bool // false for a status or basis that is not a usable mark
}

// RiskFunds are the funds of one alias in one currency.
type RiskFunds struct{ Currency, NetLiquidation, Available, ExcessLiquidity string }

// RiskBook is one alias on one run day; Funds is nil with no account.value of the day or the day before.
type RiskBook struct {
	Alias, Currency string
	RunDay          contracts.Day
	Rate            string
	Holdings        []RiskHolding
	Funds           *RiskFunds
	HasCapture      bool                    // false for an alias with no capture: its STOCK rows are unknown unless the ledger carries them
	SessionsOld     func(contracts.Day) int // sessions between a day and RunDay, supplied by the caller
}

// RiskPoint is one point of the ladder: the move, the profit and loss and the assignment cash at it.
type RiskPoint struct{ Move, PnL, AssignmentCash string }

// RiskClassResult is the ladder of one class in one currency.
type RiskClassResult struct {
	Class         RiskClass
	Currency      string
	Ladder        []RiskPoint // eleven points on ten equal intervals
	Worst         RiskPoint
	OldestSpotDay contracts.Day
}

// RiskExcluded is a holding not stressed; Reason is no_class, no_vol, stale_mark, expired, not_in_ledger or no_capture.
type RiskExcluded struct{ Underlying, Currency, Reason string }

// RiskTotal is the sum of the class worsts in one currency, a loss as a positive number; it exists only when
// that currency is complete (nothing of it excluded).
type RiskTotal struct{ Currency, WorstLoss string }

// RiskOutcome is the stress of one book. Classes are per class and currency over the holdings stressed; Totals
// only for complete currencies; NotStressed counts holdings not stressed by currency.
type RiskOutcome struct {
	Alias       string
	Classes     []RiskClassResult
	Totals      []RiskTotal
	Excluded    []RiskExcluded
	NotStressed map[string]int
}

// RiskStresser revalues a book over the FINRA ladder, purely.
type RiskStresser interface{ Stress(RiskBook) RiskOutcome }

// RiskCandidate is the structure of one ok candidate row; Spot and QuoteIV come from its facts.
type RiskCandidate struct {
	Key   string
	Class RiskClass
	Legs  []RiskHolding
}

// RiskFit is the verdict of one candidate in one book: the check and the facts.
type RiskFit struct {
	Check Check // Name "fit_capital"
	Facts map[string]string
}

// RiskFitter is the capital fit, purely.
type RiskFitter interface {
	Fit(RiskBook, RiskOutcome, RiskCandidate) RiskFit
}
