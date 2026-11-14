package marketstate

import (
	"math/big"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// rawGap is the exact (unrounded) change of the fund minus the change of the benchmark over back sessions, in
// percentage points, by the c0 rule of report.sector_excess: the benchmark needs a close and must not be stale
// against day; the fund needs a close on the benchmark's newest day and on the benchmark's day back sessions
// earlier. The reason is empty when the gap is a number.
func rawGap(spy, s blockapi.CloseIn, day contracts.Day, back, staleWeekdays int) (*big.Rat, compute.Reason) {
	if len(spy.Closes) == 0 || len(spy.Days) != len(spy.Closes) {
		return nil, compute.NoInput
	}
	if weekdaysBehind(spy.Days[0], day, staleWeekdays) > staleWeekdays {
		return nil, compute.StaleInput
	}
	if len(s.Closes) == 0 || len(s.Days) != len(s.Closes) {
		return nil, compute.NoInput
	}
	if s.Days[0] != spy.Days[0] {
		return nil, liquidityapi.MissingInput
	}
	if len(s.Closes) <= back || len(spy.Closes) <= back {
		return nil, compute.NoInput
	}
	if s.Days[back] != spy.Days[back] {
		return nil, liquidityapi.MissingInput
	}
	e0, ok1 := parseRat(s.Closes[0])
	en, ok2 := parseRat(s.Closes[back])
	b0, ok3 := parseRat(spy.Closes[0])
	bn, ok4 := parseRat(spy.Closes[back])
	if !ok1 || !ok2 || !ok3 || !ok4 || en.Sign() <= 0 || bn.Sign() <= 0 {
		return nil, liquidityapi.MissingInput
	}
	return new(big.Rat).Sub(pctChange(e0, en), pctChange(b0, bn)), ""
}

func pctChange(now, then *big.Rat) *big.Rat {
	r := new(big.Rat).Quo(now, then)
	r.Sub(r, big.NewRat(1, 1))
	return r.Mul(r, big.NewRat(100, 1))
}

// gapPair is one fund's two exact gaps.
type gapPair struct {
	short, long       *big.Rat
	shortWhy, longWhy compute.Reason
}

func pairOf(spy, s blockapi.CloseIn, day contracts.Day, p marketstateapi.Params) gapPair {
	var g gapPair
	g.short, g.shortWhy = rawGap(spy, s, day, p.ShortSessions, p.StaleWeekdays)
	g.long, g.longWhy = rawGap(spy, s, day, p.LongSessions, p.StaleWeekdays)
	return g
}

// cutAt is the series from the first close dated at or before day on, newest first.
func cutAt(s blockapi.CloseIn, day contracts.Day) blockapi.CloseIn {
	if len(s.Days) != len(s.Closes) {
		return blockapi.CloseIn{Name: s.Name, Symbol: s.Symbol}
	}
	for i, d := range s.Days {
		if d <= day {
			return blockapi.CloseIn{Name: s.Name, Symbol: s.Symbol, Closes: s.Closes[i:], Days: s.Days[i:]}
		}
	}
	return blockapi.CloseIn{Name: s.Name, Symbol: s.Symbol}
}

func skipDepth(s blockapi.CloseIn, n int) blockapi.CloseIn {
	if n > len(s.Closes) || len(s.Days) != len(s.Closes) {
		return blockapi.CloseIn{Name: s.Name, Symbol: s.Symbol}
	}
	return blockapi.CloseIn{Name: s.Name, Symbol: s.Symbol, Closes: s.Closes[n:], Days: s.Days[n:]}
}

// spotOf is the rotation point of a pair: X is the long gap, Y is short - long/3, each 2 places half-even.
func spotOf(g gapPair, day contracts.Day) (marketstateapi.Spot, bool) {
	if g.shortWhy != "" || g.longWhy != "" {
		return marketstateapi.Spot{}, false
	}
	y := new(big.Rat).Quo(g.long, big.NewRat(3, 1))
	y.Sub(g.short, y)
	return marketstateapi.Spot{Day: day, X: roundText(g.long, 2), Y: roundText(y, 2)}, true
}

// fundOf is one fund with its gaps, today's position left to the caller and the rotation tail.
func fundOf(in marketstateapi.Input, s blockapi.CloseIn, g gapPair) marketstateapi.Fund {
	p := in.Params
	f := marketstateapi.Fund{Symbol: s.Symbol, Name: s.Name}
	f.Gap21 = numOf(in.Benchmark, g.short, g.shortWhy, p.ShortSessions)
	f.Gap63 = numOf(in.Benchmark, g.long, g.longWhy, p.LongSessions)
	var endDay contracts.Day
	if len(in.Benchmark.Days) > 0 {
		endDay = in.Benchmark.Days[0]
	}
	today, ok := spotOf(g, endDay)
	if !ok {
		return f
	}
	for k := p.TailPoints; k >= 1; k-- {
		off := k * p.TailStep
		spy := skipDepth(in.Benchmark, off)
		if len(spy.Days) == 0 {
			continue
		}
		asOf := spy.Days[0]
		tg := pairOf(spy, cutAt(s, asOf), asOf, p)
		if sp, ok := spotOf(tg, asOf); ok {
			f.Rotation = append(f.Rotation, sp)
		}
	}
	f.Rotation = append(f.Rotation, today)
	return f
}

func numOf(spy blockapi.CloseIn, v *big.Rat, why compute.Reason, back int) liquidityapi.Num {
	if why != "" {
		return gapNum(why)
	}
	return liquidityapi.Num{Value: roundText(v, 2), Day: spy.Days[0], Aux: spy.Days[back]}
}

// rank fills the position of every fund at one window; vals[i] is nil for an unranked fund.
func rank(vals []*big.Rat, whys []compute.Reason) []marketstateapi.Position {
	order := make([]int, 0, len(vals))
	for i, v := range vals {
		if v != nil {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return vals[order[a]].Cmp(vals[order[b]]) > 0 })
	out := make([]marketstateapi.Position, len(vals))
	for at, i := range order {
		out[i] = marketstateapi.Position{At: at + 1, Of: len(order)}
	}
	for i, v := range vals {
		if v == nil {
			out[i] = marketstateapi.Position{Reason: whys[i]}
		}
	}
	return out
}

// sectors returns the ranked and sorted sector funds and the unranked industry funds.
func sectors(in marketstateapi.Input) ([]marketstateapi.Fund, []marketstateapi.Fund) {
	pairs := make([]gapPair, len(in.Funds))
	funds := make([]marketstateapi.Fund, len(in.Funds))
	shorts, longs := make([]*big.Rat, len(pairs)), make([]*big.Rat, len(pairs))
	shortWhy, longWhy := make([]compute.Reason, len(pairs)), make([]compute.Reason, len(pairs))
	for i, s := range in.Funds {
		pairs[i] = pairOf(in.Benchmark, s, in.Day, in.Params)
		funds[i] = fundOf(in, s, pairs[i])
		shorts[i], shortWhy[i] = pairs[i].short, pairs[i].shortWhy
		longs[i], longWhy[i] = pairs[i].long, pairs[i].longWhy
	}
	r21, r63 := rank(shorts, shortWhy), rank(longs, longWhy)
	for i := range funds {
		funds[i].Rank21, funds[i].Rank63 = r21[i], r63[i]
	}
	sort.SliceStable(funds, func(a, b int) bool {
		pa, pb := funds[a].Rank63, funds[b].Rank63
		if (pa.At == 0) != (pb.At == 0) {
			return pa.At != 0
		}
		return pa.At != 0 && pa.At < pb.At
	})
	ind := make([]marketstateapi.Fund, 0, len(in.Industry))
	for _, s := range in.Industry {
		ind = append(ind, fundOf(in, s, pairOf(in.Benchmark, s, in.Day, in.Params)))
	}
	return funds, ind
}
