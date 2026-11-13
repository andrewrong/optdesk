package pagefig

import (
	"math/big"
	"sort"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// moversLimit is the most lines one table holds; the rest stays in Other.
const moversLimit = 60

// mover is one built line with its exact rounded amount (nil for a gap) for the sort and the sum.
type mover struct {
	line blockapi.MoverLine
	amt  *big.Rat
}

// buildMovers builds the two tables, the total and Other. Pure: no I/O, no clock.
func buildMovers(in blockapi.MoversIn) blockapi.MoversResult {
	var stocks, options []mover
	for _, r := range in.Rows {
		m := moverLine(r, in.UsdJpy)
		if r.SecType == "option" {
			options = append(options, m)
		} else {
			stocks = append(stocks, m)
		}
	}
	sortMovers(stocks)
	sortMovers(options)
	listed := new(big.Rat)
	out := blockapi.MoversResult{}
	out.Stocks, out.Options = cut(stocks, listed), cut(options, listed)
	out.Total, out.Other = moversTotal(in, listed)
	return out
}

// cut keeps the first moversLimit lines and adds their amounts to listed.
func cut(ms []mover, listed *big.Rat) []blockapi.MoverLine {
	if len(ms) > moversLimit {
		ms = ms[:moversLimit]
	}
	lines := make([]blockapi.MoverLine, 0, len(ms))
	for _, m := range ms {
		lines = append(lines, m.line)
		if m.amt != nil {
			listed.Add(listed, m.amt)
		}
	}
	return lines
}

// sortMovers orders by absolute amount, largest first, ties by Name, a gap last.
func sortMovers(ms []mover) {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if (a.amt == nil) != (b.amt == nil) {
			return a.amt != nil
		}
		if a.amt != nil {
			if c := new(big.Rat).Abs(a.amt).Cmp(new(big.Rat).Abs(b.amt)); c != 0 {
				return c > 0
			}
		}
		return a.line.Name < b.line.Name
	})
}

func moverLine(r blockapi.MoverIn, usdJpy string) mover {
	l := blockapi.MoverLine{Name: r.Name, SecType: r.SecType, Currency: r.Currency, Note: moverNote(r)}
	p0, ok0 := parse(r.PricePrev)
	p1, ok1 := parse(r.PriceLast)
	if !ok0 || !ok1 || r.BasisPrev == "none" || r.BasisLast == "none" || r.BasisPrev != r.BasisLast {
		l.PricePrev, l.PriceLast, l.Amount = gap("missing_quote"), gap("missing_quote"), gap("missing_quote")
		return mover{line: l}
	}
	l.PricePrev = compute.FigureValue{Value: decimals(p0, 4)}
	l.PriceLast = compute.FigureValue{Value: decimals(p1, 4)}
	amt, g := moverAmount(r, p0, p1, usdJpy)
	if g != "" {
		l.Amount = gap(g)
		return mover{line: l}
	}
	rounded, _ := new(big.Rat).SetString(money(amt))
	l.Amount = compute.FigureValue{Value: money(amt)}
	return mover{line: l, amt: rounded}
}

// moverAmount is q_prev x multiplier x (p1 - p0) in USD, or the gap that stops it.
func moverAmount(r blockapi.MoverIn, p0, p1 *big.Rat, usdJpy string) (*big.Rat, string) {
	if r.SecType != "stock" && r.SecType != "option" {
		return nil, "missing_input"
	}
	q, ok := parse(r.Quantity)
	if !ok {
		return nil, "missing_input"
	}
	mult := big.NewRat(1, 1)
	if strings.TrimSpace(r.Multiplier) != "" || r.SecType == "option" {
		if mult, ok = parse(r.Multiplier); !ok {
			return nil, "missing_input"
		}
	}
	amt := new(big.Rat).Mul(q, mult)
	amt.Mul(amt, new(big.Rat).Sub(p1, p0))
	usd, g := toUSD(amt, r.Currency, usdJpy)
	return usd, g
}

// toUSD converts v from cur: USD as is, JPY divided by the rate; the gap otherwise.
func toUSD(v *big.Rat, cur, usdJpy string) (*big.Rat, string) {
	switch cur {
	case "USD":
		return v, ""
	case "JPY":
		rt := rate(usdJpy)
		if rt == nil {
			return nil, "missing_input"
		}
		return new(big.Rat).Quo(v, rt), ""
	}
	return nil, "unsupported_currency"
}

// moversTotal is NetLast - NetPrev in USD and Other = Total less the listed amounts, both on the rounded cents.
func moversTotal(in blockapi.MoversIn, listed *big.Rat) (total, other compute.FigureValue) {
	g := ""
	vals := [2]*big.Rat{}
	for i, n := range []blockapi.ValueIn{in.NetPrev, in.NetLast} {
		v, ok := parse(n.Value)
		switch {
		case !ok:
			g = firstNetGap(g, "missing_input")
		case n.Currency != "USD" && n.Currency != "JPY":
			g = firstNetGap(g, "unsupported_currency")
		default:
			u, ug := toUSD(v, n.Currency, in.UsdJpy)
			vals[i] = u
			g = firstNetGap(g, ug)
		}
	}
	if g != "" {
		return gap(g), gap(g)
	}
	t, _ := new(big.Rat).SetString(money(new(big.Rat).Sub(vals[1], vals[0])))
	return compute.FigureValue{Value: money(t)}, compute.FigureValue{Value: money(t.Sub(t, listed))}
}

// firstNetGap keeps the first gap; the order is prev then last.
func firstNetGap(have, g string) string {
	if have != "" {
		return have
	}
	return g
}

// moverNote is the position in words: "100 sh" or "short 2 put 240 11-20".
func moverNote(r blockapi.MoverIn) string {
	q, ok := parse(r.Quantity)
	if !ok {
		return strings.TrimSpace(r.Quantity)
	}
	if r.SecType != "option" {
		return plain(q) + " sh"
	}
	side := "long"
	if q.Sign() < 0 {
		side = "short"
	}
	parts := []string{side, plain(new(big.Rat).Abs(q))}
	if r.Right != "" {
		parts = append(parts, r.Right)
	}
	if s, ok := parse(r.Strike); ok {
		parts = append(parts, plain(s))
	}
	if e := monthDay(r.Expiry); e != "" {
		parts = append(parts, e)
	}
	return strings.Join(parts, " ")
}

// monthDay turns 2026-11-20 or 20261120 into 11-20; other text gives "".
func monthDay(s string) string {
	d := strings.ReplaceAll(strings.TrimSpace(s), "-", "")
	if len(d) != 8 {
		return ""
	}
	return d[4:6] + "-" + d[6:]
}

// plain is r in decimal text without trailing zeros.
func plain(r *big.Rat) string {
	s := r.FloatString(8)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// decimals formats r to places decimals, half-even.
func decimals(r *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))
	q, rem := new(big.Int).QuoRem(scaled.Num(), scaled.Denom(), new(big.Int))
	twice := new(big.Int).Abs(rem)
	twice.Lsh(twice, 1)
	if c := twice.Cmp(scaled.Denom()); c > 0 || (c == 0 && q.Bit(0) == 1) {
		if scaled.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		} else {
			q.Add(q, big.NewInt(1))
		}
	}
	return new(big.Rat).SetFrac(q, scale).FloatString(places)
}
