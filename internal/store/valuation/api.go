// Package valuation declares the store of the valuation structure: five insert-only tables of schema valuation
// (docs/design/valuation-structure.md, Interface entry 2 and Decision 8). It holds declarations only; the
// implementation is internal/store/valuationpg. Decimal columns are decimal text and dates are contracts.Day.
// The package imports nothing of internal/compute: the table rows are plain text structs declared here.
package valuation

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Profile is one row of valuation.company_profile; the newest row per CompanyKey wins.
type Profile struct {
	ID                                 int64
	CompanyKey, Name, Symbol, Currency string
	RMarket                            string // US, JP, HKCN
	Pool                               string // in_pool, watch, removed
	PoolDate                           contracts.Day
	Note                               string
	SetAt                              time.Time
}

// Adjustment is one adjustment of V: a percent as decimal text and its reason.
type Adjustment struct{ Percent, Reason string }

// Scenario is one scenario of a review; Label is bear, base, bull or tail.
type Scenario struct{ Label, Probability, EPS, Multiple string }

// Review is one row of valuation.review. Rows of one ReviewID are drafts, then at most one effective row, then at
// most one void row; the newest draft of a ReviewID wins.
type Review struct {
	ID                                                     int64
	ReviewID, CompanyKey                                   string
	Type                                                   string // first, post_earnings, event, backfill
	VersionDate                                            contracts.Day
	FiscalPeriodEnd, ReportDate                            contracts.Day // zero when unset
	Status                                                 string        // draft, effective, void
	Source                                                 string        // owner (program is reserved for the plan's step 9)
	V, P                                                   string
	Trigger                                                string // a, b, c, d or ""
	EarningsBase, BaseBasis, ForwardFactor, LockedMultiple string
	MethodResults                                          []string // up to three
	Adjustments                                            []Adjustment
	Scenarios                                              []Scenario
	ChangeMemo, Reaction                                   string
	Moat, Understand, Management, Financial                string
	Reasons                                                map[string]string // per dimension: p, moat, understand, management, financial
	ReportText                                             string
	VoidReason                                             string     // set on a void row
	EnteredAt                                              time.Time  // set by the database
	EffectiveAt                                            *time.Time // set by the database on the effective row
}

// RVersion is one row of valuation.r_version; the newest by EffectiveDate then SetAt wins.
type RVersion struct {
	ID              int64
	EffectiveDate   contracts.Day
	YieldUsed       string
	RUS, RJP, RHKCN string
	AISwitch        bool
	AIExpiry, Note  string
	SetAt           time.Time
}

// RTableRow is one bucket of valuation.r_table; the bucket ends at YieldUpTo, inclusive, "" is the top bucket.
type RTableRow struct{ YieldUpTo, US, JP, HKCN string }

// MarginTableRow is one bucket of valuation.margin_table, as fractions in decimal text.
type MarginTableRow struct{ PFrom, M, W, H, S string }

// RTableVersion is the rows of valuation.r_table that share one EffectiveDate and SetAt.
type RTableVersion struct {
	EffectiveDate contracts.Day
	SetAt         time.Time
	Rows          []RTableRow
}

// MarginTableVersion is the rows of valuation.margin_table that share one EffectiveDate and SetAt.
type MarginTableVersion struct {
	EffectiveDate contracts.Day
	SetAt         time.Time
	Rows          []MarginTableRow
}

// Entry is one company: its newest profile row and its current valuation.
type Entry struct {
	Profile Profile
	Current *Review // nil when the company has no current valuation
}

// State is everything the pages need, read in one call.
type State struct {
	Entries  []Entry             // newest profile row per company_key, ascending key
	RVersion *RVersion           // newest, nil when none
	RTable   *RTableVersion      // newest, nil when none
	Margin   *MarginTableVersion // newest, nil when none
}

// Reader reads the valuation tables.
type Reader interface {
	// Current returns the State. Entry.Current is the effective row with no void row of its ReviewID, whose Type is
	// not backfill, with the latest EffectiveAt (ties by ID). The slices are never nil.
	Current(ctx context.Context) (State, error)
	// Reviews returns every row of a company, newest first; a company with none gives an empty, non-nil slice.
	Reviews(ctx context.Context, companyKey string) ([]Review, error)
	// RVersions returns every R version, newest first, a non-nil slice.
	RVersions(ctx context.Context) ([]RVersion, error)
}

// Writer inserts rows; no method updates or deletes. Every call is one transaction and one insert (a table
// version: all its rows).
type Writer interface {
	PutProfile(ctx context.Context, p Profile) error
	// PutReview inserts a review row. ErrClosed: the ReviewID has an effective row and r is not its void row.
	// ErrNotEffective: r is a void row for a ReviewID with no effective row.
	PutReview(ctx context.Context, r Review) error
	PutRVersion(ctx context.Context, v RVersion) error
	PutRTable(ctx context.Context, t RTableVersion) error
	PutMarginTable(ctx context.Context, t MarginTableVersion) error
}

// ErrClosed is returned by PutReview when the ReviewID is closed by an effective row.
var ErrClosed = errors.New("valuation: review is closed by an effective row")

// ErrNotEffective is returned by PutReview for a void row whose ReviewID has no effective row.
var ErrNotEffective = errors.New("valuation: void row for a review with no effective row")

// OpenFunc opens the Reader and the Writer from a connection string; the closer closes both.
type OpenFunc func(dsn string) (Reader, Writer, io.Closer, error)
