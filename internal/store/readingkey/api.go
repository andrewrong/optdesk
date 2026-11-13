// Package readingkey declares the optional store method that writes readings by their key. Declarations
// only; the value store.OpenReadingFunc returns implements it.
package readingkey

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// WriteCount is what one UpsertReadings call did: Written readings were inserted or replaced, Skipped ones
// equalled the stored reading or were a missing_input that yields to a stored ok.
type WriteCount struct{ Written, Skipped int }

// ReadingUpserter writes readings by the key (Name, Version, SubjectKind, Subject, Day): a differing reading
// replaces the stored one with its inputs, keys and parameters, an equal one changes nothing, a missing_input
// never replaces an ok, no old value is kept. A batch with one key twice is a Permanent error. It is an optional
// method of the value store.OpenReadingFunc returns; the caller asserts it.
type ReadingUpserter interface {
	UpsertReadings(ctx context.Context, rows []contracts.ReadingRecord) (WriteCount, error)
}
