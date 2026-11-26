package optiontier

import (
	"slices"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute/optiontierapi"
)

// Grey reasons, in the order of precedence (Decision 3).
const (
	greyNoBuyPrice = "no_buy_price"
	greyNoSpot     = "no_spot"
	greyInTheMoney = "in_the_money"
	greyNoBid      = "no_bid"
	greyBadInput   = "bad_input"

	markUnreadableGates = "unreadable_gates"
)

// greyOrder is the order of precedence and of display.
var greyOrder = []string{greyNoBuyPrice, greyNoSpot, greyInTheMoney, greyNoBid, greyBadInput}

// numbers are the parsed inputs of a row; a flag is false for an empty or unreadable text.
type numbers struct {
	buy, spot, strike, bid, mult           decimal.Decimal
	okBuy, okSpot, okStrike, okBid, okMult bool
}

func readNumbers(r optiontierapi.Row) numbers {
	var n numbers
	n.buy, n.okBuy = parse(r.BuyPrice)
	n.spot, n.okSpot = parse(r.Spot)
	n.strike, n.okStrike = parse(r.Strike)
	n.bid, n.okBid = parse(r.Bid)
	n.mult, n.okMult = parse(r.Multiplier)
	if !n.okMult {
		n.mult, n.okMult = decimal.NewFromInt(100), true
	}
	return n
}

// nonPositive: the text is empty, or a number that is not above 0.
func nonPositive(text string, d decimal.Decimal, ok bool) bool {
	return text == "" || (ok && !d.IsPositive())
}

// unreadable: some needed text is not a number, the mark says the gates are unreadable, or DTE is negative.
func unreadable(r optiontierapi.Row, n numbers) bool {
	return !n.okBuy || !n.okSpot || !n.okStrike || !n.okBid || hasMark(r.Marks, markUnreadableGates) || r.DTE < 0
}

// greyReason is the first grey reason of a row, "" when none. An empty or non-positive buy price, spot
// or bid is the reason of its own; a text that is not a number, or the mark unreadable_gates, is
// bad_input, which comes last.
func greyReason(r optiontierapi.Row, n numbers) string {
	switch {
	case nonPositive(r.BuyPrice, n.buy, n.okBuy):
		return greyNoBuyPrice
	case nonPositive(r.Spot, n.spot, n.okSpot):
		return greyNoSpot
	case n.okStrike && n.okSpot && n.strike.GreaterThan(n.spot):
		return greyInTheMoney
	case nonPositive(r.Bid, n.bid, n.okBid):
		return greyNoBid
	case unreadable(r, n):
		return greyBadInput
	}
	return ""
}

// needsEvaluate: the evaluator is called for a live row and for the grey that still shows figures.
func needsEvaluate(grey string) bool {
	return grey == "" || grey == greyInTheMoney
}

func hasMark(marks []string, m string) bool {
	return slices.Contains(marks, m)
}
