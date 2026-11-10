// Package jobs owns the jobs the binary runs.
package jobs

import (
	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
)

// CaptureID identifies one capture: one per key, deterministic,
// uuid.NewSHA1(uuid.NameSpaceOID, []byte(day + "|" + dataset + "|" + subject)).
type CaptureID = uuid.UUID

// WatchEntry is one [[underlying]] of $OT_HOME/watch.toml (docs/design/event-sources.md Decision 1), one
// field per key. Issuer is cik:<10 digits> for a US company and edinet:<code> for a Japanese one; Market is
// US or JP; Contract is the IBKR contract of the company.
type WatchEntry struct {
	Issuer string
	Market string
	// ReportDateOverride is the owner's own report date, YYYY-MM-DD; empty is unset. The owner's word wins over
	// any automatic value: the job writes only an owner row for such an entry and no vendor row.
	ReportDateOverride string
	// BuyPrice is the price the owner would buy at: decimal text in the currency of the contract, positive,
	// at most 4 places; empty is unset. The owner's word wins over any automatic value.
	BuyPrice string
	// SalePrice is the price the owner would sell at: decimal text in the currency of the contract, positive,
	// at most 4 places, not below BuyPrice; empty is unset. The owner's word wins over any automatic value.
	SalePrice string
	Contract  broker.Contract
}

// ImportResult counts the lines read and the rows inserted, the same and the conflicting.
type ImportResult struct {
	Lines, Inserted, Same, Conflict int
}
