// Package spreadlegs implements spreadlegsapi.SpreadFigure: the net credit and all break-even
// prices of each structure, with the day count and the display order. Exact decimals
// (math/big), no I/O, no clock, no call into optionstruct.
package spreadlegs

import (
	"cmp"
	"math/big"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/spreadlegsapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	gapMissing     = "missing_input"
	gapUnsupported = "unsupported_currency"
	gapNoRule      = "no_rule"
)

type figure struct{}

// New returns the figure.
func New() spreadlegsapi.SpreadFigure { return figure{} }

// Spreads gives one row per structure in display order.
func (figure) Spreads(in spreadlegsapi.SpreadIn) spreadlegsapi.SpreadResult {
	type keyed struct {
		row        spreadlegsapi.SpreadFigures
		underlying string
	}
	rows := make([]keyed, 0, len(in.Structures))
	for _, s := range in.Structures {
		exp, days := expiry(s, in.AsOf)
		r := spreadlegsapi.SpreadFigures{ID: s.ID, Expiry: exp, DaysToExpiry: days, NetCredit: netCredit(s)}
		r.BreakEvens, r.BreakEvenGap = breakEvens(s)
		rows = append(rows, keyed{r, s.Underlying})
	}
	slices.SortStableFunc(rows, func(a, b keyed) int {
		return cmp.Or(cmp.Compare(a.row.DaysToExpiry, b.row.DaysToExpiry),
			cmp.Compare(a.underlying, b.underlying), cmp.Compare(a.row.ID, b.row.ID))
	})
	out := spreadlegsapi.SpreadResult{Rows: make([]spreadlegsapi.SpreadFigures, len(rows))}
	for i, k := range rows {
		out.Rows[i] = k.row
	}
	return out
}

// leg is a parsed leg.
type leg struct {
	put                     bool
	strike, qty, mult, open *big.Rat
}

func parseDec(s string) (*big.Rat, bool) {
	body := strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	if body == "" || body == "." || strings.Count(body, ".") > 1 {
		return nil, false
	}
	for _, c := range body {
		if (c < '0' || c > '9') && c != '.' {
			return nil, false
		}
	}
	return new(big.Rat).SetString(s)
}

func parseLeg(l compute.StructureLeg) (leg, bool) {
	var p leg
	var ok [4]bool
	p.strike, ok[0] = parseDec(l.Strike)
	p.qty, ok[1] = parseDec(l.Quantity)
	p.mult, ok[2] = parseDec(l.Multiplier)
	p.open, ok[3] = parseDec(l.OpenPrice)
	switch l.Right {
	case "P":
		p.put = true
	case "C":
	default:
		return p, false
	}
	return p, !slices.Contains(ok[:], false)
}

func netCredit(s compute.Structure) compute.FigureValue {
	if s.Currency != "USD" {
		return compute.FigureValue{Gap: gapUnsupported}
	}
	if len(s.Legs) == 0 {
		return compute.FigureValue{Gap: gapMissing}
	}
	total := new(big.Rat)
	for _, l := range s.Legs {
		q, ok1 := parseDec(l.Quantity)
		p, ok2 := parseDec(l.OpenPrice)
		m, ok3 := parseDec(l.Multiplier)
		if !ok1 || !ok2 || !ok3 {
			return compute.FigureValue{Gap: gapMissing}
		}
		term := new(big.Rat).Mul(new(big.Rat).Mul(q, p), m)
		total.Sub(total, term)
	}
	return compute.FigureValue{Value: fixed(total, 2)}
}

func breakEvens(s compute.Structure) ([]string, string) {
	if s.Currency != "USD" {
		return nil, gapUnsupported
	}
	if s.Kind == "other" || s.Kind == "collar" || s.Kind == "covered_call" || len(s.Legs) < 2 ||
		slices.ContainsFunc(s.Legs, func(l compute.StructureLeg) bool { return l.Expiry != s.Legs[0].Expiry }) {
		return nil, gapNoRule
	}
	legs := make([]leg, 0, len(s.Legs))
	for _, l := range s.Legs {
		p, ok := parseLeg(l)
		if !ok {
			return nil, gapMissing
		}
		legs = append(legs, p)
	}
	zeros, ok := zeroCrossings(legs)
	if !ok || len(zeros) == 0 {
		return nil, gapNoRule
	}
	out := make([]string, len(zeros))
	for i, z := range zeros {
		out[i] = fixed(z, 4)
	}
	return out, ""
}

// pnl is the expiry payoff of the legs at price px.
func pnl(legs []leg, px *big.Rat) *big.Rat {
	total := new(big.Rat)
	for _, l := range legs {
		in := new(big.Rat).Sub(px, l.strike)
		if l.put {
			in.Neg(in)
		}
		if in.Sign() < 0 {
			in.SetInt64(0)
		}
		in.Sub(in, l.open)
		total.Add(total, in.Mul(in, new(big.Rat).Mul(l.qty, l.mult)))
	}
	return total
}

// zeroCrossings lists the prices (>= 0) where the piecewise linear payoff is zero, ascending.
// The corners are price 0 and the strikes; above the top strike the slope is the signed call
// quantity. ok is false when the payoff is zero over an interval.
func zeroCrossings(legs []leg) ([]*big.Rat, bool) {
	prices := []*big.Rat{new(big.Rat)}
	slope := new(big.Rat)
	for _, l := range legs {
		prices = append(prices, l.strike)
		if !l.put {
			slope.Add(slope, new(big.Rat).Mul(l.qty, l.mult))
		}
	}
	slices.SortFunc(prices, func(a, b *big.Rat) int { return a.Cmp(b) })
	prices = slices.CompactFunc(prices, func(a, b *big.Rat) bool { return a.Cmp(b) == 0 })
	vals := make([]*big.Rat, len(prices))
	for i, p := range prices {
		vals[i] = pnl(legs, p)
	}
	var out []*big.Rat
	for i, v := range vals {
		if v.Sign() == 0 {
			if i+1 < len(vals) && vals[i+1].Sign() == 0 {
				return nil, false
			}
			out = append(out, prices[i])
		}
		if i+1 < len(vals) && v.Sign()*vals[i+1].Sign() < 0 {
			t := new(big.Rat).Quo(v, new(big.Rat).Sub(v, vals[i+1]))
			out = append(out, new(big.Rat).Add(prices[i],
				t.Mul(t, new(big.Rat).Sub(prices[i+1], prices[i]))))
		}
	}
	last := len(vals) - 1
	switch {
	case slope.Sign() == 0 && vals[last].Sign() == 0:
		return nil, false
	case slope.Sign()*vals[last].Sign() < 0:
		out = append(out, new(big.Rat).Sub(prices[last], new(big.Rat).Quo(vals[last], slope)))
	}
	return out, true
}

// expiry is the shortest expiry of the legs and its day count from asOf (not below 0).
func expiry(s compute.Structure, asOf contracts.Day) (contracts.Day, int) {
	best, bestN := "", 0
	for _, l := range s.Legs {
		n, ok := dayNumber(l.Expiry)
		if ok && (best == "" || n < bestN) {
			best, bestN = l.Expiry, n
		}
	}
	if best == "" {
		return "", 0
	}
	days := 0
	if a, ok := dayNumber(string(asOf)); ok && bestN > a {
		days = bestN - a
	}
	return contracts.Day(best), days
}

// dayNumber is the day count of a YYYY-MM-DD text since 1970-01-01 (civil calendar algorithm).
func dayNumber(s string) (int, bool) {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return 0, false
	}
	num := func(t string) (int, bool) {
		n := 0
		for _, c := range t {
			if c < '0' || c > '9' {
				return 0, false
			}
			n = n*10 + int(c-'0')
		}
		return n, true
	}
	y, ok1 := num(s[:4])
	m, ok2 := num(s[5:7])
	d, ok3 := num(s[8:])
	if !ok1 || !ok2 || !ok3 || m < 1 || m > 12 || d < 1 || d > 31 {
		return 0, false
	}
	if m <= 2 {
		y--
	}
	era := y / 400
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468, true
}

// fixed writes r with the given places, rounding half to even.
func fixed(r *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	abs := new(big.Rat).Abs(r)
	q, rem := new(big.Int).QuoRem(new(big.Int).Mul(abs.Num(), scale), abs.Denom(), new(big.Int))
	if c := rem.Lsh(rem, 1).Cmp(abs.Denom()); c > 0 || c == 0 && q.Bit(0) == 1 {
		q.Add(q, big.NewInt(1))
	}
	digits := q.String()
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	out := digits[:len(digits)-places] + "." + digits[len(digits)-places:]
	if r.Sign() < 0 && q.Sign() != 0 {
		out = "-" + out
	}
	return out
}
