// Package liquidity computes the figures of the funding and credit block of the macro page:
// levels with their age rule, change over 4 and 13 weeks, SOFR minus IORB, the high-yield
// percentile, the reserves ratio and net liquidity. Pure: no I/O, no clock. All arithmetic is
// exact decimal (math/big over the shortest decimal text of each float64); results are text.
// Design: docs/design/liquidity-view.md Decision 8.
package liquidity

import (
	"math/big"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// TrialParams are the trial constants of the figures.
func TrialParams() liquidityapi.Params {
	return liquidityapi.Params{DailyMaxAgeDays: 4, WeeklyMaxAgeDays: 14, PercentileMinDays: 250}
}

func gap(r compute.Reason) liquidityapi.Num { return liquidityapi.Num{Reason: r} }

// ratOf is the exact decimal of the shortest text of v; ok is false for a non-finite value.
func ratOf(v float64) (*big.Rat, bool) {
	return new(big.Rat).SetString(strconv.FormatFloat(v, 'g', -1, 64))
}

func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, string(d))
	return t, err == nil
}

// ageDays is the calendar days from then to now; ok is false when either is not a day.
func ageDays(now, then contracts.Day) (int, bool) {
	a, ok1 := parseDay(now)
	b, ok2 := parseDay(then)
	if !ok1 || !ok2 {
		return 0, false
	}
	return int(a.Sub(b).Hours() / 24), true
}

func minusDays(d contracts.Day, n int) (contracts.Day, bool) {
	t, ok := parseDay(d)
	if !ok {
		return "", false
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format(time.DateOnly)), true
}

// roundText is x rounded half to even at places decimals, as text with exactly that many.
func roundText(x *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	n := new(big.Int).Mul(x.Num(), scale)
	q, m := new(big.Int).DivMod(n, x.Denom(), new(big.Int)) // floor, m >= 0
	twice := new(big.Int).Lsh(m, 1)
	switch c := twice.Cmp(x.Denom()); {
	case c > 0, c == 0 && q.Bit(0) == 1:
		q.Add(q, big.NewInt(1))
	}
	return new(big.Rat).SetFrac(q, scale).FloatString(places)
}

// exactText is a finite decimal exactly; ok is false when it does not terminate.
func exactText(x *big.Rat) string {
	for p := 0; p <= 40; p++ {
		scaled := new(big.Rat).Mul(x, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(p)), nil)))
		if scaled.IsInt() {
			return x.FloatString(p)
		}
	}
	return roundText(x, 6)
}

// point is the finite value of s on exactly day d.
func point(s compute.Series, d contracts.Day) (*big.Rat, bool) {
	for _, p := range s {
		if p.Day == d {
			if r, ok := ratOf(p.Value); ok {
				return r, true
			}
		}
	}
	return nil, false
}

// newest is the newest finite point on or before day.
func newest(s compute.Series, day contracts.Day) (d contracts.Day, v *big.Rat, ok bool) {
	for _, p := range s {
		if p.Day > day || (ok && p.Day < d) {
			continue
		}
		if r, fine := ratOf(p.Value); fine {
			d, v, ok = p.Day, r, true
		}
	}
	return
}

// fresh gives the reason a figure dated then is not current at now, or empty.
func fresh(now, then contracts.Day, maxAge int) compute.Reason {
	age, ok := ageDays(now, then)
	switch {
	case !ok:
		return compute.BadInput
	case age > maxAge:
		return compute.StaleInput
	}
	return ""
}

// level is the newest point on or before in.Day with its own day, under the age rule.
func level(s compute.Series, in liquidityapi.Input, maxAge int) liquidityapi.Num {
	d, v, ok := newest(s, in.Day)
	if !ok {
		return gap(compute.NoInput)
	}
	if r := fresh(in.Day, d, maxAge); r != "" {
		return liquidityapi.Num{Day: d, Reason: r}
	}
	return liquidityapi.Num{Value: exactText(v), Day: d}
}

// change is the newest value minus the value exactly 7*weeks days earlier.
func change(s compute.Series, in liquidityapi.Input, weeks int) liquidityapi.Change {
	out := liquidityapi.Change{Weeks: weeks}
	d, v, ok := newest(s, in.Day)
	if !ok {
		out.Reason = compute.NoInput
		return out
	}
	out.Day = d
	if r := fresh(in.Day, d, in.Params.WeeklyMaxAgeDays); r != "" {
		out.Reason = r
		return out
	}
	from, _ := minusDays(d, 7*weeks)
	out.From = from
	prev, ok := point(s, from)
	if !ok {
		out.Reason = liquidityapi.MissingInput
		return out
	}
	out.Value = exactText(new(big.Rat).Sub(v, prev))
	return out
}

// sofrIorb is SOFR minus IORB in basis points at the newest SOFR day on which IORB has a
// point on the same day.
func sofrIorb(in liquidityapi.Input) liquidityapi.Num {
	if len(in.SOFR) == 0 || len(in.IORB) == 0 {
		return gap(compute.NoInput)
	}
	var day contracts.Day
	var diff *big.Rat
	for _, p := range in.SOFR {
		if p.Day > in.Day || p.Day <= day {
			continue
		}
		sv, ok := ratOf(p.Value)
		iv, ok2 := point(in.IORB, p.Day)
		if ok && ok2 {
			day, diff = p.Day, new(big.Rat).Sub(sv, iv)
		}
	}
	if diff == nil {
		return gap(liquidityapi.MissingInput)
	}
	if r := fresh(in.Day, day, in.Params.DailyMaxAgeDays); r != "" {
		return liquidityapi.Num{Day: day, Reason: r}
	}
	return liquidityapi.Num{Value: roundText(diff.Mul(diff, big.NewRat(100, 1)), 1), Day: day}
}

// ratio is WRESBAL over TLAACBW027SBOG on the newest day both have a point, in percent.
func ratio(in liquidityapi.Input) liquidityapi.Num {
	if len(in.WRESBAL) == 0 || len(in.TLAACBW027SBOG) == 0 {
		return gap(compute.NoInput)
	}
	var day contracts.Day
	var num, den *big.Rat
	for _, p := range in.WRESBAL {
		if p.Day > in.Day || p.Day <= day {
			continue
		}
		a, ok := ratOf(p.Value)
		b, ok2 := point(in.TLAACBW027SBOG, p.Day)
		if ok && ok2 {
			day, num, den = p.Day, a, b
		}
	}
	if num == nil {
		return gap(liquidityapi.MissingInput)
	}
	if r := fresh(in.Day, day, in.Params.WeeklyMaxAgeDays); r != "" {
		return liquidityapi.Num{Day: day, Reason: r}
	}
	if den.Sign() == 0 {
		return liquidityapi.Num{Day: day, Reason: compute.BadInput}
	}
	pct := new(big.Rat).Quo(num, den)
	return liquidityapi.Num{Value: roundText(pct.Mul(pct, big.NewRat(100, 1)), 2), Day: day}
}

// net takes the value of in.Net as returned and applies the age rule to its Wednesday.
func net(in liquidityapi.Input) liquidityapi.Num {
	if in.Net == nil {
		return gap(compute.NoInput)
	}
	n := in.Net(in.WALCL, in.WDTGAL, in.RRPONTSYD, in.Day)
	if n.Reason != "" {
		n.Value = ""
		return n
	}
	if n.Value == "" {
		return gap(compute.NoInput)
	}
	if r := fresh(in.Day, n.Day, in.Params.WeeklyMaxAgeDays); r != "" {
		n.Value, n.Reason = "", r
	}
	return n
}

// Compute computes every figure of the block. It reads no clock and no store.
func Compute(in liquidityapi.Input) liquidityapi.Result {
	d, w := in.Params.DailyMaxAgeDays, in.Params.WeeklyMaxAgeDays
	return liquidityapi.Result{
		Reserves:           level(in.WRESBAL, in, w),
		RRP:                level(in.RRPONTSYD, in, d),
		Reserves4w:         change(in.WRESBAL, in, 4),
		Reserves13w:        change(in.WRESBAL, in, 13),
		SOFR:               level(in.SOFR, in, d),
		IORB:               level(in.IORB, in, d),
		SOFR99:             level(in.SOFR99, in, d),
		SofrIorbBP:         sofrIorb(in),
		HighYield:          level(in.BAMLH0A0HYM2, in, d),
		InvestmentGrade:    level(in.BAMLC0A0CM, in, d),
		Baa10Y:             level(in.BAA10Y, in, d),
		HighYieldRank:      rank(in.BAMLH0A0HYM2, in.Day, in.Day, in.Params),
		NFCI:               level(in.NFCI, in, w),
		STLFSI4:            level(in.STLFSI4, in, w),
		ReservesOverAssets: ratio(in),
		DiscountWindow:     level(in.WLCFLPCL, in, w),
		NetLiquidity:       net(in),
	}
}
