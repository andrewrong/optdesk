package optiontrack

import (
	"math/big"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// mark is the usable quote of one position: the mid under the ledger's mark rule.
type mark struct {
	mid, spot *big.Rat
	spotText  string
	day       contracts.Day
	delta, iv string
}

// newestQuote finds the newest quote of the position's terms that is not after the as-of day.
func newestQuote(in trackapi.TrackInput, pos compute.LedgerPosition) (trackapi.TrackQuote, bool) {
	var best trackapi.TrackQuote
	found := false
	for _, q := range in.Quotes {
		if q.Underlying != pos.Underlying || q.Expiry != contracts.Day(pos.Expiry) ||
			!sameStrike(q.Strike, pos.Strike) || side(q.Right) != side(pos.Right) {
			continue
		}
		if in.AsOf != "" && q.Day > in.AsOf {
			continue
		}
		if !found || q.Day > best.Day {
			best, found = q, true
		}
	}
	return best, found
}

// markFor applies the ledger's mark rule to that quote: at most MarkMaxAgeSessions session
// days old, a bid not negative and an ask positive.
func markFor(in trackapi.TrackInput, p trackapi.TrackParams, pos compute.LedgerPosition) (mark, bool) {
	best, found := newestQuote(in, pos)
	if !found {
		return mark{}, false
	}
	age := 0
	for _, d := range in.SessionDays {
		if d > best.Day && (in.AsOf == "" || d <= in.AsOf) {
			age++
		}
	}
	bid, ok1 := dec(best.Bid)
	ask, ok2 := dec(best.Ask)
	if age > p.MarkMaxAgeSessions || !ok1 || !ok2 || bid.Sign() < 0 || ask.Sign() <= 0 {
		return mark{}, false
	}
	mid := new(big.Rat).Add(bid, ask)
	mid.Quo(mid, big.NewRat(2, 1))
	m := mark{mid: mid, day: best.Day, delta: best.Delta, iv: best.Iv}
	if s, ok := dec(best.UnderlyingClose); ok && s.Sign() > 0 {
		m.spot, m.spotText = s, strings.TrimSpace(best.UnderlyingClose)
	}
	return m, true
}

// closeOf is the positive UnderlyingClose of the newest quote of the contract, with its text;
// it does not wait for a mid and has no age limit.
func closeOf(in trackapi.TrackInput, pos compute.LedgerPosition) (*big.Rat, string) {
	best, found := newestQuote(in, pos)
	if !found {
		return nil, ""
	}
	if s, ok := dec(best.UnderlyingClose); ok && s.Sign() > 0 {
		return s, strings.TrimSpace(best.UnderlyingClose)
	}
	return nil, ""
}

// baseRow copies what the ledger gave.
func baseRow(pos compute.LedgerPosition) trackapi.TrackedOption {
	return trackapi.TrackedOption{
		PositionID: pos.PositionID, Underlying: pos.Underlying, Right: sideName(pos.Right),
		Strike: pos.Strike, Expiry: pos.Expiry, Quantity: pos.Quantity, Multiplier: pos.Multiplier,
		Currency: pos.Currency, Status: pos.Status, Early: pos.Early,
		PremiumIncome: pos.PremiumIncome, Commissions: pos.Commissions, RealizedPnL: pos.RealizedPnL,
		OpenedOn: pos.OpenedOn, ClosedOn: pos.ClosedOn,
		OpeningCredit: openingCredit(pos), ClosingCost: closingCost(pos), BrokerRealized: pos.BrokerRealized,
	}
}

// fillMoney is quantity times price times multiplier of one fill; false when a number is missing.
func fillMoney(f compute.LedgerFill, mult *big.Rat) (*big.Rat, bool) {
	q, ok1 := dec(f.Quantity)
	px, ok2 := dec(f.Price)
	if !ok1 || !ok2 {
		return nil, false
	}
	m := new(big.Rat).Mul(q, px)
	return m.Mul(m, mult), true
}

// openingCredit is minus the opening fill's quantity times price times multiplier (Decision 1);
// empty without a usable opening fill.
func openingCredit(pos compute.LedgerPosition) string {
	mult, ok := dec(pos.Multiplier)
	if !ok || len(pos.Fills) == 0 {
		return ""
	}
	m, ok := fillMoney(pos.Fills[0], mult)
	if !ok {
		return ""
	}
	return fixed(m.Neg(m), 4)
}

// closingCost sums the closing fills' quantity times price times multiplier; "0.0000" when none
// has a price, empty while the piece is open or has no fills.
func closingCost(pos compute.LedgerPosition) string {
	mult, ok := dec(pos.Multiplier)
	if !ok || len(pos.Fills) == 0 || pos.Status == "open" {
		return ""
	}
	sum := new(big.Rat)
	for _, f := range pos.Fills[1:] {
		if m, ok := fillMoney(f, mult); ok {
			sum.Add(sum, m)
		}
	}
	return fixed(sum, 4)
}

// openRow builds the row of an open short position (Decisions 5 to 8).
func openRow(in trackapi.TrackInput, p trackapi.TrackParams, pos compute.LedgerPosition) trackapi.TrackedOption {
	row := baseRow(pos)
	isPut := side(pos.Right) == "P"
	expiry := contracts.Day(pos.Expiry)
	if n, ok := daysBetween(in.AsOf, expiry); ok {
		row.DTE = n
	}
	row.Moneyness = "unknown"
	strike, strikeOK := dec(pos.Strike)
	spot, spotText := closeOf(in, pos)
	if spot != nil && strikeOK {
		row.Spot = spotText
		row.Moneyness = moneyness(isPut, strike, spot)
		if o, ok := otmPercent(isPut, strike, spot); ok {
			row.OtmPercent = o
		}
	}
	m, ok := markFor(in, p, pos)
	if !ok {
		row.MarkStatus = "no_mark"
	} else {
		fillMark(&row, m, pos, isPut, strike, strikeOK)
	}

	ev := newestEvent(in.Events, pos.Underlying)
	if !ev.found {
		row.EventStatus = "no_event_row"
	} else {
		row.EventStatus = "ok"
		row.NextEarnings = ev.nextEarnings
		row.EarningsInLife = inLife(ev.nextEarnings, in.AsOf, expiry)
		if !isPut {
			row.NextExDate, row.ExAmount = ev.nextExDate, ev.exAmount
			row.ExInLife = inLife(ev.nextExDate, in.AsOf, expiry)
		}
	}

	if isPut {
		row.Guard = "n/a"
		return row
	}
	if g, ok := guardRow(in.Events, pos.Underlying, expiry, pos.Strike); ok && g.GuardStatus != "" {
		row.Guard, row.GuardReason, row.GuardBasis = g.GuardStatus, g.GuardReason, "bid"
	} else if ok {
		row.Guard, row.GuardReason, row.GuardBasis = "unknown", g.GuardReason, "bid"
	} else {
		row.Guard, row.GuardReason = "unknown", "no_guard_row"
	}
	return row
}

// fillMark sets the figures that need a usable mark.
func fillMark(row *trackapi.TrackedOption, m mark, pos compute.LedgerPosition, isPut bool, strike *big.Rat, strikeOK bool) {
	row.MarkStatus, row.MarkDay, row.Mid = "ok", m.day, fixed(m.mid, 4)
	row.Delta, row.Iv = m.delta, m.iv
	if m.spot != nil && strikeOK {
		row.Extrinsic = fixed(extrinsic(isPut, m.mid, strike, m.spot), 4)
	}
	prem, ok1 := dec(pos.PremiumIncome)
	qty, ok2 := dec(pos.Quantity)
	mult, ok3 := dec(pos.Multiplier)
	if !ok1 || !ok2 || !ok3 {
		return
	}
	value := new(big.Rat).Mul(m.mid, qty)
	value.Mul(value, mult)
	kept := new(big.Rat).Add(prem, value)
	row.Kept = fixed(kept, 4)
	if prem.Sign() != 0 {
		pct := new(big.Rat).Quo(kept, prem)
		pct.Mul(pct, big.NewRat(100, 1))
		row.KeptPercent = fixed(pct, 2)
	}
}

// recentRow builds the row of a closed short position.
func recentRow(pos compute.LedgerPosition) trackapi.TrackedOption {
	return baseRow(pos)
}
