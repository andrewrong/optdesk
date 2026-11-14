// Package macroevent declares the store of macro events (obs.macro_event): the scheduled releases and meetings the
// macro page lists. It holds declarations only; the implementation is internal/store/macroeventpg. Design:
// docs/design/macro-events.md#Interface.
package macroevent

import (
	"context"
	"io"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Event is one stored row: a kind and a date, nothing else.
type Event struct {
	Kind, Source, SourceRef string
	Date, RunDay            contracts.Day // RunDay: the run day (fred) or the read day (file)
}

// Kind is one row per kind and source, derived from the stored rows.
type Kind struct {
	Kind, Source string
	LastRunDay   contracts.Day // max run_day
	LastDate     contracts.Day // max event_date: the observed horizon
	HeldDay      contracts.Day // max held_day, empty when none
	Rows         int
}

// Outcome is the result of one ReplaceFuture.
type Outcome struct {
	Written, Deleted int
	Held             bool // a delete rule of Decision 5 refused; rows were still upserted
}

// Writer replaces the future rows of a kind.
type Writer interface {
	// ReplaceFuture upserts events by (kind, date) and returns the rows it changed in Written; the same call twice
	// writes 0 the second time. In the same transaction it deletes a stored row of the kind and source only when
	// all of these hold, checked in this order: the answer is not empty; its latest date is not before the latest
	// stored date of the kind and source; the row is dated after runDay; the row is not in the answer. When the
	// first two refuse, Held is true and HeldDay of the stored rows of the kind after runDay is set to runDay; a
	// delete that passes clears HeldDay.
	ReplaceFuture(ctx context.Context, kind, source string, runDay contracts.Day, events []Event) (Outcome, error)
}

// Reader reads the stored events.
type Reader interface {
	// Between returns the events dated from..to inclusive, in date then kind order; none gives an empty slice.
	Between(ctx context.Context, from, to contracts.Day) ([]Event, error)
	// Kinds returns one row per kind and source, in kind then source order; none gives an empty slice.
	Kinds(ctx context.Context) ([]Kind, error)
}

// OpenWriterFunc opens a Writer from a DSN (internal/store/macroeventpg.OpenWriter); the Closer releases it.
type OpenWriterFunc func(ctx context.Context, dsn string) (Writer, io.Closer, error)

// OpenReaderFunc opens a Reader from a DSN (internal/store/macroeventpg.OpenReader); the Closer releases it.
type OpenReaderFunc func(ctx context.Context, dsn string) (Reader, io.Closer, error)
