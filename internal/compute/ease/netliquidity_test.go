package ease

import (
	"math"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// nlPart runs Ease on the three liquidity series and gives the net_liquidity part.
func nlPart(walcl, wdtgal, rrp compute.Series, asOf contracts.Day) compute.EasePart {
	s := compute.EaseSeries{WALCL: walcl, WDTGAL: wdtgal, RRPONTSYD: rrp}
	return newEase().Ease("macro.ease", s, asOf).Parts[0]
}

func TestNetLiquidityChange(t *testing.T) {
	t.Parallel()
	walcl, wdtgal, rrp := nlSeries(wed, weeksUp(17, 101.5), 1e9)
	p := nlPart(walcl, wdtgal, rrp, wed)
	if p.Status != compute.StatusOK || p.Figure != 0.015 || p.Score != 75 || p.DataDay != wed {
		t.Fatalf("part %+v", p)
	}
}

func TestRRPNewestInWeek(t *testing.T) {
	t.Parallel()
	week2 := addDays(wed, -14)
	moved := func(to contracts.Day) compute.Series {
		_, _, rrp := nlSeries(wed, weeksUp(17, 101.5), 1e9)
		var out compute.Series
		for _, pt := range rrp {
			if pt.Day == addDays(week2, -1) {
				pt.Day = to
			}
			out = append(out, pt)
		}
		return sortSeries(out)
	}
	walcl, wdtgal, _ := nlSeries(wed, weeksUp(17, 101.5), 1e9)
	// Monday of the week still lies in (W-7, W].
	if p := nlPart(walcl, wdtgal, moved(addDays(week2, -2)), wed); p.Status != compute.StatusOK {
		t.Errorf("a point inside the week: %+v", p)
	}
	// A point dated W-7 belongs to the week before.
	p := nlPart(walcl, wdtgal, moved(addDays(week2, -7)), wed)
	if p.Status != compute.StatusMissing || p.Reason != compute.ShortHistory {
		t.Errorf("a point dated W-7: %+v", p)
	}
}

func TestWalclNotWednesdayIgnored(t *testing.T) {
	t.Parallel()
	walcl, wdtgal, rrp := nlSeries(wed, weeksUp(17, 101.5), 1e9)
	thu := addDays(wed, 1)
	walcl[len(walcl)-1].Day = thu
	p := nlPart(walcl, wdtgal, rrp, thu)
	if p.Status != compute.StatusMissing || p.Reason != compute.ShortHistory {
		t.Fatalf("part %+v", p)
	}
}

func TestMissingWeekNoChange(t *testing.T) {
	t.Parallel()
	for _, k := range []int{0, 1, 2, 3, 13, 14, 15, 16} {
		weeks := weeksUp(17, 101.5)
		weeks[k] = math.NaN()
		walcl, wdtgal, rrp := nlSeries(wed, weeks, 1e9)
		p := nlPart(walcl, wdtgal, rrp, wed)
		if p.Status != compute.StatusMissing || p.Reason != compute.ShortHistory {
			t.Errorf("week %d missing: %+v", k, p)
		}
	}
	weeks := weeksUp(17, 101.5)
	weeks[7] = math.NaN()
	walcl, wdtgal, rrp := nlSeries(wed, weeks, 1e9)
	if p := nlPart(walcl, wdtgal, rrp, wed); p.Status != compute.StatusOK || p.Figure != 0.015 {
		t.Errorf("an unsummed week missing: %+v", p)
	}
}

func TestEarlierSumZeroNoChange(t *testing.T) {
	t.Parallel()
	weeks := weeksUp(17, 101.5)
	for k := 13; k < 17; k++ {
		weeks[k] = 0
	}
	walcl, wdtgal, rrp := nlSeries(wed, weeks, 1e9)
	p := nlPart(walcl, wdtgal, rrp, wed)
	if p.Status != compute.StatusMissing || p.Reason != compute.ShortHistory {
		t.Fatalf("part %+v", p)
	}
}

func TestFallsBackToOlderWednesday(t *testing.T) {
	t.Parallel()
	weeks := weeksUp(18, 101.5)
	weeks = append([]float64{math.NaN()}, weeks[:17]...) // week 0 lacks WDTGAL; weeks 1 to 4 are up
	walcl, wdtgal, rrp := nlSeries(wed, weeks, 1e9)
	prev := addDays(wed, -7)
	fresh := nlPart(walcl, wdtgal, rrp, addDays(wed, 1)) // prev is 8 days old
	if fresh.Status != compute.StatusOK || fresh.Figure != 0.015 || fresh.DataDay != prev {
		t.Errorf("8 days old: %+v", fresh)
	}
	stale := nlPart(walcl, wdtgal, rrp, addDays(wed, 4)) // prev is 11 days old
	if stale.Status != compute.StatusMissing || stale.Reason != compute.StaleInput || stale.DataDay != prev {
		t.Errorf("11 days old: %+v", stale)
	}
	edge := nlPart(walcl, wdtgal, rrp, addDays(wed, 3)) // 10 days old
	if edge.Status != compute.StatusOK {
		t.Errorf("10 days old: %+v", edge)
	}
}

func TestScaleInvariance(t *testing.T) {
	t.Parallel()
	weeks := weeksUp(17, 101.5)
	a1, a2, a3 := nlSeries(wed, weeks, 1)
	b1, b2, b3 := nlSeries(wed, weeks, 1e9)
	small, big := nlPart(a1, a2, a3, wed), nlPart(b1, b2, b3, wed)
	if small.Status != compute.StatusOK || small.Figure != big.Figure || small.Figure != 0.015 {
		t.Fatalf("small %+v, big %+v", small, big)
	}
}

func TestFedFundsOnSp500Sessions(t *testing.T) {
	t.Parallel()
	const n = 128
	sp := weekdayPoints(wed, n, func(int) float64 { return 5000 })
	rate := func(i int) float64 { return float64(i*i) / 100 }
	dff := weekdayPoints(wed, n, func(i int) float64 { return rate(i) })
	part := func(dff compute.Series) compute.EasePart {
		s := compute.EaseSeries{DFF: dff, SP500: sp}
		return newEase().Ease("macro.ease", s, wed).Parts[2]
	}
	base := part(dff)
	if base.Status != compute.StatusOK || base.Figure != 16128 || base.DataDay != wed {
		t.Fatalf("base %+v", base)
	}
	withWeekend := append(compute.Series(nil), dff...)
	for d := addDays(wed, -200); d <= wed; d = addDays(d, 1) {
		if isWeekend(d) {
			withWeekend = append(withWeekend, compute.Point{Day: d, Value: 999})
		}
	}
	if got := part(sortSeries(withWeekend)); got != base {
		t.Errorf("weekend points moved the figure: %+v, want %+v", got, base)
	}
	// The newest session has no DFF: the newest earlier session with one is used.
	if got := part(dff[:n-1]); got.Status != compute.StatusOK || got.Figure != 15876 || got.DataDay != addDays(wed, -1) {
		t.Errorf("fallback %+v", got)
	}
}

func TestNoValuePointNotCounted(t *testing.T) {
	t.Parallel()
	val := func(i int) float64 {
		if i == 0 {
			return 4
		}
		return 4.5
	}
	s := weekdayPoints(wed, 65, val)
	s[1].Value = math.NaN()
	got := newEase().Ease("macro.ease", compute.EaseSeries{DGS10: s}, wed).Parts[1]
	if got.Status != compute.StatusOK || got.Figure != 50 {
		t.Errorf("65 points, one without a value: %+v", got)
	}
	short := weekdayPoints(wed, 64, val)
	short[1].Value = math.NaN()
	got = newEase().Ease("macro.ease", compute.EaseSeries{DGS10: short}, wed).Parts[1]
	if got.Status != compute.StatusMissing || got.Reason != compute.ShortHistory {
		t.Errorf("64 points, one without a value: %+v", got)
	}
}

func TestGoldenWeekly(t *testing.T) {
	t.Parallel()
	for _, c := range loadGolden(t) {
		if c.Kind != "weekly" {
			continue
		}
		weeks := make([]float64, len(c.Weeks))
		for i, w := range c.Weeks {
			weeks[i] = math.NaN()
			if w != nil {
				weeks[i] = *w
			}
		}
		walcl, wdtgal, rrp := nlSeries(wed, weeks, 1e9)
		p := nlPart(walcl, wdtgal, rrp, wed)
		if c.WantChange == nil {
			if p.Status != compute.StatusMissing || string(p.Reason) != c.WantReason {
				t.Errorf("%s: %+v", c.Name, p)
			}
			continue
		}
		if p.Status != compute.StatusOK || p.Figure != *c.WantChange {
			t.Errorf("%s: %+v, want %v", c.Name, p, *c.WantChange)
		}
	}
}

func TestNetLiquidityLevelMatchesEasePart(t *testing.T) {
	t.Parallel()
	walcl, wdtgal, rrp := nlSeries(wed, weeksUp(17, 101.5), 1e9)
	part := nlPart(walcl, wdtgal, rrp, wed)
	got := NetLiquidity(walcl, wdtgal, rrp, wed)
	// WALCL 131.5e9 minus WDTGAL 10e9 minus RRP 20e9 on the Wednesday of the part.
	if got.Reason != "" || got.Day != part.DataDay || got.Value != "101500000000" || got.Aux != addDays(wed, -1) {
		t.Fatalf("level %+v, part %+v", got, part)
	}
	// A Thursday asks for the same Wednesday; a day with no WALCL gives no_input.
	if g := NetLiquidity(walcl, wdtgal, rrp, addDays(wed, 1)); g.Day != wed || g.Value != got.Value {
		t.Errorf("thursday %+v", g)
	}
	if g := NetLiquidity(nil, wdtgal, rrp, wed); g.Value != "" || g.Reason != compute.NoInput {
		t.Errorf("no walcl %+v", g)
	}
}
