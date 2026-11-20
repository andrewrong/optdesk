package optioncandidates

import (
	"math/big"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// spreadFigures gives the least and the median of (ask - bid) / mid over the puts of one name that expire
// inside the DTE window [minDTE, maxDTE] of day and have a usable quote with a bid above 0 and an ask, as
// decimal text with four digits after the point. Both are empty when no put qualifies. The unit is the one
// of the max_spread limit: a ratio of the mid price. The median of an even count is the mean of the two
// middle values.
func spreadFigures(quotes []compute.OptionQuote, day contracts.Day, minDTE, maxDTE int) (least, median string) {
	d, ok := parseDay(day)
	if !ok {
		return "", ""
	}
	var rs []*big.Rat
	for _, q := range quotes {
		if q.Right != "P" || !q.QuoteOK {
			continue
		}
		exp, err := time.Parse(time.DateOnly, string(q.Expiry))
		if err != nil {
			continue
		}
		if dte := int(exp.Sub(d).Hours() / 24); dte < minDTE || dte > maxDTE {
			continue
		}
		bid, okB := new(big.Rat).SetString(q.Bid)
		ask, okA := new(big.Rat).SetString(q.Ask)
		if !okB || !okA || bid.Sign() <= 0 {
			continue
		}
		mid := new(big.Rat).Add(ask, bid)
		mid.Quo(mid, big.NewRat(2, 1))
		if mid.Sign() <= 0 {
			continue
		}
		r := new(big.Rat).Sub(ask, bid)
		rs = append(rs, r.Quo(r, mid))
	}
	if len(rs) == 0 {
		return "", ""
	}
	sort.Slice(rs, func(a, b int) bool { return rs[a].Cmp(rs[b]) < 0 })
	n := len(rs)
	med := rs[n/2]
	if n%2 == 0 {
		med = new(big.Rat).Add(rs[n/2-1], rs[n/2])
		med.Quo(med, big.NewRat(2, 1))
	}
	return rs[0].FloatString(4), med.FloatString(4)
}
