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
// spy is the benchmark cut at the common day; s is the fund cut at the same day.
func fundOf(p marketstateapi.Params, spy, s blockapi.CloseIn, common contracts.Day, g gapPair) marketstateapi.Fund {
	f := marketstateapi.Fund{Symbol: s.Symbol, Name: s.Name}
	f.Gap21 = numOf(spy, g.short, g.shortWhy, p.ShortSessions, common)
	f.Gap63 = numOf(spy, g.long, g.longWhy, p.LongSessions, common)
	today, ok := spotOf(g, common)
	if !ok {
		return f
	}
	for k := p.TailPoints; k >= 1; k-- {
		off := k * p.TailStep
		spy := skipDepth(spy, off)
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

// numOf is one cell; a fund that lacks a close names the common day it lacks.
func numOf(spy blockapi.CloseIn, v *big.Rat, why compute.Reason, back int, common contracts.Day) liquidityapi.Num {
	if why == liquidityapi.MissingInput {
		return liquidityapi.Num{Reason: why, Day: common}
	}
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

// commonDay is the newest day on which the benchmark and every fund that is not left out have a close. A fund is
// left out when it has no close or its newest close is more than StaleWeekdays weekdays behind the benchmark's
// newest day. It is empty when the benchmark has no closes; with no shared day it is the benchmark's newest day.
func commonDay(in marketstateapi.Input) contracts.Day {
	b := in.Benchmark
	if len(b.Days) == 0 || len(b.Days) != len(b.Closes) {
		return ""
	}
	var sets []map[contracts.Day]bool
	for _, list := range [][]blockapi.CloseIn{in.Funds, in.Industry} {
		for _, f := range list {
			if len(f.Days) == 0 || len(f.Days) != len(f.Closes) ||
				weekdaysBehind(f.Days[0], b.Days[0], in.Params.StaleWeekdays) > in.Params.StaleWeekdays {
				continue
			}
			set := make(map[contracts.Day]bool, len(f.Days))
			for _, d := range f.Days {
				set[d] = true
			}
			sets = append(sets, set)
		}
	}
next:
	for _, d := range b.Days {
		for _, set := range sets {
			if !set[d] {
				continue next
			}
		}
		return d
	}
	return b.Days[0]
}

// sectors returns the ranked and sorted sector funds and the unranked industry funds. Benchmark and funds are cut
// at the common day; a stale benchmark is judged on its own newest day.
func sectors(in marketstateapi.Input) ([]marketstateapi.Fund, []marketstateapi.Fund) {
	common := commonDay(in)
	spy, day := cutAt(in.Benchmark, common), common
	if len(in.Benchmark.Days) > 0 && weekdaysBehind(in.Benchmark.Days[0], in.Day, in.Params.StaleWeekdays) > in.Params.StaleWeekdays {
		spy, day = in.Benchmark, in.Day
	}
	one := func(s blockapi.CloseIn) (marketstateapi.Fund, gapPair) {
		c := cutAt(s, common)
		g := pairOf(spy, c, day, in.Params)
		return fundOf(in.Params, spy, c, common, g), g
	}
	pairs := make([]gapPair, len(in.Funds))
	funds := make([]marketstateapi.Fund, len(in.Funds))
	shorts, longs := make([]*big.Rat, len(pairs)), make([]*big.Rat, len(pairs))
	shortWhy, longWhy := make([]compute.Reason, len(pairs)), make([]compute.Reason, len(pairs))
	for i, s := range in.Funds {
		funds[i], pairs[i] = one(s)
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
		f, _ := one(s)
		ind = append(ind, f)
	}
	return funds, ind
}
