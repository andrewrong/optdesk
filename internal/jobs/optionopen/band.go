package optionopen

import (
	"math"
	"sort"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// The stored band of a stock: which contracts of a chain the resolve jobs ask for.

// candidates lists the contracts of the stored band of the stock, the held contracts first and then by distance
// from the spot, so that a cap drops the far strikes first.
func (r *keyRun) candidates(chains []broker.Chain, spot, sigma float64, held bool) []candidate {
	seen := map[string]bool{}
	var out []candidate
	add := func(c candidate) {
		if !seen[c.key()] {
			seen[c.key()] = true
			out = append(out, c)
		}
	}
	mk := func(ch broker.Chain, exp, strike, right string, isHeld bool) candidate {
		req := broker.Contract{SecType: "OPT", Symbol: r.st.Symbol, Exchange: ch.Exchange, Currency: r.st.Currency, Expiry: exp,
			Strike: strike, Right: right, Multiplier: ch.Multiplier, TradingClass: ch.TradingClass}
		d, _ := time.Parse("20060102", exp)
		return candidate{req: req, expiry: contracts.Day(d.Format(time.DateOnly)), dist: math.Abs(strikeValue(strike) - spot), held: isHeld}
	}
	r.heldCandidates(chains, mk, add)
	p := r.j.m.params
	for _, ch := range chains {
		strikes := sortedStrikes(ch.Strikes)
		for _, exp := range ch.Expirations {
			d, err := time.Parse("20060102", exp)
			if err != nil {
				continue
			}
			dte := int(d.Sub(r.t).Hours() / 24)
			if dte < p.MinDTE || dte > p.MaxDTE+r.j.m.LookaheadDays {
				continue
			}
			lo, hi := StoredBand(p, spot, sigma, max(dte, 1))
			for _, c := range bandOf(strikes, spot, lo, hi, p.MinPuts, held || p.Calls == CallsAll) {
				add(mk(ch, exp, c.strike, c.right, false))
			}
		}
	}
	sort.SliceStable(out, func(i, k int) bool {
		if out[i].held != out[k].held {
			return out[i].held
		}
		return out[i].dist < out[k].dist
	})
	return out
}

// heldCandidates adds the open option positions of the stock, whatever their expiry, on the chain that lists
// their expiry.
func (r *keyRun) heldCandidates(chains []broker.Chain, mk func(broker.Chain, string, string, string, bool) candidate, add func(candidate)) {
	for _, pos := range r.pos {
		if pos.SecType != "OPT" || pos.Symbol != r.st.Symbol || !nonzero(pos.Quantity) {
			continue
		}
		right := "P"
		if strings.EqualFold(pos.Right, "call") || strings.EqualFold(pos.Right, "c") {
			right = "C"
		}
		exp := strings.ReplaceAll(pos.Expiry, "-", "")
		for _, ch := range chains {
			if containsString(ch.Expirations, exp) && (pos.Multiplier == "" || strikeKey(pos.Multiplier) == strikeKey(ch.Multiplier)) {
				add(mk(ch, exp, pos.Strike, right, true))
				break
			}
		}
	}
}

type strikeRight struct{ strike, right string }

// bandOf is the contracts of one expiry inside the stored band: the puts from lo up to the strike above the spot
// (and at least minPuts puts at or below the spot), the calls around the spot, and with calls the calls up to hi.
func bandOf(strikes []strikeText, spot, lo, hi float64, minPuts int, calls bool) []strikeRight {
	below, above := aroundSpot(strikes, spot)
	need := minPuts
	for _, s := range strikes {
		if s.v <= spot && s.v >= lo {
			need--
		}
	}
	var out []strikeRight
	for i := len(strikes) - 1; i >= 0; i-- {
		s := strikes[i]
		switch {
		case s.v <= spot && s.v >= lo, s.v == above:
			out = append(out, strikeRight{s.text, "P"})
		case s.v <= spot && need > 0:
			need--
			out = append(out, strikeRight{s.text, "P"})
		}
	}
	for _, s := range strikes {
		if s.v == below || s.v == above || (calls && s.v >= below && s.v <= hi) {
			out = append(out, strikeRight{s.text, "C"})
		}
	}
	return out
}

type strikeText struct {
	text string
	v    float64
}

func sortedStrikes(in []string) []strikeText {
	out := make([]strikeText, 0, len(in))
	for _, s := range in {
		if v := strikeValue(s); v > 0 {
			out = append(out, strikeText{s, v})
		}
	}
	sort.Slice(out, func(i, k int) bool { return out[i].v < out[k].v })
	return out
}

// aroundSpot returns the strike at or below the spot and the one above it; a side with none is 0 and +Inf.
func aroundSpot(strikes []strikeText, spot float64) (below, above float64) {
	above = math.Inf(1)
	for _, s := range strikes {
		if s.v <= spot {
			below = s.v
		} else if s.v < above {
			above = s.v
		}
	}
	return below, above
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
