package optioneod

import (
	"sort"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/broker"
)

type strikeValue struct {
	text  string
	value float64
}

// selectContracts picks the pairs to collect from one chain: the nearest maxExpiries expirations on or after
// day (YYYY-MM-DD), and of the strikes those inside price*(1-band) to price*(1+band), at most perSide below the
// price and at most perSide at or above it, nearest first; both rights. Strikes compare by value and are sent on as
// the text the chain gave. The result is ordered by expiry, strike and right (C before P).
func selectContracts(chain broker.Chain, u underlying, day string, price float64, m manifest) []broker.Contract {
	expiries := nearestExpiries(chain.Expirations, strings.ReplaceAll(day, "-", ""), m.MaxExpiries)
	strikes := nearestStrikes(chain.Strikes, price, m.StrikeBand, m.MaxStrikesPerSide)
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
