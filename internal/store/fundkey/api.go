// Package fundkey declares the optional upsert methods of the stores of the
// earnings, dividend and fact tables. It holds declarations only; the
// implementations are in internal/store/events and internal/store/fundamental.
package fundkey

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// WriteCount is what one upsert call did to the rows it was given: Written rows
// were inserted or replaced (their data row or their answer row), Unchanged
// rows were equal to what is stored.
type WriteCount struct{ Written, Unchanged int }

// EarningsUpserter stores earnings answers by key: a listed row by (issuer,
// period key), every row's answer by (issuer, observed day). A differing row
// replaces the stored one, an equal row changes nothing, an error never
// replaces an answer, a missing optional value never replaces a present one,
// Attempt is ignored. writtenBy names the run. It is an optional method of the
// value store.OpenEarningsDateStoreFunc returns; the job asserts it at Run.
type EarningsUpserter interface {
	UpsertEarnings(ctx context.Context, rows []contracts.EarningsDate, writtenBy string) (WriteCount, error)
	// ReplayEarnings is UpsertEarnings for a run of a past day: the data rows
	// are upserted, an answer row is written only where none is stored for its
	// key.
	ReplayEarnings(ctx context.Context, rows []contracts.EarningsDate, writtenBy string) (WriteCount, error)
}

// DividendUpserter stores dividend answers: a present row by (instrument,
// ex-date), every row's answer by (instrument, session day), under the same
// rules, with the same replay method. It is an optional method of the value
// dividend.OpenFunc returns.
type DividendUpserter interface {
	UpsertDividends(ctx context.Context, rows []contracts.DividendExpectation, writtenBy string) (WriteCount, error)
	// ReplayDividends is UpsertDividends for a run of a past day, as
	// ReplayEarnings is for earnings.
	ReplayDividends(ctx context.Context, rows []contracts.DividendExpectation, writtenBy string) (WriteCount, error)
}

// FactUpserter stores filed facts by their key: a fact read differently
// replaces the stored one. It is an optional method of the value
// store.OpenFactStoreFunc returns.
type FactUpserter interface {
	UpsertFacts(ctx context.Context, rows []contracts.FundamentalFact, writtenBy string) (WriteCount, error)
}
