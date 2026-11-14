package liquidity

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// rank is the percentile of the newest value among all stored points up to day: the share of
// days whose value is at or below it (a tie counts), in percent, 0 decimals. The window is
// whatever is stored; First, Day and Days describe it.
func rank(s compute.Series, day contracts.Day, now contracts.Day, p liquidityapi.Params) liquidityapi.Rank {
	var vals []*big.Rat
	var days []contracts.Day
	for _, pt := range s {
		if pt.Day > day {
			continue
		}
		if r, ok := ratOf(pt.Value); ok {
			vals = append(vals, r)
			days = append(days, pt.Day)
		}
	}
	if len(vals) == 0 {
		return liquidityapi.Rank{Num: gap(compute.NoInput)}
	}
	newest, last := 0, days[0]
	for i, d := range days {
		if d >= last {
			newest, last = i, d
		}
	}
	out := liquidityapi.Rank{First: days[0], Days: len(vals)}
	for _, d := range days {
		if d < out.First {
			out.First = d
		}
	}
	out.Day = last
	if age, ok := ageDays(now, last); !ok {
		out.Reason = compute.BadInput
		return out
	} else if age > p.DailyMaxAgeDays {
		out.Reason = compute.StaleInput
		return out
	}
	if len(vals) < p.PercentileMinDays {
		out.Reason = compute.ShortHistory
		return out
	}
	below := 0
	for _, v := range vals {
		if v.Cmp(vals[newest]) <= 0 {
			below++
		}
	}
	pct := big.NewRat(int64(below)*100, int64(len(vals)))
	out.Value = roundText(pct, 0)
	return out
}
