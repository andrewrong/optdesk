// Package liquidityapi declares the figures of the funding and credit block of the macro page
// and the function that computes them. Declarations only: no function body. The one
// implementation is internal/compute/liquidity.Compute. Design:
// docs/design/liquidity-view.md#Interface.
package liquidityapi

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Params are the trial constants of the figures. liquidity.TrialParams is their one source.
type Params struct {
	DailyMaxAgeDays   int // 4: a daily level older than this many calendar days is stale
	WeeklyMaxAgeDays  int // 14: the same for a weekly level
	PercentileMinDays int // 250: fewer stored days is short_history
}

// Input is everything the figures need. Day is the as-of day. Each series is ascending by day
// (compute.Series), nil when none is stored; values are in the unit the store holds (percent
// as percent, money in whole USD).
type Input struct {
	Day                                                         contracts.Day
	WRESBAL, RRPONTSYD, WALCL, WDTGAL, TLAACBW027SBOG, WLCFLPCL compute.Series
	SOFR, IORB, SOFR99                                          compute.Series
	BAMLH0A0HYM2, BAMLC0A0CM, BAA10Y                            compute.Series
	NFCI, STLFSI4                                               compute.Series
	Net                                                         NetFunc // ease.NetLiquidity
	Params                                                      Params
}

// NetFunc is the net liquidity level on the newest day it can be had at or before asOf, in
// whole USD; internal/compute/ease.NetLiquidity is its one implementation (the Ease's own).
type NetFunc func(walcl, wdtgal, rrp compute.Series, asOf contracts.Day) Num

// Num is one figure: Value is decimal text (empty when Reason is set), Day its own data day.
// Aux is a second data day, the RRP day of net liquidity; empty otherwise. Reason is empty or
// one of compute's no_input, stale_input, short_history, zero_range, bad_input, or
// missing_input.
type Num struct {
	Value  string
	Day    contracts.Day
	Aux    contracts.Day
	Reason compute.Reason
}

// MissingInput is the reason of a figure whose second input has no point; compute.Reason has
// no such value and internal/compute/api.go cannot grow.
const MissingInput compute.Reason = "missing_input"

// Change is a difference over Weeks weeks; Day is the newest day and From the day it is taken
// against.
type Change struct {
	Num
	Weeks int
	From  contracts.Day
}

// Rank is the percentile of the newest value in the stored history: Value 0 to 100 as text,
// First and Day the first and last stored day of the window, Days the number of days in it.
type Rank struct {
	Num
	First contracts.Day
	Days  int
}

// Result holds every figure of the block and its taps. SofrIorbBP is in basis points, one
// decimal; ReservesOverAssets in percent, two decimals; NetLiquidity in whole USD. A missing
// figure has an empty Value and a Reason, never 0.
type Result struct {
	Reserves, RRP                                    Num
	Reserves4w, Reserves13w                          Change
	SOFR, IORB, SOFR99, SofrIorbBP                   Num
	HighYield, InvestmentGrade, Baa10Y               Num
	HighYieldRank                                    Rank
	NFCI, STLFSI4                                    Num
	ReservesOverAssets, DiscountWindow, NetLiquidity Num
}

// Func computes the Result; no I/O, no clock (internal/compute/liquidity.Compute).
type Func func(Input) Result
