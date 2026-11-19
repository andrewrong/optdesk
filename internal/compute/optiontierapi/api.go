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
// (no_buy_price, no_spot, in_the_money, no_bid, above_buy_price, bad_input). Place is the 1-based position
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

// Result: Verdicts are in input order; Order is the keys in display order (fits, check, then grey by
// reason); Common is the info marks every row carries; CashBuilt says whether any alias figure is known;
// Params echoes the numbers used, for the legend.
type Result struct {
	Verdicts  []Verdict
	Order     []string
	Companies []CompanyCount
	Common    []string
	Kept      []Kept
	CashBuilt bool
	Params    Params
}

// Tierer judges all short-put rows of one build at once. It writes nothing, reads no clock and returns
// optionevidenceapi.ErrBadInput's cause only inside a row (the row becomes grey bad_input); any other
// error is returned.
type Tierer interface {
	Judge(in Input) (Result, error)
}
