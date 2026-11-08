package figures

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	minutesPerDay  = 1440
	minutesPerYear = 525600
	target30       = 30 * minutesPerDay
	// minDays is the first whole day count an expiry may have: expiries of 0 and 1 day are ignored.
	minDays = 2
)

// atmExpiry is one usable expiry: its length in minutes and its ATM IV.
type atmExpiry struct {
	minutes int
	iv      float64
}

// Iv30 is the 30-day IV of the chain by the arithmetic of GLOSSARY.md "Own IV30": per expiry
// the ATM IV at the strike nearest the forward, then the Cboe variance interpolation in
// minutes between the two expiries around 30 days, or the one expiry of exactly 30 days. It
// only reads the IBKR model IV of the quotes; it prices and solves nothing. StaleRate is true
// when rate.Day is not asOf. DataDay is asOf and Observations the count of expiries used.
func (c calc) Iv30(id string, chain []compute.ExpiryQuotes, rate compute.Point, asOf contracts.Day) compute.Figure {
	_, f, ok := c.begin(id, asOf)
	if !ok {
		return f
	}
	f.StaleRate = rate.Day != asOf
	if len(chain) == 0 {
		return missing(f, compute.NoInput)
	}
	f.DataDay = asOf
	if math.IsNaN(rate.Value) || math.IsInf(rate.Value, 0) {
		return missing(f, compute.BadInput)
	}
	var usable []atmExpiry
	for _, e := range chain {
		if e.MinutesToExpiry < minDays*minutesPerDay {
			continue
		}
		star, ok := strikeStar(e.Strikes)
		if !ok {
			continue
		}
		fwd := forward(star, rate.Value, e.MinutesToExpiry)
		if iv, ok := atmIV(e.Strikes, fwd); ok {
			usable = append(usable, atmExpiry{minutes: e.MinutesToExpiry, iv: iv})
		}
	}
	v, n, ok := interpolate(usable)
	if !ok {
		f.DataDay = ""
		return missing(f, compute.NoInput)
	}
	f.Value, f.Observations = v, n
	return f
}

// strikeStar is the strike with both legs and the smallest |C-P| (mids), the lower strike on
// a tie.
func strikeStar(strikes []compute.StrikeQuote) (compute.StrikeQuote, bool) {
	var best compute.StrikeQuote
	found := false
	for _, s := range strikes {
		if !s.HasCall || !s.HasPut {
			continue
		}
		if !found || math.Abs(s.CallMid-s.PutMid) < math.Abs(best.CallMid-best.PutMid) ||
			(math.Abs(s.CallMid-s.PutMid) == math.Abs(best.CallMid-best.PutMid) && s.Strike < best.Strike) {
			best, found = s, true
		}
	}
	return best, found
}

// forward is F = K* + exp(rT)(C-P) with T the minutes in years.
func forward(star compute.StrikeQuote, rate float64, minutes int) float64 {
	t := float64(minutes) / minutesPerYear
	return star.Strike + math.Exp(rate*t)*(star.CallMid-star.PutMid)
}

// atmIV is the mean of call and put IV at the strike nearest fwd (the lower on a tie). A
// strike without a model IV on either leg makes the expiry unusable; no other strike stands in.
func atmIV(strikes []compute.StrikeQuote, fwd float64) (float64, bool) {
	if len(strikes) == 0 {
		return 0, false
	}
	best := strikes[0]
	for _, s := range strikes[1:] {
		d, bd := math.Abs(s.Strike-fwd), math.Abs(best.Strike-fwd)
		if d < bd || (d == bd && s.Strike < best.Strike) {
			best = s
		}
	}
	if !best.HasCall || !best.HasPut || !(best.CallIV > 0) || !(best.PutIV > 0) {
		return 0, false
	}
	return (best.CallIV + best.PutIV) / 2, true
}

// interpolate joins the expiries around 30 days: one of exactly 30 days stands alone;
// otherwise the nearest shorter and the nearest longer are joined by the Cboe weights, with
// no extrapolation (a missing side is not ok). n is the count of expiries used.
func interpolate(exp []atmExpiry) (iv float64, n int, ok bool) {
	var near, far *atmExpiry
	for i := range exp {
		e := &exp[i]
		switch {
		case e.minutes == target30:
			return e.iv, 1, true
		case e.minutes < target30 && (near == nil || e.minutes > near.minutes):
			near = e
		case e.minutes > target30 && (far == nil || e.minutes < far.minutes):
			far = e
		}
	}
	if near == nil || far == nil {
		return 0, 0, false
	}
	n1, n2 := float64(near.minutes), float64(far.minutes)
	t1, t2 := n1/minutesPerYear, n2/minutesPerYear
	w1 := (n2 - target30) / (n2 - n1)
	w2 := (target30 - n1) / (n2 - n1)
	v := (t1*near.iv*near.iv*w1 + t2*far.iv*far.iv*w2) * minutesPerYear / target30
	return math.Sqrt(v), 2, true
}
