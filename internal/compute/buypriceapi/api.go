// Package buypriceapi declares the buy price resolver: the three sources of a resolved price,
// the two marks a stored row carries, the reasons a valuation gave no price, the input and
// result types and the Resolver. Declarations only, decimal text, no float, no clock, no I/O
// (docs/design/buy-price-source.md, Interface; docs/contracts/compute/buyprice.md).
package buypriceapi

import "github.com/andrewrong/optdesk/internal/compute/valuelinesapi"

// Sources of a resolved price, in the order they are tried: valuation, hand, none.
// No other source exists.
const (
	SourceValuation = "valuation"
	SourceHand      = "hand"
	SourceNone      = "none"
)

// The marks a stored row carries for its source. They are written here and nowhere else:
// the writer is the job, the reader is the page.
const (
	MarkValuation = "buy_from_valuation"
	MarkHand      = "buy_from_hand"
)

// Reasons a valuation gave no price (Result.ZoneReason), in the order of Decision 2.
const (
	ReasonNoProfile = "no_profile"
	ReasonAmbiguous = "ambiguous"
	ReasonRemoved   = "removed"
	ReasonNoReview  = "no_review"
	ReasonNoTables  = "no_tables"
	ReasonCurrency  = "currency"
	ReasonBadInput  = "bad_input"
)

// Zone is one valuation profile with its current review, as plain text copied from the store.
type Zone struct {
	Symbol, Currency, Pool, Market, V, P, ReviewDay string
	Reviewed                                        bool
}

// Tables are the R values and the margin rows of the newest versions; Present is false when
// either is missing.
type Tables struct {
	Present bool
	R       valuelinesapi.RValues
	Margin  []valuelinesapi.MarginRow
}

// Input is one company: its key (the issuer) and contract currency, its hand price text
// ("" unset) and the valuation.
type Input struct {
	Symbol, Currency, Hand string
	Zones                  []Zone
	Tables                 Tables
}

// Result is the resolved price. Price is exact decimal text, never rounded, and "" for
// SourceNone. ReviewDay is set for SourceValuation. ZoneReason is "" when the zone was used,
// else a Reason*.
type Result struct {
	Price, Source, ReviewDay, ZoneReason string
}

// Resolver is pure: no I/O, no clock, no state. Resolve never fails; a bad input is a reason.
type Resolver interface {
	Resolve(in Input) Result
}
