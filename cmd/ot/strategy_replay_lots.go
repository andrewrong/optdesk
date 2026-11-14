package main

import (
	"math/big"
	"slices"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// Lot fates (docs/design/strategy-replay.md, Decision 2).
const (
	fateHeld         = "held"
	fateAssigned     = "assigned"
	fateAssignedEarl = "assigned_early"
	fateClosedEarly  = "closed_early"
	fateRolled       = "rolled"
	fateOpen         = "open"
)

// shortLots keeps the lots of Decision 1: a lot is short when its quantity is negative; for short_puts its right
// is P.
func shortLots(positions []compute.LedgerPosition, set string) []compute.LedgerPosition {
	var out []compute.LedgerPosition
	for _, p := range positions {
		q, ok := new(big.Rat).SetString(p.Quantity)
		if !ok || q.Sign() >= 0 {
			continue
		}
		if set == lotsShortPuts && !isRight(p.Right, "P") {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isRight(right, want string) bool {
	return strings.HasPrefix(strings.ToUpper(right), want)
}

// lotRange is the first entry day and the later of the as-of day and the last expiry of the lots.
func lotRange(lots []compute.LedgerPosition, asOf contracts.Day) (from, to contracts.Day) {
	from, to = lots[0].OpenedOn, asOf
	for _, p := range lots {
		if p.OpenedOn < from {
			from = p.OpenedOn
		}
		if contracts.Day(p.Expiry) > to {
			to = contracts.Day(p.Expiry)
		}
	}
	return from, to
}

// buildLots maps each short position to a Lot with its fate and settle day; sessions order the "next session day"
// of the rolled rule. The actual of an assigned lot is left empty: it needs the settle-day close (priceAssigned).
// Lots sort by entry day, then contract, and the id keeps that order.
func buildLots(positions []compute.LedgerPosition, sessions []contracts.Day) []replayapi.Lot {
	out := make([]replayapi.Lot, 0, len(positions))
	for _, p := range positions {
		l := replayapi.Lot{
			ID:         string(p.OpenedOn) + " " + p.Contract + " " + p.PositionID,
			Underlying: p.Underlying, Right: rightLetter(p.Right), Strike: p.Strike, Currency: p.Currency,
			Multiplier: p.Multiplier, Expiry: contracts.Day(p.Expiry), EntryDay: p.OpenedOn,
			Credit: p.OpenPrice, SettleDay: contracts.Day(p.Expiry), Fate: fateOpen,
		}
		switch p.Status {
		case "expired":
			l.Fate, l.ActualGross = fateHeld, normalized(p.OpenPrice)
		case "assigned", "exercised":
			l.Fate = fateAssigned
			if p.Early {
				l.Fate, l.SettleDay = fateAssignedEarl, p.ClosedOn
			}
		case "closed":
			l.Fate = fateClosedEarly
			if rolledInto(p, positions, sessions) {
				l.Fate = fateRolled
			}
			l.ActualGross = minus(p.OpenPrice, buyBack(p))
		}
		if p.Status != "open" {
			l.ActualNet = netPerShare(p)
		}
		out = append(out, l)
	}
	slices.SortStableFunc(out, func(a, b replayapi.Lot) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// buyBack is the price of the last fill that is not the open.
func buyBack(p compute.LedgerPosition) string {
	for i := len(p.Fills) - 1; i >= 0; i-- {
		if p.Fills[i].Action != "open" {
			return p.Fills[i].Price
		}
	}
	return ""
}

// rolledInto is true when a new short put of the same alias and underlying with a later expiry opens on the day
// of the buy-back or on the next session day.
func rolledInto(p compute.LedgerPosition, all []compute.LedgerPosition, sessions []contracts.Day) bool {
	next := contracts.Day("")
	for _, d := range sessions {
		if d > p.ClosedOn {
			next = d
			break
		}
	}
	for _, n := range all {
		if n.Alias != p.Alias || n.Underlying != p.Underlying || !isRight(n.Right, "P") || n.Expiry <= p.Expiry {
			continue
		}
		if q, ok := new(big.Rat).SetString(n.Quantity); !ok || q.Sign() >= 0 {
			continue
		}
		if n.OpenedOn == p.ClosedOn || (next != "" && n.OpenedOn == next) {
			return true
		}
	}
	return false
}

// priceAssigned sets the actual of each assigned lot to credit minus intrinsic value at the close of its settle
// day (Decision 2); RealizedPnL is never the basis. A lot without that close keeps an empty actual.
func priceAssigned(lots []replayapi.Lot, mkt replayapi.Market) {
	for i, l := range lots {
		if l.Fate != fateAssigned && l.Fate != fateAssignedEarl {
			continue
		}
		credit, ok1 := new(big.Rat).SetString(l.Credit)
		strike, ok2 := new(big.Rat).SetString(l.Strike)
		px, ok3 := new(big.Rat).SetString(mkt.Closes[l.Underlying][l.SettleDay])
		if !ok1 || !ok2 || !ok3 {
			continue
		}
		in := new(big.Rat).Sub(strike, px)
		if l.Right == "C" {
			in.Neg(in)
		}
		if in.Sign() < 0 {
			in.SetInt64(0)
		}
		lots[i].ActualGross = ratText(new(big.Rat).Sub(credit, in))
	}
}

// netPerShare is RealizedPnL over quantity and multiplier; empty when a figure is missing.
func netPerShare(p compute.LedgerPosition) string {
	pnl, ok1 := new(big.Rat).SetString(p.RealizedPnL)
	q, ok2 := new(big.Rat).SetString(p.Quantity)
	m, ok3 := new(big.Rat).SetString(p.Multiplier)
	if !ok1 || !ok2 || !ok3 {
		return ""
	}
	d := new(big.Rat).Mul(new(big.Rat).Abs(q), m)
	if d.Sign() == 0 {
		return ""
	}
	return ratText(pnl.Quo(pnl, d))
}

func minus(a, b string) string {
	x, ok1 := new(big.Rat).SetString(a)
	y, ok2 := new(big.Rat).SetString(b)
	if !ok1 || !ok2 {
		return ""
	}
	return ratText(x.Sub(x, y))
}

// ratText prints r to at most 4 places without trailing zeros.
func ratText(r *big.Rat) string {
	s := r.FloatString(4)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func rightLetter(r string) string {
	if isRight(r, "C") {
		return "C"
	}
	if isRight(r, "P") {
		return "P"
	}
	return strings.ToUpper(r)
}

// normalized is the decimal text without trailing zeros; text that is no decimal gives "".
func normalized(s string) string {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return ""
	}
	return ratText(r)
}
