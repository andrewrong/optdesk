// Package blockapi declares the page figures of the household page, the report blocks and the companies blocks,
// and the tech state (NDX against SPX).
// Declarations only; the implementation is internal/compute/pagefig.
package blockapi

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Every figure is a compute.FigureValue (decimal text or a Gap; declared in internal/compute/api.go).

// ValueIn is one summary row of the account (tag NetLiquidation, TotalCashValue, ExcessLiquidity, ...).
type ValueIn struct{ Tag, Value, Currency string }

// DayValue is the net liquidation of one capture day.
type DayValue struct {
	Day            contracts.Day
	NetLiquidation string
	Currency       string
}

// ObligationIn is one open short put of the replayed ledger: strike, multiplier, contracts, premium income.
type ObligationIn struct{ Underlying, Currency, Strike, Multiplier, Contracts, PremiumIncome string }

// MoneyLine is one row of a money block.
type MoneyLine struct {
	Name, Currency, Note string
	Amount, Share        compute.FigureValue
}

// HouseholdInput is one account on one day (Alias is never the account id; UsdJpy is empty without a rate).
type HouseholdInput struct {
	Alias, Whose string
	Day          contracts.Day
	Values       []ValueIn
	MonthStart   []ValueIn
	History      []DayValue
	Obligations  []ObligationIn
	UsdJpy       string
	// Positions are the held stocks and options of the capture; PositionsRead is false when no position source
	// exists, which keeps the Stock value and Option value cells the gap missing_quote. True with no positions gives
	// 0.00. A cell sums quantity x multiplier x mark in the NetLiquidation currency: a JPY position converts with a
	// positive UsdJpy against a USD net and is the gap unsupported_currency without one, any other pair is that gap;
	// an empty Mark makes the cell the gap missing_quote and the Note names the symbols (at most five, then
	// "and <M> more"); a position of another SecType makes both cells missing_input and the Note names it.
	Positions     []PositionIn
	PositionsRead bool
}

// PositionIn is one held stock or option of the capture, all numbers decimal text. SecType is the store's value
// ("stock", "option"; any other value is a gap). Mark is the price per unit, empty when the position has none.
// Multiplier is 1 for a stock. Quantity is signed.
type PositionIn struct {
	Underlying, SecType, Currency, Quantity, Multiplier, Mark string
}

// HouseholdResult holds the lines of the blocks this design builds, in display order.
type HouseholdResult struct {
	Overview, CashAfter, Stress, Obligations, Income, MonthChange, Accounts []MoneyLine
	Trend                                                                   []DayValue
}

// ExecIn is one execution; PriorQuantity is the signed quantity of the contract in the newest earlier capture.
type ExecIn struct{ Time, Symbol, SecType, Right, Strike, Expiry, Side, Quantity, Price, Currency, PriorQuantity string }

// TradeLine is one row of actions.trades; Kind is open, close, expire or exercise.
type TradeLine struct{ Account, Time, Kind, Instrument, Quantity, Price string }

// AccountMoveIn is one account on two capture days.
type AccountMoveIn struct {
	Alias         string
	Values, Prior []ValueIn
	Obligations   []ObligationIn
}

// AccountMoveOut is the row and the two meters of one account.
type AccountMoveOut struct {
	Net, DayAmount, DayPct, MarginBuffer, CashAfterAssignment compute.FigureValue
}

// CloseIn is a series: closes newest first with their days.
type CloseIn struct {
	Name, Symbol string
	Closes       []string
	Days         []contracts.Day
}

// MoveLine is the day and five-day percent change of a series.
type MoveLine struct {
	Name, Symbol string
	D1, D5       compute.FigureValue
	DataDay      contracts.Day
}

// CompanyState is one company row: State is band, owner override, awaiting approval, expired or unvalued.
type CompanyState struct{ Symbol, State, BandToday, BandThen string }

// ChangeLine is one line of the changes block.
type ChangeLine struct{ Symbol, Text string }

// PageFigures evaluates every figure of this design, no I/O, no clock.
type PageFigures interface {
	Household(HouseholdInput) HouseholdResult
	Trades(day contracts.Day, in []ExecIn) []TradeLine
	AccountMove(AccountMoveIn) AccountMoveOut
	Moves(in []CloseIn) []MoveLine
	Glance(rows []CompanyState) []MoneyLine
	Changes(rows []CompanyState) []ChangeLine
}

// NewPageFiguresFunc constructs the implementation (internal/compute/pagefig.New).
type NewPageFiguresFunc func() PageFigures

// HoldingIn is one held position, all numbers decimal text. Cost is the average cost per unit and Mark the price
// of the capture; Mark is empty when the position has no price. Multiplier is 1 for a stock.
type HoldingIn struct {
	Underlying, SecType, Currency, Quantity, Multiplier, Cost, Mark string
	// Prev is the previous close per unit, decimal text, empty when there is none. Only a stock row reads it.
	Prev string
}

// HoldingsIn is the held positions of one account in one currency. NetLiquidation is the NetLiquidation tag of the
// capture values in Currency, the denominator of every share; empty gives the gap no_data for shares.
// UsdJpy is the USD/JPY rate (empty without one): with a positive rate a JPY row converts against a USD Currency.
type HoldingsIn struct {
	Currency, NetLiquidation, UsdJpy string
	Rows                             []HoldingIn
}

// HoldingLine is the value, unrealised profit and share of one position. A row with an empty Mark has the gap
// missing_quote in all three, never a zero. Currency is the currency of Value and Unrealised: USD when a JPY row
// was converted, the row's own otherwise.
type HoldingLine struct {
	Underlying, SecType, Currency string
	Value, Unrealised, Share      compute.FigureValue
	// Day is the day result of a stock row: quantity x multiplier x (Mark - Prev), in Currency. The gap is
	// missing_quote without a Mark and no_data without a readable Prev; an option row always has no_data. Never a zero
	// for a missing figure.
	Day compute.FigureValue
}

// HoldingsResult holds one line per input row, in input order. A total that left a position out carries a gap.
type HoldingsResult struct {
	Rows                        []HoldingLine
	TotalValue, TotalUnrealised compute.FigureValue
}

// GreekIn is the net greeks of the option structures of one underlying and expiry (compute.GreekSet).
// Delta is in dollars per one dollar move of the underlying; Vega is in dollars per volatility point.
type GreekIn struct {
	Underlying  string
	Expiry      contracts.Day
	Delta, Vega string
}

// RiskIn is the input of Risk. Stocks are read for Underlying, SecType, Currency and Quantity only; Mark is never read.
type RiskIn struct {
	Currency string
	Stocks   []HoldingIn
	Greeks   []GreekIn
}

// RiskLine is one line of delta (Key is the underlying) or of vega (Key is the expiry day).
type RiskLine struct {
	Key   string
	Value compute.FigureValue
}

// RiskResult holds delta by underlying and vega by expiry.
// Delta is in dollars per one dollar move of the underlying (a stock counts its share quantity);
// vega is in dollars per volatility point.
type RiskResult struct {
	Delta, Vega           []RiskLine
	DeltaTotal, VegaTotal compute.FigureValue
}

// MoverIn is one contract held on both days, all numbers decimal text. Quantity is q_prev, signed; Multiplier is
// 1 for a stock and empty only when the store has none; a price is empty when that day has no mark; BasisPrev and
// BasisLast are the mark bases of D0 and D1. Right is "put", "call" or "" (a stock).
type MoverIn struct {
	Name, SecType, Right, Strike, Expiry, Currency, Quantity, Multiplier string
	PricePrev, PriceLast, BasisPrev, BasisLast                           string
}

// MoversIn is the input of household.movers. Prev and Last are D0 and D1. NetPrev and NetLast are the
// NetLiquidation of each day, in their own Currency; a Value of "" is missing. UsdJpy is the D1 rate, empty
// without one.
type MoversIn struct {
	Prev, Last       contracts.Day
	UsdJpy           string
	NetPrev, NetLast ValueIn
	Rows             []MoverIn
}

// MoverLine is one line. Amount is USD, 2 places, signed, or a gap; PricePrev and PriceLast are 4 places in
// Currency, or the gap missing_quote. Note is the position in words.
type MoverLine struct {
	Name, SecType, Currency, Note string
	PricePrev, PriceLast, Amount  compute.FigureValue
}

// MoversResult holds the two tables, each by absolute amount (a line without an amount last), at most 60 lines
// each, and the total and Other of Decisions 8 and 15, both in USD.
type MoversResult struct {
	Stocks, Options []MoverLine
	Total, Other    compute.FigureValue
}

// HoldingFigures evaluates the figures of held positions, no I/O, no clock.
type HoldingFigures interface {
	Holdings(HoldingsIn) HoldingsResult
	Risk(RiskIn) RiskResult
	Movers(MoversIn) MoversResult
}

// NewHoldingFiguresFunc constructs the implementation (internal/compute/pagefig.NewHoldings).
type NewHoldingFiguresFunc func() HoldingFigures

// OptionTilesIn is the input of the options card: the replayed positions of every stored alias and the structures
// that Evaluate recognised among their open positions (Structures and Other of every alias). Pure.
type OptionTilesIn struct {
	Day        contracts.Day
	Positions  []compute.LedgerPosition
	Structures []compute.Structure
}

// UsdText is one exact sum as text: Value has two decimals (half away from zero), Dollars is whole dollars
// (half up); both come from the same exact sum. Both are never negative.
type UsdText struct{ Value, Dollars string }

// OptionTilesOut holds the three figures and the counts the captions name. SoldCount is the sold opening lots
// of the month of Day. OpenPremiumGap and MaxLossGap are empty or "no_data" or "unsupported_currency"
// (page-schema reasons); with an empty gap the matching UsdText is the figure. With MaxLossGap "no_data"
// and Uncovered above 0, MaxLossUsd is still the sum of the other structures (empty when none) and the gap text reads "at least" that sum. Uncovered
// counts calls recognised without assigned shares; Unnumbered the USD structures with no number that are not such calls;
// Unreadable the USD structures with a leg whose numbers do not parse; OtherCurrency the structures not
// in USD; OpenCount the open structures of every currency.
type OptionTilesOut struct {
	SoldCount                                                   int
	OpenPremium, MaxLossUsd                                     UsdText
	OpenPremiumGap, MaxLossGap                                  string
	OpenCount, Uncovered, Unnumbered, Unreadable, OtherCurrency int
}

// OptionTiles evaluates the three figures of the home options card, no I/O, no clock.
type OptionTiles interface {
	Tiles(OptionTilesIn) OptionTilesOut
}

// NewOptionTilesFunc constructs the implementation (internal/compute/pagefig.NewOptionTiles).
type NewOptionTilesFunc func() OptionTiles

// GlobalIn is the input of the global assets block. Day is the page day. Proxies, Sectors and Benchmark are closes
// newest first with their days (CloseIn), in list order; Name is the label.
type GlobalIn struct {
	Day       contracts.Day
	Proxies   []CloseIn
	Benchmark CloseIn
	Sectors   []CloseIn
}

// SectorLine is the excess over the benchmark in percentage points over 1, 5 and 20 sessions.
type SectorLine struct {
	Name, Symbol string
	D1, D5, D20  compute.FigureValue
}

// RotationLine is the facts of the rotation sentence: Leaders5 are the names of the first three rows by d5, Laggard
// and LaggardD5 the last row with a d5 and its decimal text, Leader20 the name with the greatest d20.
type RotationLine struct {
	Leaders5           []string
	Laggard, LaggardD5 string
	Leader20           string
}

// GlobalResult: DataDay is the day of SPY's c0 (empty when SPY has no close); Proxies in input order; Sectors sorted
// by d5 descending, gap rows last; Rotation nil when omitted.
type GlobalResult struct {
	DataDay  contracts.Day
	Proxies  []MoveLine
	Sectors  []SectorLine
	Rotation *RotationLine
}

// GlobalFigures evaluates the global assets block, no I/O, no clock.
type GlobalFigures interface {
	Global(GlobalIn) GlobalResult
}

// NewGlobalFiguresFunc constructs the implementation (internal/compute/pagefig.NewGlobal).
type NewGlobalFiguresFunc func() GlobalFigures

// TechParams are the trial constants of the tech state: Band and Hard in percentage points as decimal text,
// Short and Long the session counts of the two windows, Fold the sessions of the two-close fold, MaxAgeDays the
// age of c0 in calendar days beyond which the reading is stale. techstate.TrialParams is their one source.
type TechParams struct {
	Band, Hard        string
	Short, Long, Fold int
	MaxAgeDays        int
}

// TechIn is the input of the tech state. Day is the as-of day. NDX and SPX are closes newest first with their
// days (CloseIn).
type TechIn struct {
	Day      contracts.Day
	NDX, SPX CloseIn
	Params   TechParams
}

// TechResult: State is LEADING, IN LINE, LAGGING or LAGGING HARD, empty when Reason is set; Rank is 1, 0, -1 or -2;
// Raw is the raw state of c0; Reason is empty or a missing reason of compute (no_input, short_history,
// stale_input, bad_input); DataDay is the day of c0 (empty when there is none); RawDays is the number of raw days
// the fold used. The six returns and gaps are decimal text of percent, 2 places, half-even, signed.
type TechResult struct {
	State, Raw                               string
	Rank                                     int
	Reason                                   compute.Reason
	DataDay                                  contracts.Day
	RawDays                                  int
	NDX1m, SPX1m, Gap1m, NDX3m, SPX3m, Gap3m string
}

// TechFunc evaluates the tech state, no I/O, no clock (internal/compute/techstate.Classify).
type TechFunc func(TechIn) TechResult

// FamilyAccountIn is one account of the book for the family figure. Day is the data day of the capture ("" when
// there is none). Reason is "" for an account to count, "no_data" (never captured), "stale_input" (capture too old)
// or "read_error" (the reader failed). Values, Prior and MonthStart are the summary rows of the newest capture, the
// capture before it and the month start.
type FamilyAccountIn struct {
	Alias, Whose, Day         string
	Reason                    string
	Values, Prior, MonthStart []ValueIn
}

// FamilyIn is the input of the family figure: every account of the book and the USDJPY rate as the Fx reader gives
// it (UsdJpy, its day and its source; empty when none was read).
type FamilyIn struct {
	Accounts                    []FamilyAccountIn
	UsdJpy, RateDay, RateSource string
}

// FamilyAccountOut is one account in the result: its Reason (empty when counted), its Day and its net and cash in USD.
type FamilyAccountOut struct {
	Alias, Reason, Day string
	Net, Cash          compute.FigureValue
}

// FamilyResult is the family figure. Currency is "USD". Included lists the aliases in the total in input order,
// Excluded those left out (Reason no_data, stale_input, read_error or unsupported_currency), Rows has one row per
// distinct input alias in input order. CashShare is Cash in percent of Net. OldestDay is the least Day among
// Included, "" when none. FxRate, FxDay and FxSource are set only when a JPY account is counted.
type FamilyResult struct {
	Currency                                                       string
	Included                                                       []string
	Excluded, Rows                                                 []FamilyAccountOut
	Net, Cash, CashShare, DayAmount, DayPct, MonthAmount, MonthPct compute.FigureValue
	OldestDay                                                      string
	FxRate, FxDay, FxSource                                        string
}

// FamilyFigure is the sum of every account of the book. It is a figure of its own, not a method of PageFigures.
type FamilyFigure interface {
	// Family sums the accounts of in. Pure: no I/O, no clock, no float; exact decimal arithmetic.
	//
	// Counting: an account is counted when Reason is empty and its NetLiquidation is USD, or JPY with a positive
	// UsdJpy (the amount is divided by the rate); any other currency, or JPY without a rate, is left out with the
	// reason unsupported_currency. An account with a Reason is left out with that Reason. An account listed twice
	// (same Alias) is counted once, the first; Rows still has one row per distinct alias.
	//
	// Sums: Net and Cash are sums over the counted accounts; DayAmount and MonthAmount are sums of changes (net
	// minus the prior net, net minus the month-start net); DayPct and MonthPct are those amounts over the sum of
	// the earlier nets.
	//
	// Gaps: a figure a counted account cannot give is a gap (missing_input) and never the sum of the others. Cash
	// is a gap when a counted account has no TotalCashValue; DayAmount and MonthAmount when one lacks the earlier
	// net. DayAmount is the gap stale_input when the counted accounts have different Day, because a sum of
	// one-day changes of different days is not a day change. With no account counted every figure is the gap
	// no_data.
	//
	// Reasons: no_data (never captured, or nothing counted), stale_input (capture too old, or mixed days),
	// read_error (the reader failed), unsupported_currency (not USD, or JPY without a rate).
	//
	// CashShare is Cash in percent of Net; a gap when either is a gap, and none given for a negative Cash.
	//
	// FxRate, FxDay and FxSource are set (the rate used, its day and its source) only when a JPY account is
	// counted, and empty otherwise.
	Family(FamilyIn) FamilyResult
}

// NewFamilyFigureFunc constructs the implementation (internal/compute/pagefig.NewFamily, a later issue).
type NewFamilyFigureFunc func() FamilyFigure

// HeldLineIn is one held stock line of one account. Alias is never the account id. Value is in USD, as
// HoldingFigures.Holdings gives it (a gap when the line has no mark). MarkDay is the day of the mark.
type HeldLineIn struct {
	Alias, Underlying, Currency string
	Value                       compute.FigureValue
	MarkDay                     contracts.Day
}

// HeldEarningsIn is what is known of the next earnings date of one company. Rows are the stored earnings rows
// of the window; Override is the owner's date (empty when none); Read is false when no earnings reader exists.
type HeldEarningsIn struct {
	Rows     []HeldEarningsRow
	Override contracts.Day
	Read     bool
}

// HeldEarningsRow is one stored earnings row: Status is listed, absent, unsupported or error; Report is the
// reported date; ObservedOn is the day the row was observed.
type HeldEarningsRow struct {
	Status             string
	Report, ObservedOn contracts.Day
}

// HeldCompanyIn is the company of one (Underlying, Currency). Resolve is ok, no_underlying (no underlying row
// for the symbol) or no_issuer (the row has no issuer); any other value counts as no_underlying.
type HeldCompanyIn struct {
	Underlying, Currency string
	Resolve              string
	Earnings             HeldEarningsIn
}

// HeldIn is the input of HeldFigure. TotalUSD is the family net in USD; Today is the day earnings are counted from.
type HeldIn struct {
	Lines     []HeldLineIn
	Companies []HeldCompanyIn
	TotalUSD  compute.FigureValue
	Today     contracts.Day
}

// HeldRow is one company. Aliases are in input order, without repeats. OldestMark is the oldest MarkDay of its
// lines. Next is the next earnings date, empty when EarnReason is set; DaysTo counts calendar days from Today.
type HeldRow struct {
	Underlying, Currency string
	Aliases              []string
	Value, Share         compute.FigureValue
	OldestMark           contracts.Day
	Next                 contracts.Day
	DaysTo               int
	EarnReason           string
}

// HeldResult holds the rows in display order and the sum of the rows that have a value.
type HeldResult struct {
	Rows    []HeldRow
	RowsSum compute.FigureValue
}

// HeldFigure merges held stock lines into one row per company. It is a figure of its own, not a method of
// PageFigures.
type HeldFigure interface {
	// HeldCompanies is pure: no I/O, no clock, no float; exact decimals.
	//
	// Merge: lines of one (Underlying, Currency) are one row; Value is the sum of its lines, and one line with a
	// gap makes Value the gap missing_quote. A negative line (short stock) counts as its negative value.
	// Share is Value over TotalUSD in percent, a gap when either is a gap. Rows sort by Value descending, gap rows
	// last, ties by Underlying. RowsSum sums the rows with a value (gap no_data when none has one).
	//
	// Earnings (the one copy of the rule): Resolve is checked first (no_underlying, no_issuer), then Read false
	// gives no_reader. Then Override is the date; else the newest row by ObservedOn (a tie keeps the later) is the
	// state: absent or no rows is no_row, unsupported is unsupported, error is row_error, otherwise its Report.
	// A date before Today is past_date. A company with no HeldCompanyIn is no_underlying.
	HeldCompanies(HeldIn) HeldResult
}

// NewHeldFigureFunc constructs the implementation (internal/compute/pagefig.NewHeld).
type NewHeldFigureFunc func() HeldFigure

// FamilyTrendAccountIn is one counted account and its net liquidation of each day it has one.
type FamilyTrendAccountIn struct {
	Alias string
	Days  []DayValue // ascending by day; Currency "USD" or "JPY"; a negative or malformed value is no point
}

// FamilyTrendIn: the counted accounts and the USDJPY rate of each day a yen account has a value (decimal text).
type FamilyTrendIn struct {
	Accounts []FamilyTrendAccountIn
	UsdJpy   map[contracts.Day]string
	Window   []contracts.Day // the session days asked, ascending
}

// FamilyTrendPoint is the net value of one day in USD.
type FamilyTrendPoint struct {
	Day contracts.Day
	Net string // USD, 2 decimal places, half-even
}

// FamilyTrendAccountOut is the points of one account, in USD, on the Window days it has a point.
type FamilyTrendAccountOut struct {
	Alias  string
	Points []FamilyTrendPoint
}

// FamilyTrendResult: Family holds only days on which every account has a point. LeftOut is the number of
// Window days without a family point. Gap is "" or "no_data" (fewer than 2 family points).
type FamilyTrendResult struct {
	Family   []FamilyTrendPoint
	Accounts []FamilyTrendAccountOut // input order
	LeftOut  int
	YenUsed  bool
	Gap      string
}

// FamilyTrendFigure is the daily family net value. It is a figure of its own, not a method of FamilyFigure or
// PageFigures.
type FamilyTrendFigure interface {
	// FamilyTrend sums the accounts of in per day. Pure: no I/O, no clock, no float.
	//
	// An account point is its day's NetLiquidation in USD, 2 places half-even; a JPY value is divided by the
	// UsdJpy rate of its own day. A negative, empty or malformed value, a currency other than USD or JPY, a yen
	// day without a positive rate, or a day outside Window gives no point, never a zero. A family day needs every
	// account: the family point is the sum of the account points of that day, and a day on which one account has
	// no point is not in Family, not filled and not zero. YenUsed is true only when a yen value became a point.
	FamilyTrend(in FamilyTrendIn) FamilyTrendResult
}

// NewFamilyTrendFigureFunc constructs the implementation (internal/compute/pagefig.NewFamilyTrend).
type NewFamilyTrendFigureFunc func() FamilyTrendFigure

// FamilyRiskAccountIn is one counted account: its cash in USD (FamilyAccountOut.Cash), its open short puts and
// whether its ledger was read. Alias is never the account id.
type FamilyRiskAccountIn struct {
	Alias       string
	Cash        compute.FigureValue
	Obligations []ObligationIn
	Read        bool // false: the ledger or the positions of the account could not be read
}

// FamilyRiskIn: the counted accounts, the stock lines of all of them (USD, as HeldLineIn), the family net in USD
// and the USDJPY rate (empty without one).
type FamilyRiskIn struct {
	Accounts []FamilyRiskAccountIn
	Stocks   []HeldLineIn
	Net      compute.FigureValue
	UsdJpy   string
}

// FamilyRiskPut is one company's short puts of the family: the assignment cash in USD, the aliases that hold them.
type FamilyRiskPut struct {
	Underlying string
	Cash       compute.FigureValue
	Aliases    []string
	Premium    string // USD text, "" when unknown
}

// FamilyRiskAccountOut is one account: cash, the cash its puts need, cash after (negative is a shortfall).
type FamilyRiskAccountOut struct {
	Alias                 string
	Cash, Need, CashAfter compute.FigureValue
	Short                 bool // CashAfter is below zero
}

// FamilyRiskCompany is one company of the concentration: stock value, put cash, their sum and its share of Net.
type FamilyRiskCompany struct {
	Underlying                  string
	Stock, Put, Exposure, Share compute.FigureValue
}

// FamilyRiskResult. Puts and Companies are sorted by amount descending, a gap row last, ties by Underlying, at
// most 60 rows each; Hidden counts the rows cut. NeedTotal, CashTotal and AfterTotal are the gap of the first
// missing part, never a partial sum. ShortCount is the number of accounts with Short.
type FamilyRiskResult struct {
	Puts                             []FamilyRiskPut
	Accounts                         []FamilyRiskAccountOut // input order
	Companies                        []FamilyRiskCompany
	NeedTotal, CashTotal, AfterTotal compute.FigureValue
	NeedShare, AfterShare            compute.FigureValue // of Net, percent
	ShortCount, Hidden               int
}

// FamilyRiskFigure is the family's assignment cash, cash after assignment and company exposure. It is a figure
// of its own, not a method of FamilyFigure, HeldFigure or PageFigures.
type FamilyRiskFigure interface {
	// FamilyRisk is pure: no I/O, no clock, no float; exact decimals.
	//
	// Need of an account is the sum of strike x multiplier x contracts of its Obligations in USD; a JPY
	// obligation is divided by UsdJpy; without a positive rate, or for another currency, it is the gap
	// unsupported_currency, an unreadable number the gap missing_input, and every sum it is part of is that gap.
	// CashAfter is Cash minus Need; Short is true when it is below zero. An account with Read false has the gap
	// missing_input in Need and CashAfter, and the three totals are that gap. Puts has one row per underlying
	// (USD cash of all accounts, aliases in input order without repeats, the premium text). Companies has one row
	// per underlying of Stocks and Puts: Exposure is Stock plus Put, Share is Exposure over Net in percent; a
	// stock line with a gap makes Stock and Exposure the gap missing_quote, a Net with a gap makes Share a gap.
	// Both lists sort by amount descending, a gap row last, ties by Underlying, at most 60 rows each.
	FamilyRisk(in FamilyRiskIn) FamilyRiskResult
}

// NewFamilyRiskFigureFunc constructs the implementation (internal/compute/pagefig.NewFamilyRisk).
type NewFamilyRiskFigureFunc func() FamilyRiskFigure
