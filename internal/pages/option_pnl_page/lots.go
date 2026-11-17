package option_pnl_page

import (
	"errors"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optionattrapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// pnlLot is one short put lot of the second replay with the stored rows and the close it needs.
type pnlLot struct {
	pos      compute.LedgerPosition
	rows     []optionday.Day
	endClose string
}

// outcome is one lot after the attribution: a lot with a reason has class and reason set and no result.
type outcome struct {
	lot    pnlLot
	in     optionattrapi.Input
	res    optionattrapi.Result
	class  string
	reason string
}

func (o outcome) isGap() bool { return o.reason != "" }

func endedKind(status string) bool {
	return status == "expired" || status == "assigned" || status == "exercised"
}

// ledgerPnL is the ledger figure of a lot: realised, or the unrealised one while it is open.
func ledgerPnL(p compute.LedgerPosition) string {
	if p.Status == statusOpen {
		return p.UnrealizedPnL
	}
	return p.RealizedPnL
}

// inputOf maps a lot to the attributor input (Decisions 5, 6, 9 and 12). No row is filtered out.
func inputOf(c Config, l pnlLot, rates []optionattrapi.Rate) optionattrapi.Input {
	p := l.pos
	lot := optionattrapi.Lot{Currency: p.Currency, Strike: p.Strike, Multiplier: p.Multiplier,
		Expiry: contracts.Day(p.Expiry), End: p.Status, EndClose: l.endClose}
	if p.Status != statusOpen {
		lot.EndDay = p.ClosedOn
	}
	for _, f := range p.Fills {
		if f.Action == "open" || f.Action == "close" {
			lot.Fills = append(lot.Fills, optionattrapi.Fill{Day: f.Day, Side: f.Action, Quantity: f.Quantity,
				Price: f.Price, Commission: f.Commission})
		}
	}
	quotes := make([]optionattrapi.Quote, 0, len(l.rows))
	for _, r := range l.rows {
		quotes = append(quotes, optionattrapi.Quote{Day: r.SessionDay, Bid: deref(r.Bid), Ask: deref(r.Ask),
			Spot: deref(r.UnderlyingPrice), Ref: optionattrapi.Ref{IV: deref(r.Iv), Delta: deref(r.Delta),
				Gamma: deref(r.Gamma), Theta: deref(r.ThetaPerCalendarDay), Vega: deref(r.VegaPerVolPoint)}})
	}
	return optionattrapi.Input{Lot: lot, Quotes: quotes, Rates: rates, LedgerPnL: ledgerPnL(p),
		LedgerMarkDay: p.MarkDay, MaxRateAgeDays: c.MaxRateAgeDays}
}

// reasonOf decides before the attribution whether the lot can be split (Decision 7).
func reasonOf(c Config, l pnlLot) (class, text string) {
	p := l.pos
	switch {
	case p.Status == statusOpen && p.MarkStatus == "no_mark":
		return classNoMark, fmt.Sprintf("no stored quote of %s within %d sessions: no split", p.Underlying, c.MarkMaxAgeSessions)
	case endedKind(p.Status) && l.endClose == "":
		return classNoClose, fmt.Sprintf("no stored close of %s on %s: no split", p.Underlying, p.ClosedOn)
	case p.Status != statusOpen && p.RealizedPnL == "":
		return classNoLedger, fmt.Sprintf("no ledger P&L of %s: no split", p.Underlying)
	}
	return "", ""
}

// attribute calls the attributor for one lot. A reason, or an ErrBadInput, makes the lot a gap lot;
// any other error is returned.
func attribute(c Config, l pnlLot, rates []optionattrapi.Rate) (outcome, error) {
	o := outcome{lot: l, in: inputOf(c, l, rates)}
	if class, text := reasonOf(c, l); class != "" {
		o.class, o.reason = class, text
		return o, nil
	}
	res, err := c.Attributor.Attribute(o.in)
	if errors.Is(err, optionattrapi.ErrBadInput) {
		o.class, o.reason = classRefused, refusedPrefix+err.Error()
		return o, nil
	}
	if err != nil {
		return o, fmt.Errorf("option_pnl_page: attribute %s: %w", l.pos.PositionID, err)
	}
	o.res = res
	return o, nil
}

// roll is true when another shown lot of the same underlying was closed on the day this one opened,
// or opened on the day this one was closed: an equality test on ledger text.
func roll(o outcome, all []outcome) bool {
	p := o.lot.pos
	for _, x := range all {
		q := x.lot.pos
		if q.PositionID == p.PositionID || q.Underlying != p.Underlying {
			continue
		}
		if (p.Status == "closed" && q.OpenedOn == p.ClosedOn) || (q.Status == "closed" && q.ClosedOn == p.OpenedOn) {
			return true
		}
	}
	return false
}
