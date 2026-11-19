// Package candidatecur declares the per-currency read and write of the candidate rows
// (docs/design/candidates-jp.md Decisions 19 to 21). It holds declarations only; the implementation is
// internal/store/candidate. A row whose currency is null reads as USD.
package candidatecur

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Row is a candidate row and the currency its amounts are in.
type Row struct {
	store.Candidate
	Currency string // three upper-case letters, such as USD or JPY
}

// Reader reads the candidate rows of one currency.
type Reader interface {
	// CandidatesIn returns the rows of one kind of one day written by the newest run of that currency (the latest
	// ComputedAt among the rows of the currency), ranked rows first, so the rank sequences of two currencies never
	// mix in one result; none gives an empty slice, never nil. Currency USD includes the rows with a null currency.
	// A zero day, an unknown kind or a currency that is not three upper-case letters is an error.
	CandidatesIn(ctx context.Context, day contracts.Day, kind, currency string) ([]Row, error)
}

// Writer writes candidate rows with their currency.
type Writer interface {
	// InsertCandidatesIn inserts the rows in one transaction with the semantics of
	// store.CandidateWriter.InsertCandidates (none is replaced, the result counts the rows really inserted), and
	// stores each Currency. A row whose Currency is not three upper-case letters fails the call before anything
	// is written.
	InsertCandidatesIn(ctx context.Context, rows []Row) (int, error)
}
