package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func TestRiskDeltaByUnderlyingVegaByExpiry(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Risk(blockapi.RiskIn{
		Currency: "USD",
		Stocks:   []blockapi.HoldingIn{hold("AAPL", "USD", "100", "1", "1", "")},
		Greeks: []blockapi.GreekIn{
			{Underlying: "AAPL", Expiry: "2026-12-18", Delta: "-30", Vega: "5"},
			{Underlying: "MSFT", Expiry: "2026-12-18", Delta: "10", Vega: "2.5"},
			{Underlying: "AAPL", Expiry: "2027-01-15", Delta: "-20", Vega: "1"},
		},
	})
	if len(got.Delta) != 2 || got.Delta[0].Key != "AAPL" || got.Delta[0].Value != fv("50.00") || got.Delta[1].Value != fv("10.00") {
		t.Fatalf("delta: %+v", got.Delta)
	}
	if got.DeltaTotal != fv("60.00") {
		t.Fatalf("delta total: %+v", got.DeltaTotal)
	}
	if len(got.Vega) != 2 || got.Vega[0].Key != "2026-12-18" || got.Vega[0].Value != fv("7.50") || got.Vega[1].Value != fv("1.00") || got.VegaTotal != fv("8.50") {
		t.Fatalf("vega: %+v %+v", got.Vega, got.VegaTotal)
	}
}

func TestRiskForeignStockSkipped(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Risk(blockapi.RiskIn{
		Currency: "USD",
		Stocks:   []blockapi.HoldingIn{hold("A", "USD", "10", "1", "1", ""), hold("T", "JPY", "500", "1", "1", "")},
	})
	if len(got.Delta) != 1 || got.Delta[0].Key != "A" || got.DeltaTotal != gap("unsupported_currency") {
		t.Fatalf("%+v", got)
	}
	if got.VegaTotal != fv("0.00") {
		t.Fatalf("vega total: %+v", got.VegaTotal)
	}
}

func TestRiskUnreadableGreekGapsTotal(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Risk(blockapi.RiskIn{Currency: "USD", Greeks: []blockapi.GreekIn{{Underlying: "A", Expiry: "2026-12-18", Delta: "", Vega: "1"}}})
	if got.Delta[0].Value != gap("missing_input") || got.DeltaTotal != gap("missing_input") || got.VegaTotal != fv("1.00") {
		t.Fatalf("%+v", got)
	}
}
