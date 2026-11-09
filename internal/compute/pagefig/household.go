package pagefig

import (
	"math/big"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// trendDays is the number of session days the trend keeps.
const trendDays = 90

// stressSteps are the underlying drops of the stress rows, in percent (window default: the
// owner may change them).
var stressSteps = []string{"10", "20", "30"}

func gap(g string) compute.FigureValue { return compute.FigureValue{Gap: g} }

// parse reads decimal text; ok is false for empty or malformed text.
func parse(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// money formats r rounded half-even to 2 places.
func money(r *big.Rat) string {
	scaled := new(big.Rat).Mul(r, big.NewRat(100, 1))
	q, rem := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	// QuoRem truncates toward zero; rem has the sign of the numerator.
	twice := new(big.Int).Abs(rem)
	twice.Lsh(twice, 1)
	switch c := twice.Cmp(scaled.Denom()); {
	case c > 0, c == 0 && q.Bit(0) == 1:
		if scaled.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	neg := q.Sign() < 0
	s := new(big.Int).Abs(q).String()
	for len(s) < 3 {
		s = "0" + s
	}
	s = s[:len(s)-2] + "." + s[len(s)-2:]
	if neg {
		s = "-" + s
	}
	return s
}

// wholeUSD formats r rounded half up to whole units.
func wholeUSD(r *big.Rat) string {
	half := new(big.Rat).Add(new(big.Rat).Abs(r), big.NewRat(1, 2))
	n := new(big.Int).Quo(half.Num(), half.Denom())
	if r.Sign() < 0 && n.Sign() != 0 {
		n.Neg(n)
	}
	return n.String()
}

func tagValue(vs []blockapi.ValueIn, tag string) (blockapi.ValueIn, *big.Rat, bool) {
	for _, v := range vs {
		if v.Tag == tag {
			r, ok := parse(v.Value)
			return v, r, ok
		}
	}
	return blockapi.ValueIn{}, nil, false
}

// line makes a money row. A JPY amount needs the USD/JPY rate: without it the amount is the gap
// unsupported_currency; with it the yen stay the amount and the note carries the display-only ≈USD.
func line(name, currency string, amount *big.Rat, usdJpy string) blockapi.MoneyLine {
	l := blockapi.MoneyLine{Name: name, Currency: currency}
	if currency == "JPY" {
		rate, ok := parse(usdJpy)
		if !ok || rate.Sign() <= 0 {
			l.Amount = gap("unsupported_currency")
			return l
		}
		l.Note = "≈USD " + wholeUSD(new(big.Rat).Quo(amount, rate))
	}
	l.Amount = compute.FigureValue{Value: money(amount)}
	return l
}

func gapLine(name, currency, g string) blockapi.MoneyLine {
	return blockapi.MoneyLine{Name: name, Currency: currency, Amount: gap(g)}
}

// notional is strike x multiplier x contracts of one short put.
func notional(o blockapi.ObligationIn) (*big.Rat, bool) {
	s, ok1 := parse(o.Strike)
	m, ok2 := parse(o.Multiplier)
	n, ok3 := parse(o.Contracts)
	if !ok1 || !ok2 || !ok3 {
		return nil, false
	}
	return new(big.Rat).Mul(new(big.Rat).Mul(s, m), n), true
}

func household(in blockapi.HouseholdInput) blockapi.HouseholdResult {
	net, netR, netOK := tagValue(in.Values, "NetLiquidation")
	cash, cashR, cashOK := tagValue(in.Values, "TotalCashValue")
	r := blockapi.HouseholdResult{
		Overview:    overview(net, netR, netOK, in.UsdJpy),
		Accounts:    accounts(net, netR, netOK, cash, cashR, cashOK, in.UsdJpy),
		Obligations: obligations(in),
		Income:      income(in.Obligations),
		Trend:       trend(in.History),
	}
	after := cashAfter(cash, cashR, cashOK, in)
	r.CashAfter = []blockapi.MoneyLine{after}
	r.Stress = stress(after)
	r.MonthChange = monthChange(in, net, netR, netOK)
	return r
}

// overview is the figure household.overview: one row, 净值, from NetLiquidation.
func overview(net blockapi.ValueIn, r *big.Rat, ok bool, usdJpy string) []blockapi.MoneyLine {
	if !ok {
		return []blockapi.MoneyLine{gapLine("净值", net.Currency, "missing_input")}
	}
	return []blockapi.MoneyLine{line("净值", net.Currency, r, usdJpy)}
}

// accounts is the figure household.accounts: net liquidation and cash from the summary; the
// stock and option cells need quotes this input does not carry, so they are the gap missing_quote.
func accounts(net blockapi.ValueIn, netR *big.Rat, netOK bool, cash blockapi.ValueIn, cashR *big.Rat, cashOK bool, usdJpy string) []blockapi.MoneyLine {
	netLine := gapLine("净清算值", net.Currency, "missing_input")
	if netOK {
		netLine = line("净清算值", net.Currency, netR, usdJpy)
	}
	cashLine := gapLine("现金", cash.Currency, "missing_input")
	if cashOK {
		cashLine = line("现金", cash.Currency, cashR, usdJpy)
		if netOK && netR.Sign() > 0 && cash.Currency == net.Currency {
			share := new(big.Rat).Quo(new(big.Rat).Mul(cashR, big.NewRat(100, 1)), netR)
			cashLine.Share = compute.FigureValue{Value: money(share)}
		}
	}
	return []blockapi.MoneyLine{
		netLine, cashLine,
		gapLine("股票市值", net.Currency, "missing_quote"),
		gapLine("期权市值", net.Currency, "missing_quote"),
	}
}

// cashAfter is the figure household.cash_after: TotalCashValue minus the gross put notional of
// every obligation, signed (a negative is a shortfall).
func cashAfter(cash blockapi.ValueIn, cashR *big.Rat, cashOK bool, in blockapi.HouseholdInput) blockapi.MoneyLine {
	const name = "指派后现金"
	if !cashOK {
		return gapLine(name, cash.Currency, "missing_input")
	}
	left := new(big.Rat).Set(cashR)
	for _, o := range in.Obligations {
		n, ok := notional(o)
		if !ok {
			return gapLine(name, cash.Currency, "missing_input")
		}
		if o.Currency != cash.Currency {
			return gapLine(name, cash.Currency, "unsupported_currency")
		}
		left.Sub(left, n)
	}
	return line(name, cash.Currency, left, in.UsdJpy)
}

// stress is the figure household.stress: the cash left after all short puts are assigned, then
// one row per constant step; the steps need an underlying price that the input does not carry,
// so they are the gap missing_quote.
func stress(after blockapi.MoneyLine) []blockapi.MoneyLine {
	after.Name = "全部指派后现金"
	rows := []blockapi.MoneyLine{after}
	for _, s := range stressSteps {
		rows = append(rows, gapLine("标的跌 "+s+"%", after.Currency, "missing_quote"))
	}
	return rows
}

// obligations is the figure household.obligations: one row per short put, its gross put
// notional (strike x multiplier x contracts), the premium in the note.
func obligations(in blockapi.HouseholdInput) []blockapi.MoneyLine {
	var rows []blockapi.MoneyLine
	for _, o := range in.Obligations {
		name := o.Underlying + " 卖出认沽"
		n, ok := notional(o)
		if !ok {
			rows = append(rows, gapLine(name, o.Currency, "missing_input"))
			continue
		}
		l := line(name, o.Currency, n, in.UsdJpy)
		if o.PremiumIncome != "" {
			l.Note = strings.TrimSpace(l.Note + " 权利金 " + o.PremiumIncome)
		}
		rows = append(rows, l)
	}
	return rows
}

// premium sums PremiumIncome per currency, in order of first appearance.
func premium(obs []blockapi.ObligationIn) (currencies []string, sums map[string]*big.Rat, bad bool) {
	sums = map[string]*big.Rat{}
	for _, o := range obs {
		p, ok := parse(o.PremiumIncome)
		if !ok {
			bad = true
			continue
		}
		if _, seen := sums[o.Currency]; !seen {
			sums[o.Currency] = new(big.Rat)
			currencies = append(currencies, o.Currency)
		}
		sums[o.Currency].Add(sums[o.Currency], p)
	}
	return currencies, sums, bad
}

// income is the figure household.income: the premium income of the month, one row per currency.
// A JPY row keeps its yen, with the rate only for the display-only ≈USD.
func income(obs []blockapi.ObligationIn) []blockapi.MoneyLine {
	cur, sums, _ := premium(obs)
	var rows []blockapi.MoneyLine
	for _, c := range cur {
		rows = append(rows, blockapi.MoneyLine{Name: "本月权利金", Currency: c, Amount: compute.FigureValue{Value: money(sums[c])}})
	}
	return rows
}

// monthChange is the figure household.month_change: the waterfall of net liquidation from the
// month start: premium, the trading result (needs marks: missing_quote) and the rest as 其他.
func monthChange(in blockapi.HouseholdInput, end blockapi.ValueIn, endR *big.Rat, endOK bool) []blockapi.MoneyLine {
	const name = "月度资产变化"
	start, startR, startOK := tagValue(in.MonthStart, "NetLiquidation")
	if !startOK || !endOK {
		return []blockapi.MoneyLine{gapLine(name, end.Currency, "missing_input")}
	}
	if start.Currency != end.Currency {
		return []blockapi.MoneyLine{gapLine(name, end.Currency, "unsupported_currency")}
	}
	c := end.Currency
	rows := []blockapi.MoneyLine{line("月初", c, startR, in.UsdJpy)}
	cur, sums, bad := premium(in.Obligations)
	prem := gapLine("已平仓期权结果", c, "no_data")
	rest := gapLine("其他", c, "no_data")
	if p, ok := sums[c]; ok && !bad && len(cur) == 1 {
		prem = line("已平仓期权结果", c, p, in.UsdJpy)
		change := new(big.Rat).Sub(endR, startR)
		rest = line("其他", c, change.Sub(change, p), in.UsdJpy)
	}
	rows = append(rows, prem, gapLine("交易结果", c, "missing_quote"), rest, line("月末", c, endR, in.UsdJpy))
	return rows
}

// trend is the figure household.trend: the net liquidation of the last 90 session days, oldest first.
func trend(h []blockapi.DayValue) []blockapi.DayValue {
	out := slices.Clone(h)
	slices.SortStableFunc(out, func(a, b blockapi.DayValue) int { return strings.Compare(string(a.Day), string(b.Day)) })
	if len(out) > trendDays {
		out = out[len(out)-trendDays:]
	}
	return out
}
