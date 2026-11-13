// Package blockapi declares the page figures of the household page, the report blocks and the companies blocks.
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
type HoldingIn struct{ Underlying, SecType, Currency, Quantity, Multiplier, Cost, Mark string }

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

// HoldingFigures evaluates the figures of held positions, no I/O, no clock.
type HoldingFigures interface {
	Holdings(HoldingsIn) HoldingsResult
	Risk(RiskIn) RiskResult
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
