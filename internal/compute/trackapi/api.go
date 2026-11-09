// Package trackapi declares the option tracking rule of compute: the types and the
// OptionTracker interface. It holds declarations only; the implementation is
// internal/compute/optiontrack. Design: docs/design/option-tracking.md#Interface.
//
// Every number is decimal text or an integer; no float and no clock. An empty string is
// "no value". The position type is compute.LedgerPosition.
package trackapi

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// TrackQuote is the newest option_eod row of one contract. Delta and Iv are empty when
// the row has none.
type TrackQuote struct {
	Underlying      string
	Expiry          contracts.Day
	Strike, Right   string
	Day             contracts.Day
	Bid, Ask        string
	UnderlyingClose string
	Delta, Iv       string
}

// TrackEvent is one stored candidate row. Kind is as stored and Day is its run day. The
// guard fields are set only on a held_call_guard row.
type TrackEvent struct {
	Underlying    string
	Expiry        contracts.Day
	Strike, Right string
	Kind          string
	Day           contracts.Day
	NextEarnings  contracts.Day
	EarningsState string
	NextExDate    contracts.Day
	ExAmount      string
	GuardStatus   string
	GuardReason   string
}

// TrackHeld is an account position that is an option.
type TrackHeld struct {
	Underlying              string
	Expiry                  contracts.Day
	Strike, Right, Quantity string
}

// TrackInput is what one call reads. Positions are the Flex replay of one alias.
// HeldKnown false means there is no account capture: the check is skipped, not empty.
type TrackInput struct {
	Alias       string
	AsOf        contracts.Day
	SessionDays []contracts.Day
	Positions   []compute.LedgerPosition
	Quotes      []TrackQuote
	Events      []TrackEvent
	Held        []TrackHeld
	HeldKnown   bool
}

// TrackParams holds the two limits of the rule: the age of a mark in session days and
// the number of days a closed position stays in Recent.
type TrackParams struct {
	MarkMaxAgeSessions, RecentDays int
}

// TrackedOption is one row of the tracking page; every number is made by the rule
// (Decision 5) and the builder only formats. An empty string is "no value": the builder
// shows a gap cell with the reason of MarkStatus (ok, no_mark), EventStatus (for example
// no_event_row) or Guard. DTE counts calendar days. OtmPercent is signed text with two
// places, half even, positive out of the money. Mid follows the ledger's mark rule.
// Guard is the stored verdict of the held_call_guard row of a short call (Decision 8):
// unknown with GuardReason no_guard_row when there is none, n/a for a put; GuardBasis is
// bid for a stored verdict.
type TrackedOption struct {
	PositionID, Underlying, Right, Strike, Expiry, Quantity, Multiplier, Currency, Status string
	Early                                                                                 bool
	DTE                                                                                   int
	Moneyness, OtmPercent, Spot, Mid                                                      string
	MarkDay                                                                               contracts.Day
	MarkStatus                                                                            string
	Extrinsic, PremiumIncome, Commissions, Kept, KeptPercent, RealizedPnL, Delta, Iv      string
	NextEarnings, NextExDate                                                              contracts.Day
	ExAmount                                                                              string
	EarningsInLife, ExInLife                                                              bool
	EventStatus                                                                           string
	Guard, GuardReason, GuardBasis                                                        string
	OpenedOn, ClosedOn                                                                    contracts.Day
}

// TrackRecon is one row of the check between Flex and the account. Code is not_in_flex
// or not_in_account.
type TrackRecon struct {
	Underlying, Right, Strike, Quantity string
	Expiry                              contracts.Day
	Code                                string
}

// TrackResult is the output. Open holds the open short positions, Recent the positions of
// another status that closed within RecentDays, and LongLegsLeftOut counts the open long
// positions. Order (a rule of the tracker; the builder sorts nothing): Open by expiry,
// then underlying, then strike, then position id, all ascending; Recent by closing day
// descending, then position id; Recon by underlying, expiry, strike.
type TrackResult struct {
	Open, Recent    []TrackedOption
	Recon           []TrackRecon
	LongLegsLeftOut int
}

// OptionTracker is the one rule (Decision 4): pure, exact, no float, no clock, no I/O.
type OptionTracker interface {
	Track(TrackInput, TrackParams) TrackResult
}

// NewOptionTrackerFunc builds an OptionTracker.
type NewOptionTrackerFunc func() OptionTracker
