// Package stress revalues the open positions of one book at the FINRA portfolio-margin moves: per class and
// currency the eleven points of ten equal intervals, the worst point, and a total only when nothing of the
// currency was left out. Every sum is a math/big Rat; the pricer (package bsm) is injected.
package stress

import (
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy"
)

// MaxSessionsOld is how many sessions before the run day a mark or spot may be and still be used; it is the
// value risk_page uses for max_mark_age_sessions [O].
const MaxSessionsOld = 5

const baseCurrency = "USD"

const dayLayout = "2006-01-02"

// optionIn is the option part of a pricer call that does not change along the ladder.
type optionIn struct{ right, spot, strike, years, rate string }

type stresser struct{ p Pricer }

// New returns the stresser that reprices options with p.
func New(p Pricer) strategy.RiskStresser { return stresser{p: p} }

type groupKey struct {
	currency string
	class    strategy.RiskClass
}

type group struct {
	low, high *big.Rat
	pnl, cash []*big.Rat
	oldest    contracts.Day
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

func cents(r *big.Rat) string { return r.FloatString(2) }

// moveText writes a fraction exactly, without trailing zeros.
func moveText(r *big.Rat) string {
	s := r.FloatString(6)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func (s stresser) Stress(b strategy.RiskBook) strategy.RiskOutcome {
	out := strategy.RiskOutcome{Alias: b.Alias, NotStressed: map[string]int{}}
	groups := map[groupKey]*group{}
	incomplete := map[string]bool{}

	hasShares := false
	for _, h := range b.Holdings {
		if h.Option == nil {
			hasShares = true
		}
	}
	if !b.HasCapture && !hasShares {
		out.Excluded = append(out.Excluded, strategy.RiskExcluded{Currency: b.Currency, Reason: "no_capture"})
		incomplete[b.Currency] = true
	}

	for _, h := range b.Holdings {
		class := h.Class
		if h.Currency != baseCurrency {
			if class == "" || h.Mark == "" {
				out.NotStressed[h.Currency]++
				incomplete[h.Currency] = true
				continue
			}
			class = "equity" // outside the rule's scope: stressed at the equity range [T]
		}
		low, high, ok := bounds(class)
		reason := ""
		var pnl, cash []*big.Rat
		if !ok {
			reason = "no_class"
		} else {
			pnl, cash, reason = s.holding(b, h, moves(low, high))
		}
		if reason != "" {
			out.Excluded = append(out.Excluded, strategy.RiskExcluded{Underlying: h.Underlying, Currency: h.Currency, Reason: reason})
			incomplete[h.Currency] = true
			continue
		}
		k := groupKey{h.Currency, class}
		g := groups[k]
		if g == nil {
			g = &group{low: low, high: high, oldest: h.SpotDay, pnl: zeros(), cash: zeros()}
			groups[k] = g
		}
		for i := range pnl {
			g.pnl[i].Add(g.pnl[i], pnl[i])
			g.cash[i].Add(g.cash[i], cash[i])
		}
		if h.SpotDay < g.oldest {
			g.oldest = h.SpotDay
		}
	}
	if len(out.NotStressed) == 0 {
		out.NotStressed = nil
	}

	summarise(&out, groups, incomplete)
	return out
}

// summarise turns the summed groups into the class results and the complete totals.
func summarise(out *strategy.RiskOutcome, groups map[groupKey]*group, incomplete map[string]bool) {
	keys := make([]groupKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].currency != keys[j].currency {
			return keys[i].currency < keys[j].currency
		}
		return keys[i].class < keys[j].class
	})
	sums := map[string]*big.Rat{}
	var order []string
	for _, k := range keys {
		g := groups[k]
		mv := moves(g.low, g.high)
		res := strategy.RiskClassResult{Class: k.class, Currency: k.currency, OldestSpotDay: g.oldest}
		worst := 0
		for i := range mv {
			res.Ladder = append(res.Ladder, strategy.RiskPoint{Move: moveText(mv[i]), PnL: cents(g.pnl[i]), AssignmentCash: cents(g.cash[i])})
			if g.pnl[i].Cmp(g.pnl[worst]) < 0 {
				worst = i
			}
		}
		res.Worst = res.Ladder[worst]
		out.Classes = append(out.Classes, res)
		if sums[k.currency] == nil {
			sums[k.currency] = new(big.Rat)
			order = append(order, k.currency)
		}
		sums[k.currency].Add(sums[k.currency], g.pnl[worst])
	}
	for _, cur := range order {
		if !incomplete[cur] {
			out.Totals = append(out.Totals, strategy.RiskTotal{Currency: cur, WorstLoss: cents(new(big.Rat).Neg(sums[cur]))})
		}
	}
}

func zeros() []*big.Rat {
	z := make([]*big.Rat, intervals+1)
	for i := range z {
		z[i] = new(big.Rat)
	}
	return z
}

// holding is the profit and loss and the assignment cash of one holding at each move, or the reason it is
// left out.
func (s stresser) holding(b strategy.RiskBook, h strategy.RiskHolding, mv []*big.Rat) (pnl, cash []*big.Rat, reason string) {
	qty, okq := parseDec(h.Quantity)
	mult, okm := parseDec(h.Multiplier)
	spot, oks := parseDec(h.Spot)
	if !okq || !okm || !oks || spot.Sign() <= 0 {
		return nil, nil, "no_vol"
	}
	pnl, cash = zeros(), zeros()
	if h.Option == nil {
		if h.Mark != "" && (!h.MarkUsable || h.MarkDay != h.SpotDay) || stale(b, h.SpotDay) {
			return nil, nil, "stale_mark"
		}
		for i, m := range mv {
			d := new(big.Rat).Mul(spot, m)
			pnl[i] = d.Mul(d, new(big.Rat).Mul(qty, mult))
		}
		return pnl, cash, ""
	}
	return s.option(b, h, qty, mult, spot, mv, pnl, cash)
}

// option is the ladder of one option leg.
func (s stresser) option(b strategy.RiskBook, h strategy.RiskHolding, qty, mult, spot *big.Rat, mv, pnl, cash []*big.Rat) ([]*big.Rat, []*big.Rat, string) {
	o := h.Option
	days, reason := optionGate(b, h)
	if reason != "" {
		return nil, nil, reason
	}
	mark, okk := parseDec(h.Mark)
	strike, oks2 := parseDec(o.Strike)
	right := strings.ToUpper(strings.TrimSpace(o.Right))
	isPut := right == "P" || right == "PUT"
	if !okk || !oks2 || (!isPut && right != "C" && right != "CALL") {
		return nil, nil, "no_vol"
	}
	years := new(big.Rat).SetFrac64(days, 365).FloatString(12)
	in := optionIn{right: right[:1], spot: h.Spot, strike: o.Strike, years: years, rate: b.Rate}
	intrinsic := func(sp *big.Rat) *big.Rat {
		v := new(big.Rat).Sub(strike, sp)
		if !isPut {
			v.Neg(v)
		}
		if v.Sign() < 0 {
			return new(big.Rat)
		}
		return v
	}
	vol := ""
	if mark.Cmp(intrinsic(spot)) > 0 {
		var err error
		vol, err = s.p.Implied(in.right, in.spot, in.strike, in.years, in.rate, h.Mark)
		if err != nil {
			return nil, nil, "no_vol"
		}
	}
	size := new(big.Rat).Mul(qty, mult)
	for i, m := range mv {
		moved := new(big.Rat).Mul(spot, new(big.Rat).Add(big.NewRat(1, 1), m))
		val := intrinsic(moved)
		if vol != "" {
			var ok bool
			if val, ok = s.repriced(in, moved, vol); !ok {
				return nil, nil, "no_vol"
			}
		}
		pnl[i] = new(big.Rat).Mul(size, new(big.Rat).Sub(val, mark))
		if isPut && qty.Sign() < 0 && !o.CashSettled && strike.Cmp(moved) > 0 {
			cash[i] = new(big.Rat).Mul(strike, new(big.Rat).Abs(size))
		}
	}
	return pnl, cash, ""
}

func stale(b strategy.RiskBook, d contracts.Day) bool {
	return b.SessionsOld != nil && b.SessionsOld(d) > MaxSessionsOld
}

// repriced is the model value of the option at the moved spot and the volatility implied by its own mark.
func (s stresser) repriced(in optionIn, moved *big.Rat, vol string) (*big.Rat, bool) {
	txt, err := s.p.Value(in.right, moved.FloatString(8), in.strike, in.years, in.rate, vol)
	if err != nil {
		return nil, false
	}
	return parseDec(txt)
}

// optionGate is the days to expiry of an option leg, or the reason it is left out: expired, no price at all
// (no_vol), or a mark or spot that cannot be used (stale_mark).
func optionGate(b strategy.RiskBook, h strategy.RiskHolding) (int64, string) {
	exp, err := time.Parse(dayLayout, h.Option.Expiry)
	run, err2 := time.Parse(dayLayout, string(b.RunDay))
	if err != nil || err2 != nil {
		return 0, "no_vol"
	}
	days := int64(exp.Sub(run).Hours() / 24)
	switch {
	case days <= 0:
		return 0, "expired"
	case h.Mark == "":
		return 0, "no_vol"
	case !h.MarkUsable || h.MarkDay != h.SpotDay || stale(b, h.MarkDay) || stale(b, h.SpotDay):
		return 0, "stale_mark"
	}
	return days, ""
}

// Pricer values an option and solves the volatility of a price; every number is decimal text, years is the time
// to expiry in years and rate the continuously compounded risk-free rate. An expired option, or a price at or
// below intrinsic value, is an error of Implied.
type Pricer interface {
	// Value is the price of one unit of the option at volatility vol.
	Value(right, spot, strike, years, rate, vol string) (string, error)
	// Implied is the volatility at which the option is worth price.
	Implied(right, spot, strike, years, rate, price string) (string, error)
}
