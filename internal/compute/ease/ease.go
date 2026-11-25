// Package ease computes the macro ease score: four inputs, each scored 0 to 100 between two
// fixed points, and the mean of those present. It is pure and works on exact rationals; the
// float values of the series come in as their shortest decimal text.
package ease

import (
	"math/big"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// calc is the compute.Ease of one Rules.
type calc struct {
	rules compute.Rules
}

// New is a compute.NewEaseFunc.
func New(rules compute.Rules) compute.Ease { return &calc{rules: rules} }

var _ compute.NewEaseFunc = New

var inputNames = [4]string{"net_liquidity", "us10y", "fed_funds", "hy_oas"}

// Ease computes the figure of id. An id the Rules do not know gives a missing figure with
// bad_input. A part older than its age is missing with stale_input; with fewer parts present
// than the rule's MinValid the figure is missing with no_input.
func (c *calc) Ease(id string, s compute.EaseSeries, asOf contracts.Day) compute.EaseFigure {
	rule, known := c.rules.Rule(id)
	out := compute.EaseFigure{
		Figure: compute.Figure{ID: id, Version: rule.Version, Day: asOf, Status: compute.StatusMissing},
		Parts:  make([]compute.EasePart, len(inputNames)),
	}
	if !known {
		out.Reason = compute.BadInput
		for i, name := range inputNames {
			out.Parts[i] = compute.EasePart{Input: name, Status: compute.StatusMissing, Reason: compute.BadInput}
		}
		return out
	}
	p := params()
	readings := [4]reading{
		netLiquidityChange(s.WALCL, s.WDTGAL, s.RRPONTSYD, asOf, p),
		usChange(s.DGS10, asOf, p),
		fedFundsChange(s.DFF, s.SP500, asOf, rule.Window, p),
		hyLevel(s.BAMLH0A0HYM2, asOf),
	}
	sum := new(big.Rat)
	for i, r := range readings {
		part := compute.EasePart{Input: inputNames[i], Status: compute.StatusMissing, Reason: r.reason}
		if r.figure != nil {
			part.DataDay = r.day
			part.Figure, _ = r.figure.Float64()
			if age(r.day, asOf, p.inputs[i]) > p.inputs[i].maxAge {
				part.Reason = compute.StaleInput
			} else {
				sub := score(r.figure, p.inputs[i])
				sum.Add(sum, sub)
				part.Score, _ = round2(sub).Float64()
				part.Status, part.Reason = compute.StatusOK, ""
				out.Observations++
				if r.day > out.DataDay {
					out.DataDay = r.day
				}
			}
		}
		out.Parts[i] = part
	}
	if out.Observations < rule.MinValid || out.Observations == 0 {
		out.Reason = compute.NoInput
		return out
	}
	value := round2(sum.Quo(sum, big.NewRat(int64(out.Observations), 1)))
	out.Value, _ = value.Float64()
	out.Status = compute.StatusOK
	out.Band = band(wholeHalfUp(value), p)
	return out
}

// age is how old a data day is on asOf, in the unit of the input.
func age(day, asOf contracts.Day, in input) int {
	if in.weekdays {
		return weekdaysAfter(day, asOf)
	}
	d, ok1 := parseDay(day)
	a, ok2 := parseDay(asOf)
	if !ok1 || !ok2 {
		return 0
	}
	return int(a.Sub(d).Hours() / 24)
}

// usChange is DGS10 on its newest observation minus the one usLookback observations
// earlier, in basis points.
func usChange(dgs10 compute.Series, asOf contracts.Day, p paramSet) reading {
	obs := ratsByDay(dgs10, asOf)
	days := sortedDays(obs)
	n := len(days)
	if n <= p.usLookback {
		return noFigure(obs)
	}
	d := new(big.Rat).Sub(obs[days[n-1]], obs[days[n-1-p.usLookback]])
	return reading{figure: d.Mul(d, p.basisPoints), day: days[n-1]}
}

// hyLevel is the newest BAMLH0A0HYM2 value.
func hyLevel(hy compute.Series, asOf contracts.Day) reading {
	obs := ratsByDay(hy, asOf)
	days := sortedDays(obs)
	if len(days) == 0 {
		return noFigure(obs)
	}
	return reading{figure: obs[days[len(days)-1]], day: days[len(days)-1]}
}

// ratOfFloat is the exact decimal of the shortest text of v, so 3.23 is 3.23 and not its
// binary neighbour. ok is false for a value that is not a finite number.
func ratOfFloat(v float64) (*big.Rat, bool) {
	return new(big.Rat).SetString(strconv.FormatFloat(v, 'g', -1, 64))
}

// score is 100 * (figure - at0) / (at100 - at0), clipped to 0..100.
func score(figure *big.Rat, in input) *big.Rat {
	s := new(big.Rat).Sub(figure, in.at0)
	s.Mul(s, big.NewRat(100, 1))
	s.Quo(s, new(big.Rat).Sub(in.at100, in.at0))
	if s.Sign() < 0 {
		return new(big.Rat)
	}
	if s.Cmp(big.NewRat(100, 1)) > 0 {
		return big.NewRat(100, 1)
	}
	return s
}

// round2 rounds half to even at 2 places.
func round2(r *big.Rat) *big.Rat {
	scaled := new(big.Rat).Mul(new(big.Rat).Abs(r), big.NewRat(100, 1))
	q, rem := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	switch rem.Lsh(rem, 1).Cmp(scaled.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	out := new(big.Rat).SetFrac(q, big.NewInt(100))
	if r.Sign() < 0 {
		out.Neg(out)
	}
	return out
}

// wholeHalfUp rounds r to a whole number, halves up.
func wholeHalfUp(r *big.Rat) int {
	n := new(big.Int).Lsh(r.Num(), 1)
	n.Add(n, r.Denom())
	d := new(big.Int).Lsh(r.Denom(), 1)
	return int(n.Div(n, d).Int64())
}

// band is the word of a whole number of the score.
func band(whole int, p paramSet) compute.EaseBand {
	switch {
	case whole <= p.tighterTo:
		return compute.EaseTighter
	case whole <= p.steadyTo:
		return compute.EaseSteady
	}
	return compute.EaseLooser
}

// weekdaysAfter counts the Monday to Friday days after day through asOf, with no holiday
// calendar. It is 0 when asOf is not after day or a day does not parse.
func weekdaysAfter(day, asOf contracts.Day) int {
	d, ok1 := parseDay(day)
	a, ok2 := parseDay(asOf)
	if !ok1 || !ok2 {
		return 0
	}
	n := 0
	for t := d.AddDate(0, 0, 1); !t.After(a); t = t.AddDate(0, 0, 1) {
		if wd := t.Weekday(); wd != time.Saturday && wd != time.Sunday {
			n++
		}
	}
	return n
}
