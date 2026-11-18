// Package basefact declares the read-only view of the base data facts that the base data page reads: one Fact per
// (row id, rule, subject), counted by SQL views and judged elsewhere (docs/design/base-data-page.md, Interface, entry 1).
// It holds declarations only: the implementation is internal/store/basefactpg.
package basefact

import (
	"context"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The names of the four view groups, as they appear in Result.Failed.
const (
	// GroupBars is the view rd.base_fact_bars_r1 (row bars).
	GroupBars = "bars"
	// GroupOption is the view rd.base_fact_option_r1 (rows quotes, chains, open).
	GroupOption = "option"
	// GroupOther is the view rd.base_fact_other_r1 (rows contracts, rates, fx, events, calendar, facts).
	GroupOther = "other"
	// GroupAccount is the view rd.base_fact_account_r1 (rows account, flex).
	GroupAccount = "account"
)

// Fact is one row of a view: the count of rows examined and of rows that break one rule, for one subject.
type Fact struct {
	Dataset string // the row id of the page table
	Rule    string
	Subject string        // empty for a rule of the whole row
	N, Bad  int64         // the rule id rows uses N
	Day     contracts.Day // set for the rule id newest
	Value   string        // set for the rule id newest
}

// Result is what Facts returns. Failed names the view groups that failed or timed out: their facts are absent.
type Result struct {
	Facts  []Fact
	Failed []string
}

// Reader reads the facts as the page role (ot_ro_pages). Each view works out its own newest day and uses the
// New York date as Day, so Facts has no day parameter. Result.Facts and Result.Failed are non-nil. The error is
// returned only when all four views failed.
type Reader interface {
	Facts(ctx context.Context) (Result, error)
	Close() error
}

// OpenFunc opens a Reader on the database named by dsn.
type OpenFunc func(ctx context.Context, dsn string) (Reader, error)
