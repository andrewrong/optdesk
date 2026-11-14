// Package fit computes the capital of a candidate and checks it against the available funds of an alias
// (docs/design/strategy-risk.md, Decisions 7 and 8). It reaches the stress only through strategy.RiskStresser.
package fit

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/strategy"
)

type fitter struct{ st strategy.RiskStresser }

// New returns the capital fit that revalues books through st.
func New(st strategy.RiskStresser) strategy.RiskFitter { return fitter{st: st} }

const (
	checkName     = "fit_capital"
	statusPass    = "pass"
	statusBlock   = "block"
	statusUnknown = "unknown"
	reasonNoFunds = "no_funds"
	reasonNoFx    = "no_fx"
	reasonCapital = "capital"
)

// Fit gives the one check fit_capital and the facts capital, open_put_strikes and worst_loss_with.
// No check is made on the facts.
func (f fitter) Fit(book strategy.RiskBook, out strategy.RiskOutcome, c strategy.RiskCandidate) strategy.RiskFit {
	facts := map[string]string{}
	cur := ""
	if len(c.Legs) > 0 {
		cur = c.Legs[0].Currency
	}
	capRat, _, haveCap := capital(c, book)
	if haveCap {
		facts["capital"] = capRat.FloatString(2)
	}
	facts["open_put_strikes"] = openPutStrikes(book, cur).FloatString(2)
	if w, ok := f.worstLossWith(book, out, c, cur); ok {
		facts["worst_loss_with"] = w
	}

	chk := strategy.Check{Name: checkName, Status: statusPass}
	var avail *big.Rat
	if book.Funds != nil {
		avail, _ = rat(book.Funds.Available)
	}
	switch {
	case book.Funds == nil || avail == nil:
		chk.Status, chk.Reason = statusUnknown, reasonNoFunds
	case book.Funds.Currency != cur:
		chk.Status, chk.Reason = statusUnknown, reasonNoFx
	case !haveCap:
		chk.Status, chk.Reason = statusUnknown, reasonNoRule
	// The comparison is with the broker's available funds, not with cash net of the puts already open.
	case capRat.Cmp(avail) > 0:
		chk.Status, chk.Reason = statusBlock, reasonCapital
	}
	return strategy.RiskFit{Check: chk, Facts: facts}
}

// openPutStrikes sums strike x multiplier x quantity of the book's short puts in cur.
func openPutStrikes(book strategy.RiskBook, cur string) *big.Rat {
	sum := new(big.Rat)
	for _, h := range book.Holdings {
		if h.Option == nil || h.Option.Right != "P" || h.Currency != cur {
			continue
		}
		qty, mult, strike, _, ok := terms(h)
		if !ok || qty.Sign() >= 0 {
			continue
		}
		sum.Add(sum, new(big.Rat).Mul(strike, new(big.Rat).Mul(mult, new(big.Rat).Neg(qty))))
	}
	return sum
}

// worstLossWith is the book's total in cur with the candidate's legs added to the class ladder; absent when
// the book has no complete total in cur.
func (f fitter) worstLossWith(book strategy.RiskBook, out strategy.RiskOutcome, c strategy.RiskCandidate, cur string) (string, bool) {
	if !hasTotal(out, cur) {
		return "", false
	}
	b := book
	b.Holdings = append(append([]strategy.RiskHolding(nil), book.Holdings...), c.Legs...)
	for i := len(book.Holdings); i < len(b.Holdings); i++ {
		if b.Holdings[i].Class == "" {
			b.Holdings[i].Class = c.Class
		}
	}
	for _, t := range f.st.Stress(b).Totals {
		if t.Currency == cur {
			return t.WorstLoss, true
		}
	}
	return "", false
}

func hasTotal(o strategy.RiskOutcome, cur string) bool {
	for _, t := range o.Totals {
		if t.Currency == cur {
			return true
		}
	}
	return false
}
