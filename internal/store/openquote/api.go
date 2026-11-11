// Package openquote declares the stores of the option open pull (docs/design/option-open-pull.md, Interface):
// the rows of obs.option_open, the plan of plan.option_open_item, the marks of master.option_not_listed and the
// contract lookup master.option_contract. It holds declarations only; the implementation is
// internal/store/optionopenpg.
package openquote

import (
	"context"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// DSNEnvOptionOpen names the line that holds the DSN of the role ot_w_option_open, the writer of the three jobs
// option_open_resolve (and its gap pass), option_open_plan and option_open.
const DSNEnvOptionOpen = "OT_PG_DSN_OPTION_OPEN"

// SlotRefresh is the value of the slot of a row of the refresh button. It is set by the overlay reader of `ot
// serve` and never by a writer: a stored row has a slot of four digits ("0945"), and the writer refuses SlotRefresh.
// Pages copy the slot into the source of a quote: four digits are the open pull, SlotRefresh is the refresh.
const SlotRefresh = "refresh"

// OptionOpenWriter appends the rows of obs.option_open.
type OptionOpenWriter interface {
	// InsertOptionOpen writes the rows in one transaction (all rows or none); a row whose key (session day,
	// capture id, slot, instrument id) exists is left as it is. A -1, NaN or infinity in a numeric column is stored
	// as null. A row whose slot is SlotRefresh is a Permanent error and nothing is written. No rows make no
	// database call.
	InsertOptionOpen(ctx context.Context, rows []contracts.OptionOpen) error
}

// OptionOpenReader reads obs.option_open through the view rd.option_open_r1.
type OptionOpenReader interface {
	// OptionOpen returns the rows of the session day, the newest row of each instrument: the greatest
	// received_at, ties by slot and capture id; ordered by instrument id. A day with no rows gives an empty slice.
	// Instants are UTC.
	OptionOpen(ctx context.Context, day contracts.Day) ([]contracts.OptionOpen, error)
}

// OptionOpenStore is the writer and the reader of obs.option_open.
type OptionOpenStore interface {
	OptionOpenWriter
	OptionOpenReader
}

// OptionOpenPlanStore writes and reads plan.option_open_item.
type OptionOpenPlanStore interface {
	// PutPlan writes the items in one transaction; an item whose key (session day, slot, plan id, instrument id)
	// exists is left as it is. No items make no database call.
	PutPlan(ctx context.Context, items []contracts.OptionOpenItem) error
	// Plan returns the items of the session day and slot of the latest plan of each underlying (the plan id with
	// the greatest planned_at among the rows of that underlying), ordered by underlying id and rank. With
	// underlyingIDs it returns those underlyings only; without, all. A day with no plan gives an empty slice.
	Plan(ctx context.Context, day contracts.Day, slot string, underlyingIDs ...int64) ([]contracts.OptionOpenItem, error)
}

// NotListedKey is the identity of a mark of master.option_not_listed.
type NotListedKey struct {
	UnderlyingID int64
	Expiry       contracts.Day
	Strike       string // text, as in the key of the contract
	Right        string // "P" or "C"
	TradingClass string
}

// OptionNotListedStore keeps the contracts IBKR answered none for until their retry_at. The resolve job is its
// only writer.
type OptionNotListedStore interface {
	// AddNotListed writes the marks in one transaction; a mark whose key exists is replaced (a contract asked again
	// after its retry_at and found not listed again gets the new checked_at, retry_at and error). No marks make no
	// database call.
	AddNotListed(ctx context.Context, marks []contracts.OptionNotListed) error
	// NotListed returns the marks not yet due at now (retry_at after now), ordered by key.
	NotListed(ctx context.Context, now time.Time) ([]contracts.OptionNotListed, error)
	// RemoveNotListed removes the mark of key; it reports whether one was removed.
	RemoveNotListed(ctx context.Context, key NotListedKey) (bool, error)
	// RemoveNotListedByUnderlying removes every mark of the underlying and returns how many.
	RemoveNotListedByUnderlying(ctx context.Context, underlyingID int64) (int, error)
	// RemoveNotListedExpired removes the marks whose expiry is before the day and returns how many.
	RemoveNotListedExpired(ctx context.Context, day contracts.Day) (int, error)
}

// OptionRef is one option contract and the instrument id it has in master.instrument. The contract identity is
// every field but InstrumentID. Multiplier is decimal text ("100"); Expiry is a day; Right is "P" or "C".
type OptionRef struct {
	UnderlyingID int64
	Currency     string
	Exchange     string
	TradingClass string
	Multiplier   string
	Expiry       contracts.Day
	Strike       string
	Right        string
	InstrumentID int64
}

// OptionContractStore is the one lookup from a contract to its stored instrument id (master.option_contract).
type OptionContractStore interface {
	// PutOption stores the contract; a contract that exists gets the new InstrumentID and resolved_at.
	PutOption(ctx context.Context, ref OptionRef) error
	// ListOptions returns the contracts of the underlying ordered by expiry, strike (numeric), right and trading
	// class; an underlying with none gives an empty slice.
	ListOptions(ctx context.Context, underlyingID int64) ([]OptionRef, error)
}

// ListRecord is one row of master.option_list: one (underlying, expiry, class) group asked of the broker.
// ListedSession is the day D of the pre-open run that listed the group; a group is listed for a session when
// ListedSession equals it, never by resolved_at. A short or error outcome updates TriedAt and Outcome and keeps
// ListedAt and ListedSession.
type ListRecord struct {
	UnderlyingID  int64
	Expiry        contracts.Day
	Class         string
	ListedAt      time.Time
	ListedSession contracts.Day
	Count         int // contracts stored by the listing
	TriedAt       time.Time
	Outcome       string // "listed", "short" or "error"
}

// ListRunRecord is one row of master.option_list_run: one (session, underlying) of a pre-open run. Session is the
// trading day whose open the list serves, the key day of the run. Expected is the groups to list when the run
// first reached the name, each "expiry|class"; nil (null) until the name's chain answer is known, an empty non-nil
// slice when the answer is known and holds none. Listed is recounted from the
// groups of master.option_list whose ListedSession equals Session. Reason is "not_finished", "none" (a complete
// name) or another reason of the design.
type ListRunRecord struct {
	UnderlyingID int64
	Session      contracts.Day
	Expected     []string
	Listed       int
	Reason       string
	UpdatedAt    time.Time
}

// OptionListStore writes and reads master.option_list and master.option_list_run. It is separate from Stores.
type OptionListStore interface {
	// PutLists writes the rows in one transaction; a row whose key (underlying, expiry, class) exists is replaced.
	// No rows make no database call.
	PutLists(ctx context.Context, rows []ListRecord) error
	// Lists returns the rows of the underlying ordered by expiry and class; none gives an empty slice.
	Lists(ctx context.Context, underlyingID int64) ([]ListRecord, error)
	// PutListRuns writes the rows in one transaction; a row whose key (session, underlying) exists is updated.
	// No rows make no database call.
	PutListRuns(ctx context.Context, rows []ListRunRecord) error
	// ListRuns returns the rows of the session ordered by underlying id; a session with none gives an empty slice.
	ListRuns(ctx context.Context, session contracts.Day) ([]ListRunRecord, error)
}

// Stores is every store of the option open pull on one connection, with Close.
type Stores interface {
	OptionOpenStore
	OptionOpenPlanStore
	OptionNotListedStore
	OptionContractStore
	Close() error
}

// OpenFunc opens the stores from a DSN (internal/store/optionopenpg.Open). The line DSNEnvOptionOpen holds the
// DSN of the writer; `ot serve` opens it with the pages line and calls only the readers.
type OpenFunc func(ctx context.Context, dsn string) (Stores, error)
