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
type HoldingsIn struct {
	Currency, NetLiquidation string
	Rows                     []HoldingIn
}

// HoldingLine is the value, unrealised profit and share of one position. A row with an empty Mark has the gap
// missing_quote in all three, never a zero.
type HoldingLine struct {
	Underlying, SecType      string
	Value, Unrealised, Share compute.FigureValue
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
