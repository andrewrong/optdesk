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
