package optiontier

import (
	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// Reasons an alias figure is unknown.
const (
	whyNoCash        = "no_cash"
	whyNotUSD        = "not_usd"
	whyStaleCash     = "stale_cash"
	whyOtherCurrency = "other_currency"

	currencyUSD = "USD"
)

// aliasCash is the figure of one alias: known with its free and committed cash, or unknown with a reason.
type aliasCash struct {
	alias     string
	known     bool
	free      decimal.Decimal
	committed decimal.Decimal
	why       string
}

// buildAccounts gives one aliasCash per account in input order. free is Cash minus the sum of
// strike x multiplier x |quantity| of the short puts (SecType OPT, Right put, Quantity below 0) in USD.
// The reasons are tried in the order no_cash, not_usd, stale_cash, other_currency. A short put whose
// strike or multiplier is not a number cannot be netted, so its alias is unknown with no_cash.
func buildAccounts(in []optiontierapi.AccountCash, p optiontierapi.Params) []aliasCash {
	out := make([]aliasCash, 0, len(in))
	for _, a := range in {
		out = append(out, buildAlias(a, p))
	}
	return out
}

func buildAlias(a optiontierapi.AccountCash, p optiontierapi.Params) aliasCash {
	res := aliasCash{alias: a.Alias}
	cash, ok := parse(a.Cash)
	switch {
	case !ok:
		res.why = whyNoCash
		return res
	case a.Currency != currencyUSD:
		res.why = whyNotUSD
		return res
	case a.AgeDays > p.MaxCashAgeDays:
		res.why = whyStaleCash
		return res
	}
	committed := decimal.Zero
	for _, pos := range a.Positions {
		if pos.SecType != "OPT" || pos.Right != "put" {
			continue
		}
		qty, ok := parse(pos.Quantity)
		if !ok || !qty.IsNegative() {
			continue
		}
		if pos.Currency != currencyUSD {
			res.why = whyOtherCurrency
			return res
		}
		strike, okS := parse(pos.Strike)
		mult, okM := parse(pos.Multiplier)
		if !okS || !okM {
			res.why = whyNoCash
			return res
		}
		committed = committed.Add(strike.Mul(mult).Mul(qty.Abs()))
	}
	res.known, res.committed, res.free = true, committed, cash.Sub(committed)
	return res
}

func cashBuilt(accts []aliasCash) bool {
	for _, a := range accts {
		if a.known {
			return true
		}
	}
	return false
}

// need is strike x multiplier, the full cash a short put secures.
func (n numbers) need() (decimal.Decimal, bool) {
	if !n.okStrike {
		return decimal.Zero, false
	}
	return n.strike.Mul(n.mult), true
}

// writeCash fills the cash cells, the FitAlias and, with no figure built, the info cash_unknown.
func writeCash(s *rowState, n numbers, accts []aliasCash, built bool) {
	need, haveNeed := n.need()
	s.v.Cash = make([]optiontierapi.CashCell, 0, len(accts))
	for _, a := range accts {
		cell := optiontierapi.CashCell{Alias: a.alias, Verdict: "unknown", Why: a.why}
		switch {
		case !a.known:
		case !haveNeed:
			cell.Why = greyBadInput
			cell.Free, cell.Committed = a.free.String(), a.committed.String()
		default:
			cell.Why = ""
			cell.Free, cell.Committed = a.free.String(), a.committed.String()
			cell.Verdict = "short"
			if a.free.GreaterThanOrEqual(need) {
				cell.Verdict = "fits"
				if s.v.FitAlias == "" {
					s.v.FitAlias = a.alias
				}
			}
		}
		s.v.Cash = append(s.v.Cash, cell)
	}
	if !built {
		s.infos[infoCashUnknown] = true
	}
}

// cashFits: some known alias has free cash of at least strike x multiplier.
func (s *rowState) cashFits(n numbers) bool {
	if _, ok := n.need(); !ok {
		return false
	}
	return s.v.FitAlias != ""
}
