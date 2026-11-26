// Package optiontierapi declares the tier judge of the short-put candidates table: for each stored
// short-put row a tier (fits, check or grey), its place, badges, figures and the cash verdict per
// account alias. Declarations only; decimal text everywhere, no float, no clock, no I/O; an empty
// string is "no value".
package optiontierapi

import "github.com/andrewrong/optdesk/internal/contracts"

// Params are the numbers of the tier rule; DefaultParams of optiontier gives the trial values.
type Params struct {
	DeltaMax       string // "0.3": the bound of |delta| (the owner's setting put_delta_max)
	ThinDelta      string // "0.15": below it a green row carries the note thin_premium
	MaxCashAgeDays int    // 3: the oldest capture, in days before Day, a cash figure may have
	DaysInYear     int    // 365

	// The band of the shown rows (candidates-by-company.md Decision 5); the upper delta bound is DeltaMax.
	BandOTMMinPct string // "5": the least out-of-the-money percent of a shown row
	BandOTMMaxPct string // "15": the most
	BandDeltaMin  string // "0.15": the least |delta| of a shown row
	PerCompany    int    // 3: the most rows shown per company
}

// Position is one stored account position as text; the rule picks the short puts.
type Position struct{ SecType, Right, Strike, Multiplier, Quantity, Currency string }

// AccountCash is one alias's newest capture: the TotalCashValue text, its currency, the capture day, its
// age in days before Day (counted by the caller, which knows how far back it read) and the positions of
// that capture. Cash is "" when the value is not stored.
type AccountCash struct {
	Alias, Cash, Currency string
	CaptureDay            contracts.Day
	AgeDays               int
	Positions             []Position
}

// Row is one stored short-put row. Company is the key rows are counted by (the underlying id as text).
// QuoteReason and EventsReason are the reason codes of the stored quote and events gates ("" when the gate
// passed); Marks are the stored mark words.
type Row struct {
	Key, Company                string
	Currency                    string // the currency of the strike and prices: "" or "USD" is USD; "JPY" is yen
	ChainDay, Expiry            contracts.Day
	DTE                         int
	Spot, Strike, Bid, BuyPrice string
	IV, Delta, Multiplier       string
	QuoteReason, EventsReason   string
	Marks                       []string
}

// Input is everything one build hands to the judge.
type Input struct {
	Day      contracts.Day // the chain day
	Params   Params
	Rows     []Row
	Accounts []AccountCash
}

// CashCell is one alias's verdict for one row: Verdict is fits, short or unknown; Free and Committed are
// money text; Why is the reason when unknown (stale_cash, no_cash, not_usd, other_currency).
type CashCell struct{ Alias, Verdict, Free, Committed, Why string }

// Verdict is the judgement of one row. Tier is fits, check or grey; Grey is the reason when Tier is grey
// (no_buy_price, no_spot, in_the_money, no_bid, bad_input). Place is the 1-based position
// inside the tier. The money fields are decimal text; the percent fields are percents at four decimals;
// an empty text is "no value" and Why names the figure that could not be formed.
type Verdict struct {
	Key, Tier, Grey                   string
	Place                             int
	Badges, Infos, Notes              []string // codes, never nil
	BreakEven, BelowBuy, MarginPct    string
	BelowSpotPct, Moves               string
	PerContract, Intrinsic, TimeValue string
	YieldPct                          string
	Cash                              []CashCell
	FitAlias                          string            // the first alias whose free cash covers the row, "" when none
	Why                               map[string]string // figure id to reason code
}

// CompanyCount is the tier counts of one company, in order of first appearance.
type CompanyCount struct {
	Company           string
	Fits, Check, Grey int
}

// Kept is one condition that kept rows out of Fits: Code is a grey reason or a look-at badge code, N the
// number of rows with it, Every whether N is all rows read.
type Kept struct {
	Code  string
	N     int
	Every bool
}

// Company states of a block, one per company (candidates-by-company.md Decision 3, first match wins).
const (
	StateSellable   = "sellable"     // a fit or check row is inside the band and shown
	StateOutBand    = "out_band"     // some row fits or is to check, none is inside the band
	StateAboveBuy   = "above_buy"    // no longer produced: a row above the buy price keeps its tier with a note; the constant stays declared
	StateNoBuyPrice = "no_buy_price" // all rows grey, one has no buy price
	StateNoQuote    = "no_quote"     // all rows grey, one has no spot or no bid
	StateBlocked    = "blocked"      // any other all-grey company
)

// Band fold codes: the reasons a fit or check row is folded; a grey row is folded under its grey reason.
const (
	FoldNoOpenInterest = "no_open_interest"
	FoldOTMLow         = "otm_low"
	FoldOTMHigh        = "otm_high"
	FoldNoDelta        = "no_delta"
	FoldDeltaOut       = "delta_out"
	FoldOverCap        = "over_cap"
)

// GreyCount is a fold reason and its number of rows.
type GreyCount struct {
	Code string
	N    int
}

// CompanyBlock is one company of the page: its state, its best rows and the rest folded by reason.
// len(Shown) plus the sum of Why[].N is the company's row count; one row has exactly one fold reason.
type CompanyBlock struct {
	Company string      // as CompanyCount.Company
	State   string      // one of the State constants
	Best    string      // YieldPct of Shown[0]; "" when Shown is empty or has no yield
	NFit    int         // shown rows whose tier is fits
	NCheck  int         // shown rows whose tier is check
	NOpen   int         // fit or check rows not shown (outside the band or over the cap)
	Shown   []string    // keys of the in-band fit and check rows, at most PerCompany, by YieldPct descending (empty last), then DTE, strike, key ascending
	Folded  []string    // keys of every other row: grey rows first, then band-folded rows, each by the same rule
	Why     []GreyCount // fold reasons with N > 0: grey codes in the order of precedence, then the band codes in the order above
}

// Result: Verdicts are in input order; Order is the keys in display order (fits, check, then grey by
// reason); Common is the info marks every row carries; CashBuilt says whether any alias figure is known;
// Params echoes the numbers used, for the legend; Blocks are the company blocks in display order
// (companies with shown rows by Best descending, then by state rank), never nil.
type Result struct {
	Verdicts  []Verdict
	Order     []string
	Companies []CompanyCount
	Common    []string
	Kept      []Kept
	CashBuilt bool
	Params    Params
	Blocks    []CompanyBlock
}

// Tierer judges all short-put rows of one build at once. It writes nothing, reads no clock and returns
// optionevidenceapi.ErrBadInput's cause only inside a row (the row becomes grey bad_input); any other
// error is returned.
type Tierer interface {
	Judge(in Input) (Result, error)
}
