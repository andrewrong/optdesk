package optioneod

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
)

// selectIndex picks the call-put pairs of one index chain (Decision 7 of docs/design/option-capture-need.md): the
// expiries by expiry_kind within min_dte..max_dte, the first max_expiries of them; per expiry N = max_contracts /
// (2 x expiries) strikes that are multiples of strike_multiple: the one nearest to spot always, the others spread
// evenly over spot +- W, W = spot x band_k x sigma x sqrt(T), each the nearest multiple of the band not yet taken
// (a tie keeps the lower). Strike and expiry are the chain's own text. Ordered by expiry, strike, C before P.
func selectIndex(chain broker.Chain, u underlying, row indexRow, m manifest, day string, spot, sigma float64) []broker.Contract {
	d := strings.ReplaceAll(day, "-", "")
	expiries := dteExpiries(chain.Expirations, d, m.MinDTE, m.MaxDTE)
	if row.ExpiryKind == expiryThirdFriday {
		expiries = monthlies(expiries, chain.Expirations)
	}
	if len(expiries) > m.MaxExpiries {
		expiries = expiries[:m.MaxExpiries]
	}
	if len(expiries) == 0 || !(spot > 0) {
		return nil
	}
	n := max(row.MaxContracts/(2*len(expiries)), 1)
	multiples := multipleStrikes(chain.Strikes, row.StrikeMultiple)
	today, _ := time.Parse("20060102", d)
	var out []broker.Contract
	for _, e := range expiries {
		t, _ := time.Parse("20060102", e)
		days := max(t.Sub(today).Hours()/24, 1)
		w := spot * m.BandK * sigma * math.Sqrt(days/365)
		for _, s := range spreadStrikes(multiples, spot, w, n) {
			for _, right := range []string{"C", "P"} {
				out = append(out, broker.Contract{
					SecType: "OPT", Symbol: u.Symbol, Exchange: chain.Exchange, Currency: u.Currency,
					Expiry: e, Strike: s.text, Right: right, Multiplier: chain.Multiplier, TradingClass: chain.TradingClass,
				})
			}
		}
	}
	return out
}

// monthlies keeps of the expiries the monthly of each month: the third Friday, or the Thursday before it when that
// Friday is not among all (the expiries of the chain).
func monthlies(expiries, all []string) []string {
	listed := make(map[string]bool, len(all))
	for _, e := range all {
		listed[e] = true
	}
	var out []string
	for _, e := range expiries {
		t, err := time.Parse("20060102", e)
		if err != nil {
			continue
		}
		switch {
		case t.Weekday() == time.Friday && t.Day() >= 15 && t.Day() <= 21:
			out = append(out, e)
		case t.Weekday() == time.Thursday && t.Day() >= 14 && t.Day() <= 20 && !listed[t.AddDate(0, 0, 1).Format("20060102")]:
			out = append(out, e)
		}
	}
	return out
}

// multipleStrikes is the strikes of the chain that are multiples of step, once each, ascending.
func multipleStrikes(all []string, step int) []strikeValue {
	seen := map[float64]bool{}
	var out []strikeValue
	for _, t := range all {
		v, err := strconv.ParseFloat(t, 64)
		if err != nil || v <= 0 || seen[v] {
			continue
		}
		if q := v / float64(step); math.Abs(q-math.Round(q)) > 1e-9 {
			continue
		}
		seen[v] = true
		out = append(out, strikeValue{t, v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].value < out[j].value })
	return out
}

// spreadStrikes takes n strikes of the ascending list: the one nearest spot, then n-1 spread evenly over
// spot-w..spot+w, each the nearest strike of the band not yet taken. The result is ascending.
func spreadStrikes(all []strikeValue, spot, w float64, n int) []strikeValue {
	if len(all) == 0 {
		return nil
	}
	taken := map[int]bool{}
	atm := nearestIndex(all, spot, nil, -math.MaxFloat64, math.MaxFloat64)
	taken[atm] = true
	lo, hi := spot-w, spot+w
	for i := 0; i < n-1; i++ {
		target := spot
		if n > 2 {
			target = lo + float64(i)*(hi-lo)/float64(n-2)
		}
		k := nearestIndex(all, target, taken, lo, hi)
		if k < 0 {
			break
		}
		taken[k] = true
	}
	out := make([]strikeValue, 0, len(taken))
	for i, s := range all {
		if taken[i] {
			out = append(out, s)
		}
	}
	return out
}

// nearestIndex is the index of the strike in lo..hi not in taken nearest to target (a tie keeps the lower), or -1.
func nearestIndex(all []strikeValue, target float64, taken map[int]bool, lo, hi float64) int {
	best := -1
	for i, s := range all {
		if taken[i] || s.value < lo || s.value > hi {
			continue
		}
		if best < 0 || math.Abs(s.value-target) < math.Abs(all[best].value-target) {
			best = i
		}
	}
	return best
}

// completePairs drops the contracts of a strike (of an expiry) that is missing a right, so that what is stored is
// a set of listed call-put pairs. The order is kept.
func completePairs(cs []broker.Contract) []broker.Contract {
	type key struct {
		expiry string
		strike float64
	}
	rights := map[key]map[string]bool{}
	keyOf := func(c broker.Contract) key {
		v, _ := strconv.ParseFloat(c.Strike, 64)
		return key{c.Expiry, v}
	}
	for _, c := range cs {
		k := keyOf(c)
		if rights[k] == nil {
			rights[k] = map[string]bool{}
		}
		rights[k][c.Right] = true
	}
	out := cs[:0:0]
	for _, c := range cs {
		if r := rights[keyOf(c)]; r["C"] && r["P"] {
			out = append(out, c)
		}
	}
	return out
}

// annualisedSigma is the standard deviation (sample, n-1) of the last days log returns of the closes, times the
// square root of 252; fallback when there are fewer than days+1 closes or a close is not a positive number.
func annualisedSigma(closes []string, days int, fallback float64) float64 {
	if days < 2 || len(closes) < days+1 {
		return fallback
	}
	last := closes[len(closes)-days-1:]
	prev := 0.0
	rets := make([]float64, 0, days)
	for i, t := range last {
		v, err := strconv.ParseFloat(t, 64)
		if err != nil || !(v > 0) {
			return fallback
		}
		if i > 0 {
			rets = append(rets, math.Log(v/prev))
		}
		prev = v
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	ss := 0.0
	for _, r := range rets {
		ss += (r - mean) * (r - mean)
	}
	sd := math.Sqrt(ss/float64(len(rets)-1)) * math.Sqrt(252)
	if !(sd > 0) {
		return fallback
	}
	return sd
}
