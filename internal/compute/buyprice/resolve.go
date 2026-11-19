// Package buyprice resolves the buy price of a company: the top of the valuation buy zone,
// else the hand price, else no price (docs/design/buy-price-source.md, Decisions 1 and 2).
// The resolver is pure: it adds, rounds and compares no price; the price is the text of the
// valuation calculator or the hand text.
package buyprice

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/buypriceapi"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

type resolver struct{ calc valuelinesapi.Calculator }

// New returns the resolver over the calculator that computes the lines.
func New(calc valuelinesapi.Calculator) buypriceapi.Resolver { return resolver{calc: calc} }

// Resolve tries the valuation zone, then the hand price, then gives none.
func (r resolver) Resolve(in buypriceapi.Input) buypriceapi.Result {
	price, day, reason := r.zone(in)
	if reason == "" {
		return buypriceapi.Result{Price: price, Source: buypriceapi.SourceValuation, ReviewDay: day}
	}
	if positiveDecimal(in.Hand) {
		return buypriceapi.Result{Price: in.Hand, Source: buypriceapi.SourceHand, ZoneReason: reason}
	}
	return buypriceapi.Result{Source: buypriceapi.SourceNone, ZoneReason: reason}
}

// zone returns the BuyHigh text and review day of the one usable profile, or the reason.
func (r resolver) zone(in buypriceapi.Input) (price, day, reason string) {
	if in.Symbol == "" {
		return "", "", buypriceapi.ReasonNoProfile
	}
	var found []buypriceapi.Zone
	for _, z := range in.Zones {
		if strings.EqualFold(z.Symbol, in.Symbol) {
			found = append(found, z)
		}
	}
	switch {
	case len(found) == 0:
		return "", "", buypriceapi.ReasonNoProfile
	case len(found) > 1:
		return "", "", buypriceapi.ReasonAmbiguous
	}
	z := found[0]
	switch {
	case z.Pool == "removed":
		return "", "", buypriceapi.ReasonRemoved
	case !z.Reviewed:
		return "", "", buypriceapi.ReasonNoReview
	case !in.Tables.Present:
		return "", "", buypriceapi.ReasonNoTables
	case z.Currency != in.Currency:
		return "", "", buypriceapi.ReasonCurrency
	}
	lines, err := r.calc.Lines(valuelinesapi.Input{
		V: z.V, P: z.P, Market: z.Market, R: in.Tables.R, Margin: in.Tables.Margin,
	})
	if err != nil || !positiveDecimal(lines.BuyHigh) {
		return "", "", buypriceapi.ReasonBadInput
	}
	return lines.BuyHigh, z.ReviewDay, ""
}

// positiveDecimal reports whether s is plain decimal text (digits with at most one point,
// no sign, no exponent) with a non-zero digit. It reads the text; it computes nothing.
func positiveDecimal(s string) bool {
	digits, nonZero, dots := 0, false, 0
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
			digits++
			if c != '0' {
				nonZero = true
			}
		case c == '.':
			dots++
		default:
			return false
		}
	}
	return digits > 0 && dots <= 1 && nonZero
}
