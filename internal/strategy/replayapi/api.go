// Package replayapi declares the interface of the strategy replayer and the lot, market and report types it
// reads and writes (docs/design/strategy-replay.md, Interface). Declarations only; it imports contracts and
// strategy, no store, job or page.
package replayapi

import (
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
)

// Replayer walks each lot day by day under each variant. Pure: no I/O, no clock.
type Replayer interface {
	Replay(lots []Lot, mkt Market, variants []Variant, sessionDays []contracts.Day, asOf contracts.Day, minLots int) Report
}

// Variant is one named strategy configuration to replay.
type Variant struct {
	Name     string // "early_50_21"
	Strategy strategy.Strategy
}

// Lot is one short lot. Money is decimal text per share; days YYYY-MM-DD.
type Lot struct {
	ID, Underlying, Right, Strike, Currency, Multiplier string
	InstrumentID                                        string // stored option instrument id text, empty when none matched
	Expiry, EntryDay                                    contracts.Day
	Credit                                              string        // his entry fill price
	Fate                                                string        // held, assigned, assigned_early, closed_early, rolled, open
	SettleDay                                           contracts.Day // expiry, or the assignment day of an assigned_early lot
	ActualGross                                         string        // credit minus buy-back price, or minus intrinsic at assignment; empty while open
	ActualNet                                           string        // realized over quantity and multiplier; empty while open
}

// Market holds the day's ok quote by instrument id and the daily close by underlying name; an absent day is absent.
type Market struct {
	Quotes         map[string]map[contracts.Day]Quote
	Closes         map[string]map[contracts.Day]string
	AdjustedCloses map[string]map[contracts.Day]string // basis adjusted_last, for the split check
}

// Quote is the ok bid and ask of one contract on one day.
type Quote struct{ Bid, Ask string }

// Outcome is one lot under one variant ("hold" is the variant that never closes).
type Outcome struct {
	LotID, Variant string
	Priced         bool
	Reason         string // not priced: no_instrument, gap_before_close, no_settle_price, not_expired, split_in_life
	ClosedOn       contracts.Day
	CloseReason    string // an Action.Reason word or "expiry"
	Gross          string
	InTheMoney     bool
}

// Line is one printed row: one variant (or "actual", or "hold") over one set of lots in one currency.
type Line struct {
	Set, Variant, ParamHash, Currency                             string // Set: "all", "priced:<variant>" or "rolled"
	Lots, Cycles, ClosedEarly, Held, Assigned, Rolled, InTheMoney int
	KeptShare, MeanGross, WorstGross                              string
	HoldKeptShare, ActualKeptShare                                string // the same lots, so a rule is read against hold and against him
	UnderMinimum                                                  bool
}

// Report is what one Replay returns.
type Report struct {
	Lots, LotsWithQuote, StillOpen, LotDays, QuoteDays int
	FirstQuoteDay, LastQuoteDay                        contracts.Day  // the stored quote days the walk could use; empty when none
	Excluded                                           map[string]int // reason -> lots, per variant name prefixed
	Outcomes                                           []Outcome
	Lines                                              []Line
}
