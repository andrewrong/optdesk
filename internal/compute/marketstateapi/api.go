// Package marketstateapi declares the figures of the market state page that internal/compute
// computes (the S&P 500 drawdown, the sector gaps and their ranks, the rotation map and the trend
// history of the five market rows) and the function that computes them. Declarations only: no
// function body. The one implementation is internal/compute/marketstate.Compute. Design:
// docs/design/market-state.md#Interface.
package marketstateapi

import (
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Params are the trial constants. marketstate.TrialParams is their one source.
type Params struct {
	HighSessions  int // 60: the window of the drawdown high
	ShortSessions int // 21: the short sector window
	LongSessions  int // 63: the long sector window
	MeanSessions  int // 200: the moving average of sp500_vs_200d history
	StaleWeekdays int // 3: SPY's newest close older than this many weekdays is stale (the rule of report.sector_excess)
	MaxAgeDays    int // 4: a newest series point older than this many calendar days is stale
	ChartYears    int // 3: the history window in calendar years
	ChartMax      int // 800: at most this many history points per row
	TailStep      int // 5: sessions between tail points
	TailPoints    int // 4: tail points before today's
}

// Input holds what the figures need. Day is the as-of day. Each series is ascending by day, nil when none is
// stored. Benchmark, Funds and Industry are newest-first closes as decimal text (blockapi.CloseIn).
type Input struct {
	Day                                 contracts.Day
	SP500, VIXCLS, VXVCLS, BAMLH0A0HYM2 compute.Series
	Benchmark                           blockapi.CloseIn   // SPY
	Funds, Industry                     []blockapi.CloseIn // the 11 sector funds; SMH
	Figures                             compute.Figures    // the one the registered figures use (cmd/ot/wire.go)
	Units                               compute.Units
	Rules                               compute.Rules
	Params                              Params
}

// Position is the rank of a fund at one window: At 1 is the greatest gap, Of the number of funds ranked at
// that window. Reason is set (and At, Of are 0) when the fund is not ranked.
type Position struct {
	At, Of int
	Reason compute.Reason
}

// Fund is one fund. Gap21 and Gap63 are fund percent change minus SPY percent change, points, 2 places half-even;
// Day is EndDay and Aux the window's start day (SPY's day at the window's depth). A missing figure has an empty
// Value and a Reason (never 0). Rank21 and Rank63 are zero for an industry fund.
type Fund struct {
	Symbol, Name   string
	Gap21, Gap63   liquidityapi.Num
	Rank21, Rank63 Position
	Rotation       []Spot // today's point last, oldest first, tail points that fail the rule dropped; empty when today's has no figure
}

// Spot is one position of the rotation map as of Day: X is the 63-session gap, Y is gap21 - gap63/3 (points, 2 places
// half-even, from the unrounded gaps).
type Spot struct {
	Day  contracts.Day
	X, Y string
}

// Point is one day of a history; Value is decimal text.
type Point struct {
	Day   contracts.Day
	Value string
}

// Line is the daily points of one row over the chart window, ascending, at most ChartMax; Total is the number
// of points before the cap (the page writes "newest 800 of N" from it). Sp500Vs200d and VIXTerm hold the
// register's rounded text (a fraction, 6 places); a nil Figures leaves them empty.
type Line struct {
	Points []Point
	Total  int
}

// History is the line of each market row.
type History struct {
	Sp500Vs200d, Drawdown, VIX, VIXTerm, HighYield Line
}

// Result holds every figure of the page that this package computes.
type Result struct {
	EndDay   contracts.Day    // SPY's newest close day; empty when SPY has no close
	Drawdown liquidityapi.Num // percent, 2 places, 0 or below; Aux is the day of the high
	Funds    []Fund           // the sector funds sorted by Gap63, greatest first, gap rows last, ties in input order
	Industry []Fund           // in input order, never ranked
	History  History
}

// Func computes the Result; no I/O, no clock (internal/compute/marketstate.Compute).
type Func func(Input) Result
