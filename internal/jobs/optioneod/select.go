package optioneod

import (
	"sort"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
)

// spotStrikesPerSide is how many strikes below the spot, and how many at or above it, an entry with a BuyPrice keeps
// on top of its range.
const spotStrikesPerSide = 2

type strikeValue struct {
	text  string
	value float64
}

// selectContracts picks the pairs to collect from one chain. With max_dte set the expirations are those with DTE
// in min_dte..max_dte, and a symbol with a BuyPrice takes its strikes from rangeStrikes (watch.go)
// and, per expiry, the two strikes below and the two at or above the spot (strike_band applies). Otherwise: the nearest
// maxExpiries expirations on or after
// day (YYYY-MM-DD), and of the strikes those inside price*(1-band) to price*(1+band), at most perSide below the
// price and at most perSide at or above it, nearest first; both rights. Strikes compare by value and are sent on as
// the text the chain gave. The result is ordered by expiry, strike and right (C before P).
func selectContracts(chain broker.Chain, u underlying, day string, price float64, m manifest) []broker.Contract {
	d := strings.ReplaceAll(day, "-", "")
	var expiries []string
	if m.MaxDTE > 0 {
		expiries = dteExpiries(chain.Expirations, d, m.MinDTE, m.MaxDTE)
	} else {
		expiries = nearestExpiries(chain.Expirations, d, m.MaxExpiries)
	}
	var strikes []strikeValue
	if bp := m.buy[u.Symbol]; bp != "" {
		limit := m.MaxStrikes
		if limit == 0 {
			limit = defaultMaxStrikes
		}
		strikes = mergeStrikes(rangeStrikes(chain.Strikes, bp, price, limit),
			nearestStrikes(chain.Strikes, price, m.StrikeBand, spotStrikesPerSide))
	} else {
		strikes = nearestStrikes(chain.Strikes, price, m.StrikeBand, m.MaxStrikesPerSide)
	}
	out := make([]broker.Contract, 0, len(expiries)*len(strikes)*2)
	for _, e := range expiries {
		for _, s := range strikes {
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

// mergeStrikes joins two strike lists without a strike twice (by value), ordered by value.
func mergeStrikes(a, b []strikeValue) []strikeValue {
	seen := make(map[float64]bool, len(a)+len(b))
	out := make([]strikeValue, 0, len(a)+len(b))
	for _, s := range append(append([]strikeValue{}, a...), b...) {
		if !seen[s.value] {
			seen[s.value] = true
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].value < out[j].value })
	return out
}

func nearestExpiries(all []string, day string, n int) []string {
	var keep []string
	for _, e := range all {
		if e >= day {
			keep = append(keep, e)
		}
	}
	sort.Strings(keep)
	if len(keep) > n {
		keep = keep[:n]
	}
	return keep
}

func nearestStrikes(all []string, price, band float64, perSide int) []strikeValue {
	lo, hi := price*(1-band), price*(1+band)
	var below, above []strikeValue
	for _, t := range all {
		v, err := strconv.ParseFloat(t, 64)
		if err != nil || v < lo || v > hi {
			continue
		}
		if v < price {
			below = append(below, strikeValue{t, v})
		} else {
			above = append(above, strikeValue{t, v})
		}
	}
	sort.Slice(below, func(i, j int) bool { return below[i].value > below[j].value })
	sort.Slice(above, func(i, j int) bool { return above[i].value < above[j].value })
	if len(below) > perSide {
		below = below[:perSide]
	}
	if len(above) > perSide {
		above = above[:perSide]
	}
	out := append(below, above...)
	sort.Slice(out, func(i, j int) bool { return out[i].value < out[j].value })
	return out
}
