// Package ownerset declares the store of the owner's settings: parameter values
// and option exclusions, both append-only, read as of a time. It holds
// declarations only; the implementation is internal/store/ownersetpg.
package ownerset

import (
	"context"
	"time"
)

// DSNEnvSettings names the line that holds the DSN of the role that may write
// the settings. A page process reads with the DSN of ot_ro_pages, whose Apply
// fails with a Permanent error from the database (no grant).
const DSNEnvSettings = "OT_PG_DSN_SETTINGS"

// Param is the value of one parameter in force, with the time it was set.
type Param struct {
	Key, Value string
	SetAt      time.Time
}

// Exclusion is the state of one instrument in force, with the time it was set.
type Exclusion struct {
	InstrumentID int64
	Excluded     bool
	SetAt        time.Time
}

// Change is what one Apply writes.
type Change struct {
	Params     []ParamValue
	Exclusions []ExclusionValue
}

// ParamValue sets one parameter.
type ParamValue struct{ Key, Value string }

// ExclusionValue sets one instrument excluded or not.
type ExclusionValue struct {
	InstrumentID int64
	Excluded     bool
}

// HistoryRow is one row of either table. Kind is "param" (Key and Value set) or
// "exclusion" (InstrumentID and Excluded set).
type HistoryRow struct {
	Kind, Key    string
	InstrumentID int64
	Value        string
	Excluded     bool
	SetAt        time.Time
}

// Reader reads the settings.
type Reader interface {
	// Params returns the newest row per key not after at, sorted by key.
	Params(ctx context.Context, at time.Time) ([]Param, error)
	// Exclusions returns the newest row per instrument not after at, sorted by id.
	Exclusions(ctx context.Context, at time.Time) ([]Exclusion, error)
	// History returns the newest n rows of both tables, newest first.
	History(ctx context.Context, n int) ([]HistoryRow, error)
	// Version returns the newest row id of both tables not after at, 0 when none.
	Version(ctx context.Context, at time.Time) (int64, error)
	// Close releases the connection.
	Close() error
}

// Writer writes the settings.
type Writer interface {
	// Apply inserts the changed fields in one transaction and returns the number
	// of rows inserted; a field equal to the value in force inserts nothing, so
	// applying a change twice inserts rows only the first time.
	Apply(ctx context.Context, c Change) (int, error)
}

// Store reads and writes the settings.
type Store interface {
	Reader
	Writer
}

// OpenFunc opens a Store on a DSN; internal/store/ownersetpg.Open is its value.
type OpenFunc func(ctx context.Context, dsn string) (Store, error)
