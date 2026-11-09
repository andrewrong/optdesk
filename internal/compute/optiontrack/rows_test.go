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
		r.PremiumIncome != "150.0000" || r.Commissions != "1.3000" || r.OpenedOn != "2026-10-01" || r.Spot != "110" {
		t.Fatalf("%+v", r)
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
