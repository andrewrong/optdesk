package pagefig

import (
	"math/big"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

var _ blockapi.FamilyFigure = familyFigure{}

type familyFigure struct{}

// NewFamily returns the family figure: the sum in USD of the latest capture of every account.
func NewFamily() blockapi.FamilyFigure { return familyFigure{} }

// famAcct is one counted account with its amounts in USD; nil where the account has none.
type famAcct struct {
	alias, day            string
	net, cash, prior, mon *big.Rat
}

// famUSD reads the row of tag as USD: the rate divides a JPY amount (rt nil for a USD account). nil when the
// row is missing, unreadable or in another currency.
func famUSD(vs []blockapi.ValueIn, tag, cur string, rt *big.Rat) *big.Rat {
	v, r, ok := tagValue(vs, tag)
	if !ok || v.Currency != cur {
		return nil
	}
	if rt != nil {
		return new(big.Rat).Quo(r, rt)
	}
	return r
}

func (familyFigure) Family(in blockapi.FamilyIn) blockapi.FamilyResult {
	out := blockapi.FamilyResult{Currency: "USD"}
	rt := rate(in.UsdJpy)
	seen := map[string]bool{}
	var counted []famAcct
	jpy := false
	for _, a := range in.Accounts {
		if seen[a.Alias] {
			continue
		}
		seen[a.Alias] = true
		row := blockapi.FamilyAccountOut{Alias: a.Alias, Reason: a.Reason, Day: a.Day}
		var f famAcct
		if row.Reason == "" {
			f, row.Reason = famCount(a, rt)
		}
		if row.Reason != "" {
			row.Net, row.Cash = gap(row.Reason), gap(row.Reason)
			out.Excluded = append(out.Excluded, row)
		} else {
			row.Net = compute.FigureValue{Value: money(f.net)}
			row.Cash = gap("missing_input")
			if f.cash != nil {
				row.Cash = compute.FigureValue{Value: money(f.cash)}
			}
			counted = append(counted, f)
			out.Included = append(out.Included, a.Alias)
			if v, _, _ := tagValue(a.Values, "NetLiquidation"); v.Currency == "JPY" {
				jpy = true
			}
		}
		out.Rows = append(out.Rows, row)
	}
	if jpy {
		out.FxRate, out.FxDay, out.FxSource = in.UsdJpy, in.RateDay, in.RateSource
	}
	if len(counted) == 0 {
		g := gap("no_data")
		out.Net, out.Cash, out.CashShare, out.DayAmount, out.DayPct, out.MonthAmount, out.MonthPct = g, g, g, g, g, g, g
		return out
	}
	famFigures(&out, counted)
	return out
}

// famCount reads the NetLiquidation of a in USD, or the reason that leaves the account out.
func famCount(a blockapi.FamilyAccountIn, rt *big.Rat) (famAcct, string) {
	v, r, ok := tagValue(a.Values, "NetLiquidation")
	var conv *big.Rat
	switch {
	case v.Currency == "JPY" && rt != nil:
		conv = rt
	case v.Currency != "USD":
		return famAcct{}, "unsupported_currency"
	}
	if !ok {
		return famAcct{}, "no_data"
	}
	f := famAcct{alias: a.Alias, day: a.Day, net: r}
	if conv != nil {
		f.net = new(big.Rat).Quo(r, conv)
	}
	f.cash = famUSD(a.Values, "TotalCashValue", v.Currency, conv)
	f.prior = famUSD(a.Prior, "NetLiquidation", v.Currency, conv)
	f.mon = famUSD(a.MonthStart, "NetLiquidation", v.Currency, conv)
	return f, ""
}

func famFigures(out *blockapi.FamilyResult, cs []famAcct) {
	net, cash := new(big.Rat), new(big.Rat)
	cashOK := true
	sameDay := true
	out.OldestDay = cs[0].day
	for _, c := range cs {
		net.Add(net, c.net)
		if c.cash == nil {
			cashOK = false
		} else {
			cash.Add(cash, c.cash)
		}
		if c.day != cs[0].day {
			sameDay = false
		}
		if strings.Compare(c.day, out.OldestDay) < 0 {
			out.OldestDay = c.day
		}
	}
	out.Net = compute.FigureValue{Value: money(net)}
	out.Cash, out.CashShare = gap("missing_input"), gap("missing_input")
	if cashOK {
		out.Cash = compute.FigureValue{Value: money(cash)}
		switch {
		case net.Sign() <= 0:
			out.CashShare = gap("no_data")
		case cash.Sign() < 0:
			out.CashShare = compute.FigureValue{}
		default:
			out.CashShare = share(cash, net, true)
		}
	}
	out.DayAmount, out.DayPct = famChange(cs, net, sameDay, func(c famAcct) *big.Rat { return c.prior })
	out.MonthAmount, out.MonthPct = famChange(cs, net, true, func(c famAcct) *big.Rat { return c.mon })
}

// famChange is the sum of net minus earlier(c) and its percent of the sum of the earlier nets; a gap when
// the days differ or an account lacks the earlier net.
func famChange(cs []famAcct, net *big.Rat, sameDay bool, earlier func(famAcct) *big.Rat) (amount, pct compute.FigureValue) {
	g := ""
	base := new(big.Rat)
	for _, c := range cs {
		e := earlier(c)
		if e == nil {
			g = "missing_input"
			break
		}
		base.Add(base, e)
	}
	if !sameDay {
		g = "stale_input"
	}
	if g != "" {
		return gap(g), gap(g)
	}
	diff := new(big.Rat).Sub(net, base)
	amount = compute.FigureValue{Value: money(diff)}
	return amount, share(diff, base, base.Sign() > 0)
}
