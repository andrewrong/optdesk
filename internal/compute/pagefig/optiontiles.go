package pagefig

import (
	"math/big"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

// optionTiles is the pure implementation of blockapi.OptionTiles.
type optionTiles struct{}

var _ blockapi.OptionTiles = optionTiles{}

// NewOptionTiles constructs the option tile figures (own implementation of the sums: they need exact
// big.Rat arithmetic and the two roundings of Decision 7, which no library gives).
func NewOptionTiles() blockapi.OptionTiles { return optionTiles{} }

const usd = "USD"

// Tiles evaluates the sold count, the open premium and the max loss.
func (optionTiles) Tiles(in blockapi.OptionTilesIn) blockapi.OptionTilesOut {
	out := blockapi.OptionTilesOut{SoldCount: soldCount(in), OpenCount: len(in.Structures)}
	premium, loss := new(big.Rat), new(big.Rat)
	numbered := 0
	for _, s := range in.Structures {
		if s.Currency != usd {
			out.OtherCurrency++
			continue
		}
		p, short, ok := legPremium(s.Legs)
		if !ok {
			out.Unreadable++
			continue
		}
		if short && s.Kind != "long_put" && s.Kind != "long_call" {
			premium.Add(premium, p)
		}
		if s.Kind == "naked_call" {
			out.Uncovered++
			out.MaxLossGap = "no_data"
		} else if v, ok := parse(s.Figures.MaxLoss.Value); ok && s.Figures.MaxLoss.Gap == "" {
			loss.Add(loss, v)
			numbered++
		} else {
			out.Unnumbered++
		}
	}
	if out.OpenCount > 0 && out.OtherCurrency == out.OpenCount {
		out.OpenPremiumGap, out.MaxLossGap = "unsupported_currency", "unsupported_currency"
		return out
	}
	if premium.Sign() < 0 {
		out.OpenPremiumGap = "no_data"
	} else {
		out.OpenPremium = usdText(premium)
	}
	if out.Uncovered == 0 || numbered > 0 {
		out.MaxLossUsd = usdText(new(big.Rat).Abs(loss))
	}
	return out
}

// soldCount counts the distinct lots (PositionID before its last colon) sold in the month of the day.
func soldCount(in blockapi.OptionTilesIn) int {
	day := string(in.Day)
	if len(day) < 7 {
		return 0
	}
	lots := map[string]struct{}{}
	for _, p := range in.Positions {
		opened := string(p.OpenedOn)
		if len(opened) < 7 || opened[:7] != day[:7] || opened > day {
			continue
		}
		if q, ok := parse(p.Quantity); !ok || q.Sign() >= 0 {
			continue
		}
		id := p.PositionID
		if i := strings.LastIndex(id, ":"); i >= 0 {
			id = id[:i]
		}
		lots[id] = struct{}{}
	}
	return len(lots)
}

// legPremium sums -Quantity x OpenPrice x Multiplier; short reports a leg of negative quantity; ok is
// false when a leg does not parse.
func legPremium(legs []compute.StructureLeg) (sum *big.Rat, short, ok bool) {
	sum = new(big.Rat)
	for _, l := range legs {
		q, okq := parse(l.Quantity)
		pr, okp := parse(l.OpenPrice)
		m, okm := parse(l.Multiplier)
		if !okq || !okp || !okm {
			return nil, false, false
		}
		if q.Sign() < 0 {
			short = true
		}
		t := new(big.Rat).Mul(q, pr)
		t.Mul(t, m)
		sum.Sub(sum, t)
	}
	return sum, short, true
}

// usdText rounds one non-negative exact sum: cents half away from zero, dollars half up.
func usdText(r *big.Rat) blockapi.UsdText {
	return blockapi.UsdText{Value: fixed(roundHalf(new(big.Rat).Mul(r, big.NewRat(100, 1))), 2), Dollars: roundHalf(r).String()}
}

// roundHalf rounds a non-negative rational half up (which is half away from zero for it).
func roundHalf(r *big.Rat) *big.Int {
	n := new(big.Int).Lsh(r.Num(), 1)
	n.Add(n, r.Denom())
	return n.Div(n, new(big.Int).Lsh(r.Denom(), 1))
}

// fixed writes an integer count of 10^-places as decimal text.
func fixed(n *big.Int, places int) string {
	s := n.String()
	for len(s) <= places {
		s = "0" + s
	}
	return s[:len(s)-places] + "." + s[len(s)-places:]
}
