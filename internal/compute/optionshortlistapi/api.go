// Package optionshortlistapi declares the short-put shortlist of the candidates page: from the stored
// short-put rows it decides which are shown, grouped by underlying, with the counts of those dropped.
// Declarations only; decimal text everywhere, no float, no clock, no I/O; an empty string is "no value".
// Design: docs/design/candidates-page-v2.md#Interface.
package optionshortlistapi

// Bounds echoes the constants of the rule.
type Bounds struct {
	OTMMinPct, OTMMaxPct string // "5" and "15"
	DeltaMin, DeltaMax   string // "0.15" and "0.30"
	PerUnderlying        int    // 3
}

// Row is one stored short-put row.
type Row struct {
	Key, Company, Ticker      string
	Strike, Spot              string // the stored texts; the rule computes the distance
	Delta, Bid                string // the stored texts; the rule takes the absolute delta
	Annualized                string // the stored annualized yield (fraction text), the ordering figure
	DTE                       string
	QuoteReason, EventsReason string // first non-pass reason of the stored gates
	Marks                     []string
}

// Input is the rows of one build.
type Input struct{ Rows []Row }

// Drop counts the rows dropped under one code, in the fixed order of Decision 11.
type Drop struct {
	Code string
	N    int
}

// Placed is one shown row; OTM and AbsDelta are decimal text, Liquidity is "" or "wide_spread".
type Placed struct{ Key, OTM, AbsDelta, Liquidity string }

// Group is the shown rows of one underlying; Best is the stored Annualized of its best row.
type Group struct {
	Company, Best string
	Rows          []Placed
}

// Result is the shortlist with its counts.
type Result struct {
	Groups         []Group
	Dropped        []Drop
	Scanned, Shown int
	StaleChain     int
	DTEMin, DTEMax string // smallest and largest days of the scanned rows
	Bounds         Bounds
}

// Shortlister selects the rows to show. An unreadable number drops that row and is never an error: a bid
// or annualized text gives bad_row, a spot or strike gives no_spot, a non-empty delta that cannot be read
// gives no_delta (Decisions 7 to 9); any other error is returned.
type Shortlister interface {
	Select(in Input) (Result, error)
}
