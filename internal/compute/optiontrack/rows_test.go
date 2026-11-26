package optiontrack

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/trackapi"
)

// TestRowCopiesLedgerTerms reads the row of a call: the terms and the ledger's figures are
// copied, the right is spelled out.
func TestRowCopiesLedgerTerms(t *testing.T) {
	t.Parallel()
	r := one(t, pos("p9", call), callQuote("1", "1.2", "110"))
	if r.PositionID != "p9" || r.Right != "call" || r.Strike != "100" || r.Expiry != "2026-12-18" ||
		r.Quantity != "-1" || r.Multiplier != "100" || r.Currency != "USD" || r.Status != "open" ||
		r.PremiumIncome != "150.0000" || r.Commissions != "1.3000" || r.OpenedOn != "2026-10-01" || r.Spot != "110" ||
		r.BrokerRealized != "" {
		t.Fatalf("%+v", r)
	}
	r = one(t, pos("p9", func(p *compute.LedgerPosition) { p.BrokerRealized = "-3577.86" }))
	if r.BrokerRealized != "-3577.86" {
		t.Fatalf("broker realized not copied: %+v", r)
	}
}

// TestOpeningCreditAndClosingCostApart reads the two halves from the fills: a buy-back is a
// cost, an expiry costs nothing, an open piece has no cost yet, no fills give neither.
func TestOpeningCreditAndClosingCostApart(t *testing.T) {
	t.Parallel()
	open := compute.LedgerFill{Action: "open", Quantity: "-1", Price: "20.00"}
	closed := baseRow(pos("c", func(p *compute.LedgerPosition) {
		p.Status, p.ClosedOn, p.PremiumIncome, p.RealizedPnL = "closed", "2026-11-01", "1990.0000", "1980.0000"
		p.Fills = []compute.LedgerFill{open, {Action: "close", Quantity: "1", Price: "55.00"}}
	}))
	if closed.OpeningCredit != "2000.0000" || closed.ClosingCost != "5500.0000" ||
		closed.PremiumIncome != "1990.0000" || closed.RealizedPnL != "1980.0000" {
		t.Fatalf("closed: %+v", closed)
	}
	expired := baseRow(pos("e", func(p *compute.LedgerPosition) {
		p.Status = "expired"
		p.Fills = []compute.LedgerFill{open}
	}))
	if expired.OpeningCredit != "2000.0000" || expired.ClosingCost != "0.0000" {
		t.Fatalf("expired: %+v", expired)
	}
	opened := baseRow(pos("o", func(p *compute.LedgerPosition) { p.Fills = []compute.LedgerFill{open} }))
	if opened.OpeningCredit != "2000.0000" || opened.ClosingCost != "" {
		t.Fatalf("open: %+v", opened)
	}
	none := baseRow(pos("n", func(p *compute.LedgerPosition) { p.Status = "closed" }))
	if none.OpeningCredit != "" || none.ClosingCost != "" {
		t.Fatalf("no fills: %+v", none)
	}
}

func TestQuoteJoinedByTerms(t *testing.T) {
	t.Parallel()
	// a quote of another right, strike or expiry is not the position's mark
	q1 := callQuote("1", "1.2", "110")
	q2 := quote("1", "1.2", "110")
	q2.Strike = "105"
	q3 := quote("1", "1.2", "110")
	q3.Expiry = "2026-12-11"
	q4 := quote("1", "1.2", "110")
	q4.Underlying = "MSFT"
	if r := one(t, pos("a", nil), q1, q2, q3, q4); r.MarkStatus != "no_mark" {
		t.Fatalf("%+v", r)
	}
	if r := one(t, pos("a", func(p *compute.LedgerPosition) { p.Strike = "100.5" }), func() (q trackapi.TrackQuote) {
		q = quote("1", "1.2", "110")
		q.Strike = "100.50"
		return q
	}()); r.MarkStatus != "ok" {
		t.Fatalf("strike text compared as a number: %+v", r)
	}
}

func TestSpotKeptWithoutMid(t *testing.T) {
	t.Parallel()
	r := one(t, pos("a", call), callQuote("", "1.2", "110"))
	if r.MarkStatus != "no_mark" || r.Mid != "" || r.Spot != "110" || r.Moneyness != "itm" || r.OtmPercent != "-9.09" {
		t.Fatalf("%+v", r)
	}
}

func TestNoCloseIsUnknownMoneyness(t *testing.T) {
	t.Parallel()
	for name, qs := range map[string][]trackapi.TrackQuote{
		"no quote":    nil,
		"empty close": {callQuote("1", "1.2", "")},
		"zero close":  {callQuote("1", "1.2", "0")},
		"negative":    {callQuote("1", "1.2", "-5")},
		"unparsable":  {callQuote("1", "1.2", "n/a")},
	} {
		r := one(t, pos("a", call), qs...)
		if r.Spot != "" || r.Moneyness != "unknown" || r.OtmPercent != "" {
			t.Errorf("%s: %+v", name, r)
		}
	}
}
