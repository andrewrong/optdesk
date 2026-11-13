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
