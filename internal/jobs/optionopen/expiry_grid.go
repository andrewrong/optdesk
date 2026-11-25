package optionopen

import (
	"math"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// The strike grid of an expiry (docs/design/option-open-limits.md, Decision 2): a chain lists one strike union for
// all its expiries, but a weekly lists a coarser grid than the monthly. The grid of an expiry is read from the
// strikes already stored listed at it, so a strike the expiry does not list is not asked.

type gridKey struct {
	expiry contracts.Day
	class  string
}

// expiryGrid holds the step of each stored expiry of a stock, by trading class.
type expiryGrid struct {
	steps map[gridKey]float64 // 0 when the expiry has fewer than two stored strikes
}

// newExpiryGrid reads the step of every expiry from the stored contracts: the smallest gap between two of its
// stored strikes.
func newExpiryGrid(refs []openquote.OptionRef) expiryGrid {
	strikes := map[gridKey]map[float64]bool{}
	for _, ref := range refs {
		v := strikeValue(ref.Strike)
		if v <= 0 {
			continue
		}
		k := gridKey{ref.Expiry, ref.TradingClass}
		if strikes[k] == nil {
			strikes[k] = map[float64]bool{}
		}
		strikes[k][v] = true
	}
	g := expiryGrid{steps: map[gridKey]float64{}}
	for k, set := range strikes {
		vs := make([]float64, 0, len(set))
		for v := range set {
			vs = append(vs, v)
		}
		slices.Sort(vs)
		step := 0.0
		for i := 1; i < len(vs); i++ {
			if d := vs[i] - vs[i-1]; step == 0 || d < step {
				step = d
			}
		}
		g.steps[k] = step
	}
	return g
}

// isMonthly is true for a third-Friday expiry.
func isMonthly(expiry contracts.Day) bool {
	t, err := time.Parse(time.DateOnly, string(expiry))
	return err == nil && t.Weekday() == time.Friday && t.Day() >= 15 && t.Day() <= 21
}

// stepOf is the step of an expiry: its own, else that of the nearest stored expiry of its kind (monthly or
// weekly) in the same class. It is 0 when there is none.
func (g expiryGrid) stepOf(expiry contracts.Day, class string) float64 {
	if s := g.steps[gridKey{expiry, class}]; s > 0 {
		return s
	}
	want := isMonthly(expiry)
	at, err := time.Parse(time.DateOnly, string(expiry))
	if err != nil {
		return 0
	}
	best, bestDist := 0.0, math.Inf(1)
	for k, s := range g.steps {
		if s <= 0 || k.class != class || isMonthly(k.expiry) != want {
			continue
		}
		t, err := time.Parse(time.DateOnly, string(k.expiry))
		if err != nil {
			continue
		}
		if d := math.Abs(t.Sub(at).Hours()); d < bestDist {
			best, bestDist = s, d
		}
	}
	return best
}

// allows says whether the expiry lists the strike on its grid. With no step known it allows every strike.
func (g expiryGrid) allows(expiry contracts.Day, class, strike string) bool {
	step := g.stepOf(expiry, class)
	if step <= 0 {
		return true
	}
	q := strikeValue(strike) / step
	return math.Abs(q-math.Round(q)) < 1e-6
}
