package ease

import (
	"math/big"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// reading is what one input gives before it is scored: an exact figure with its data day,
// or no figure (nil) and the reason.
type reading struct {
	figure *big.Rat
	day    contracts.Day
	reason compute.Reason
}

// parseDay reads a calendar day; ok is false for a text that is not one.
func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(time.DateOnly, string(d))
	return t, err == nil
}

func dayText(t time.Time) contracts.Day { return contracts.Day(t.Format(time.DateOnly)) }

// ratsByDay keys the points that have a value and lie on or before asOf by their day. A
// point with no value is not an observation.
func ratsByDay(s compute.Series, asOf contracts.Day) map[contracts.Day]*big.Rat {
	out := map[contracts.Day]*big.Rat{}
	for _, p := range s {
		if p.Day > asOf {
			continue
		}
		if r, ok := ratOfFloat(p.Value); ok {
			out[p.Day] = r
		}
	}
	return out
}

// sortedDays lists the days of m, oldest first.
func sortedDays(m map[contracts.Day]*big.Rat) []contracts.Day {
	days := make([]contracts.Day, 0, len(m))
	for d := range m {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i] < days[j] })
	return days
}

// noFigure is the reading of an input that cannot be had: no_input when one of the series
// it needs has no point, otherwise short_history.
func noFigure(maps ...map[contracts.Day]*big.Rat) reading {
	for _, m := range maps {
		if len(m) == 0 {
			return reading{reason: compute.NoInput}
		}
	}
	return reading{reason: compute.ShortHistory}
}

// weeklyNetLiquidity is the net liquidity of the Wednesday w: WALCL minus WDTGAL, both dated
// exactly w, minus the newest RRPONTSYD point of the week (w-7, w]. nil when one is missing.
func weeklyNetLiquidity(walcl, wdtgal, rrp map[contracts.Day]*big.Rat, w time.Time) *big.Rat {
	a, b := walcl[dayText(w)], wdtgal[dayText(w)]
	if a == nil || b == nil {
		return nil
	}
	for k := 0; k < 7; k++ {
		if r := rrp[dayText(w.AddDate(0, 0, -k))]; r != nil {
			out := new(big.Rat).Sub(a, b)
			return out.Sub(out, r)
		}
	}
	return nil
}

// changeBack is the change of the weekly net liquidity at w: the sum of the figures of the
// recent weeks ending at w over the sum of the same count of weeks, backWeeks earlier, minus
// 1. nil when one of those weeks has no figure or the earlier sum is 0.
func changeBack(figure func(time.Time) *big.Rat, w time.Time, p paramSet) *big.Rat {
	recent, earlier := new(big.Rat), new(big.Rat)
	for k := 0; k < p.recentWeeks; k++ {
		a := figure(w.AddDate(0, 0, -7*k))
		b := figure(w.AddDate(0, 0, -7*(p.backWeeks+k)))
		if a == nil || b == nil {
			return nil
		}
		recent.Add(recent, a)
		earlier.Add(earlier, b)
	}
	if earlier.Sign() == 0 {
		return nil
	}
	out := new(big.Rat).Quo(recent, earlier)
	return out.Sub(out, big.NewRat(1, 1))
}

// netLiquidityChange is the change at the newest Wednesday on or before asOf that has one.
func netLiquidityChange(walcl, wdtgal, rrp compute.Series, asOf contracts.Day, p paramSet) reading {
	wa, wd, rr := ratsByDay(walcl, asOf), ratsByDay(wdtgal, asOf), ratsByDay(rrp, asOf)
	end, ok := parseDay(asOf)
	if !ok || len(wa) == 0 {
		return noFigure(wa, wd, rr)
	}
	earliest, _ := parseDay(sortedDays(wa)[0])
	figure := func(w time.Time) *big.Rat { return weeklyNetLiquidity(wa, wd, rr, w) }
	span := 7 * (p.backWeeks + p.recentWeeks - 1)
	w := end.AddDate(0, 0, -((int(end.Weekday()) - int(time.Wednesday) + 7) % 7))
	for !w.AddDate(0, 0, -span).Before(earliest) {
		if c := changeBack(figure, w, p); c != nil {
			return reading{figure: c, day: dayText(w)}
		}
		w = w.AddDate(0, 0, -7)
	}
	return noFigure(wa, wd, rr)
}

// fedFundsChange is DFF on a session minus DFF on the session window sessions earlier, in
// basis points; the sessions are the days of sp500. It takes the newest session that has
// both.
func fedFundsChange(dff, sp500 compute.Series, asOf contracts.Day, window int, p paramSet) reading {
	rates, sessions := ratsByDay(dff, asOf), sortedDays(ratsByDay(sp500, asOf))
	for i := len(sessions) - 1; i-window >= 0; i-- {
		now, before := rates[sessions[i]], rates[sessions[i-window]]
		if now != nil && before != nil {
			d := new(big.Rat).Sub(now, before)
			return reading{figure: d.Mul(d, p.basisPoints), day: sessions[i]}
		}
	}
	return noFigure(rates, ratsByDay(sp500, asOf))
}
