// Package underlying declares the store of the names the owner follows: one insert-only versioned table,
// settings.underlying, the newest row per key winning. It holds declarations only; the implementation is
// internal/store/underlyingpg. Design: docs/design/underlyings.md#Interface.
package underlying

import (
	"context"
	"time"
)

// The lifecycle of a name. The owner sets StatusActive, StatusPaused and StatusRetired; the system sets
// StatusPending (a new name), StatusUnresolved (the broker did not list it) and StatusActive after a resolve.
const (
	StatusPending    = "pending"
	StatusActive     = "active"
	StatusPaused     = "paused"
	StatusRetired    = "retired"
	StatusUnresolved = "unresolved"
)

// What a name is.
const (
	KindStock = "stock"
	KindETF   = "etf"
)

// DSNEnvResolve names the line that holds the DSN of the role ot_w_underlying_resolve, the role of the resolve job.
// The page route and the import use ownerset.DSNEnvSettings, passed by cmd/ot.
const DSNEnvResolve = "OT_PG_DSN_UNDERLYING_RESOLVE"

// Row is one version of one name: every attribute of Decision 3 that the table stores. Text fields are the
// canonical text of the store ("" is unset); BuyPrice and SalePrice are decimal text, never a float.
type Row struct {
	ID                                                     int64
	Key                                                    string // name_key, "US:AAPL"
	Symbol, Market, Kind, Exchange, Currency, Name, Issuer string
	ConID                                                  int64 // 0 until resolved
	Status, Reason                                         string
	Options, Fundamentals                                  bool
	BuyPrice, SalePrice, ReportDateOverride, Note          string
	SetAt                                                  time.Time
}

// Reader reads through the view rd.underlying_r1.
type Reader interface {
	// Current returns the newest row of every key, sorted by Key; an empty slice when none.
	Current(ctx context.Context) ([]Row, error)
	// History returns the newest versions first, at most limit.
	History(ctx context.Context, limit int) ([]Row, error)
}

// Add is a new name, written pending (or active with a ConID, the import).
type Add struct {
	Symbol, Market, Kind, Exchange, Currency, Name, Issuer string
	ConID                                                  int64
	Options, Fundamentals                                  bool
	BuyPrice, SalePrice, ReportDateOverride, Note          string
}

// Edit changes the editable fields of one key; a nil field is unchanged.
type Edit struct {
	Key                                                                                     string
	Status, Name, Issuer, Exchange, Currency, BuyPrice, SalePrice, ReportDateOverride, Note *string
	Options, Fundamentals                                                                   *bool
}

// Change is what one Apply writes.
type Change struct {
	Adds  []Add
	Edits []Edit
}

// Outcome is what the resolve job settles for one pending or unresolved row: ConID 0 means not resolved and Reason
// says why. AfterID is the ID of the row the job read.
type Outcome struct {
	Key     string
	AfterID int64
	ConID   int64
	Reason  string
}

// InvalidError names the field a value was refused for; nothing is written.
type InvalidError struct{ Field, Why string }

// Writer is held by the page route and the import (role ot_w_settings).
type Writer interface {
	// Apply writes the change in one transaction; unchanged fields write nothing. It returns the rows written, or an
	// *InvalidError.
	Apply(ctx context.Context, c Change) (written int, err error)
	Close() error
}

// Settler is held by the resolve job (role ot_w_underlying_resolve): read and Settle only, so it cannot call Apply.
type Settler interface {
	// Pending returns the newest rows that are pending, or unresolved with SetAt before retryBefore.
	Pending(ctx context.Context, retryBefore time.Time) ([]Row, error)
	// Settle inserts the outcome only if the newest row of o.Key still has ID o.AfterID and status pending or
	// unresolved; otherwise it writes nothing and returns written false with no error.
	Settle(ctx context.Context, o Outcome) (written bool, err error)
	Close() error
}

// Store is all three.
type Store interface {
	Reader
	Writer
	Settler
}

// OpenFunc opens a Store from a DSN; underlyingpg.Open is one.
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
