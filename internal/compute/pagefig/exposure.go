package pagefig

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// maxNamed is the number of symbols a note or gap text names before it ends "and <M> more", so that the
// text stays within the 200 characters the page schema allows.
const maxNamed = 5

// kindSum is the sum of one kind of position (stock or option) and the reasons it is not complete.
type kindSum struct {
	sum        *big.Rat
	unmarked   []string
	noRate     bool
	otherPair  bool
	unreadable bool
}

// symbol names a position: the underlying, with the type for an option.
func symbol(p blockapi.PositionIn) string { return p.Underlying }

// namedList joins at most maxNamed symbols and ends "and <M> more" when more are left.
func namedList(names []string) string {
	if len(names) <= maxNamed {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:maxNamed], ", ") + " and " + strconv.Itoa(len(names)-maxNamed) + " more"
}

// add puts one position into the sum of its kind, in the currency of the net value.
func (k *kindSum) add(p blockapi.PositionIn, netCurrency string, rt *big.Rat) {
	var conv *big.Rat // divisor of the position's amount
	switch {
	case p.Currency == netCurrency:
	case p.Currency == "JPY" && netCurrency == "USD":
		if rt == nil {
			k.noRate = true
			return
		}
		conv = rt
	default:
		k.otherPair = true
		return
	}
	if p.Mark == "" {
		k.unmarked = append(k.unmarked, symbol(p))
		return
	}
	q, ok1 := parse(p.Quantity)
	m, ok2 := parse(p.Multiplier)
	mark, ok3 := parse(p.Mark)
	if !ok1 || !ok2 || !ok3 {
		k.unreadable = true
		return
	}
	v := new(big.Rat).Mul(new(big.Rat).Mul(q, m), mark)
	if conv != nil {
		v.Quo(v, conv)
	}
	k.sum.Add(k.sum, v)
}

// result is the cell of the kind: the sum, or the gap that stops it with the note that names why.
func (k *kindSum) result(name, currency, usdJpy string) blockapi.MoneyLine {
	switch {
	case k.unreadable:
		return gapLine(name, currency, "missing_input")
	case k.otherPair:
		return gapLine(name, currency, "unsupported_currency")
	case k.noRate:
		l := gapLine(name, currency, "unsupported_currency")
		l.Note = "no USDJPY rate"
		return l
	case len(k.unmarked) > 0:
		l := gapLine(name, currency, "missing_quote")
		l.Note = "no mark: " + namedList(k.unmarked)
		return l
	}
	return line(name, currency, k.sum, usdJpy)
}

// exposure is the Stock value and Option value cells of household.accounts, summed from the
// positions with big-number arithmetic, never a float. A position is never skipped: any
// that cannot be valued makes its cell a gap.
func exposure(currency, usdJpy string, ps []blockapi.PositionIn) (stock, option blockapi.MoneyLine) {
	rt := rate(usdJpy)
	s, o := &kindSum{sum: new(big.Rat)}, &kindSum{sum: new(big.Rat)}
	var other []string
	for _, p := range ps {
		switch p.SecType {
		case "stock":
			s.add(p, currency, rt)
		case "option":
			o.add(p, currency, rt)
		default:
			other = append(other, symbol(p))
		}
	}
	if len(other) > 0 {
		note := "unsupported type: " + namedList(other)
		stock, option = gapLine("Stock value", currency, "missing_input"), gapLine("Option value", currency, "missing_input")
		stock.Note, option.Note = note, note
		return stock, option
	}
	return s.result("Stock value", currency, usdJpy), o.result("Option value", currency, usdJpy)
}
