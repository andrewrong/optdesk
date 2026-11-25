package option_pnl_page

import (
	"slices"

	"github.com/shopspring/decimal"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/store/optionday"
)

// markOf is the mark of an open lot: the latest stored row that is a quote day by the ledger rule
// (ask above 0, bid at or above 0, underlying price above 0; rows after the expiry are ignored). The
// row's texts are passed unchanged: the replay takes the mid and the age. The only operations on
// figures are sign tests. A contract that is not a conid has no rows and so no mark.
func markOf(p compute.LedgerPosition, rows []optionday.Day) (compute.LedgerMark, bool) {
	for _, r := range slices.Backward(rows) {

		if string(r.SessionDay) > p.Expiry || !passesLedgerRule(r) {
			continue
		}
		return compute.LedgerMark{Contract: p.Contract, Day: r.SessionDay, Bid: deref(r.Bid), Ask: deref(r.Ask),
			UnderlyingClose: deref(r.UnderlyingPrice)}, true
	}
	return compute.LedgerMark{}, false
}

func passesLedgerRule(r optionday.Day) bool {
	bid, e1 := decimal.NewFromString(deref(r.Bid))
	ask, e2 := decimal.NewFromString(deref(r.Ask))
	spot, e3 := decimal.NewFromString(deref(r.UnderlyingPrice))
	return e1 == nil && e2 == nil && e3 == nil && ask.IsPositive() && bid.Sign() >= 0 && spot.IsPositive()
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
