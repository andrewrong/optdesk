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

// Need is what one strategy needs of the stored option chains (docs/design/option-capture-need.md). Group names
// the underlyings: a watch.toml group as watch:<group>. Ref is what Low and High multiply, buy_price or spot, and
// the reach is the strike range Low x Ref to High x Ref, both decimal text. Expiry is all or monthly. Every number
// of a declaration is trial unless the manifest row names its source.
type Need struct {
	Strategy       string
	Group          string
	Rights         []string
	MinDTE, MaxDTE int
	Ref            string
	Low, High      string
	Expiry         string
	TradingClass   string
}
