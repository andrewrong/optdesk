package pagefig

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ blockapi.FamilyTrendFigure = familyTrend{}

type familyTrend struct{}

// NewFamilyTrend returns the family trend figure: per day, the sum in USD of the net value of every account.
func NewFamilyTrend() blockapi.FamilyTrendFigure { return familyTrend{} }

// trendPoint reads one day value as USD; ok is false when it is no point. yen reports a converted JPY point.
func trendPoint(d blockapi.DayValue, usdJpy map[contracts.Day]string) (r *big.Rat, yen, ok bool) {
	v, good := parse(d.NetLiquidation)
	if !good || v.Sign() < 0 {
		return nil, false, false
	}
	switch d.Currency {
	case "USD":
		return v, false, true
	case "JPY":
		rt := rate(usdJpy[d.Day])
		if rt == nil {
			return nil, false, false
		}
		return new(big.Rat).Quo(v, rt), true, true
	}
	return nil, false, false
}

func (familyTrend) FamilyTrend(in blockapi.FamilyTrendIn) blockapi.FamilyTrendResult {
	var out blockapi.FamilyTrendResult
	inWindow := make(map[contracts.Day]bool, len(in.Window))
	for _, d := range in.Window {
		inWindow[d] = true
	}
	// perDay holds the rounded USD point of each account by day; rounding first makes the thin lines add up to
	// the thick one exactly.
	perDay := make([]map[contracts.Day]*big.Rat, len(in.Accounts))
	for i, a := range in.Accounts {
		perDay[i] = map[contracts.Day]*big.Rat{}
		ao := blockapi.FamilyTrendAccountOut{Alias: a.Alias}
		for _, d := range a.Days {
			if !inWindow[d.Day] {
				continue
			}
			r, yen, ok := trendPoint(d, in.UsdJpy)
			if !ok {
				continue
			}
			txt := money(r)
			rounded, _ := parse(txt)
			if _, dup := perDay[i][d.Day]; dup {
				continue
			}
			perDay[i][d.Day] = rounded
			ao.Points = append(ao.Points, blockapi.FamilyTrendPoint{Day: d.Day, Net: txt})
			out.YenUsed = out.YenUsed || yen
		}
		out.Accounts = append(out.Accounts, ao)
	}
	for _, day := range in.Window {
		sum, all := new(big.Rat), len(in.Accounts) > 0
		for _, m := range perDay {
			p, has := m[day]
			if !has {
				all = false
				break
			}
			sum.Add(sum, p)
		}
		if !all {
			out.LeftOut++
			continue
		}
		out.Family = append(out.Family, blockapi.FamilyTrendPoint{Day: day, Net: money(sum)})
	}
	if len(out.Family) < 2 {
		out.Gap = "no_data"
	}
	return out
}
