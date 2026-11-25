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

func TestRiskForeignStockCounted(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Risk(blockapi.RiskIn{
		Currency: "USD",
		Stocks:   []blockapi.HoldingIn{hold("A", "USD", "10", "1", "1", ""), hold("T", "JPY", "500", "1", "1", "")},
	})
	if len(got.Delta) != 2 || got.Delta[1].Key != "T" || got.Delta[1].Value != fv("500.00") || got.DeltaTotal != fv("510.00") {
		t.Fatalf("%+v", got)
	}
	if got.VegaTotal != fv("0.00") {
		t.Fatalf("vega total: %+v", got.VegaTotal)
	}
}

func TestRiskJpyStockAndStructure(t *testing.T) {
	t.Parallel()
	in := blockapi.RiskIn{
		Currency: "USD",
		UsdJpy:   "150",
		Stocks:   []blockapi.HoldingIn{hold("T", "JPY", "500", "1", "1", "")},
		Greeks: []blockapi.GreekIn{
			{Underlying: "T", Expiry: "2026-12-18", Delta: "100", Vega: "3000", Currency: "JPY"},
			{Underlying: "A", Expiry: "2026-12-18", Delta: "5", Vega: "1", Currency: "USD"},
			{Underlying: "A", Expiry: "2027-01-15", Delta: "5", Vega: "2"},
		},
	}
	got := NewHoldings().Risk(in)
	if got.Delta[1].Key != "T" || got.Delta[1].Value != fv("600.00") || got.DeltaTotal != fv("610.00") {
		t.Fatalf("delta: %+v", got)
	}
	if got.Vega[0].Value != fv("21.00") || got.Vega[1].Value != fv("2.00") || got.VegaTotal != fv("23.00") {
		t.Fatalf("vega: %+v", got)
	}
	for _, rate := range []string{"", "0", "-1"} {
		in.UsdJpy = rate
		got = NewHoldings().Risk(in)
		if got.Vega[0].Value != gap("unsupported_currency") || got.Vega[1].Value != fv("2.00") || got.VegaTotal != gap("unsupported_currency") {
			t.Fatalf("rate %q vega: %+v", rate, got)
		}
		if got.DeltaTotal != fv("610.00") {
			t.Fatalf("rate %q delta: %+v", rate, got.DeltaTotal)
		}
	}
}

func TestRiskUnreadableGreekGapsTotal(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Risk(blockapi.RiskIn{Currency: "USD", Greeks: []blockapi.GreekIn{{Underlying: "A", Expiry: "2026-12-18", Delta: "", Vega: "1"}}})
	if got.Delta[0].Value != gap("missing_input") || got.DeltaTotal != gap("missing_input") || got.VegaTotal != fv("1.00") {
		t.Fatalf("%+v", got)
	}
}

func TestRiskDeltaTotalsPerCurrency(t *testing.T) {
	t.Parallel()
	t.Run("two currencies are not added", func(t *testing.T) {
		t.Parallel()
		got := NewHoldings().Risk(blockapi.RiskIn{
			Stocks: []blockapi.HoldingIn{hold("A", "USD", "10", "1", "1", ""), hold("T", "JPY", "500", "1", "1", "")},
		})
		want := []blockapi.RiskTotal{{Currency: "USD", Value: fv("10.00")}, {Currency: "JPY", Value: fv("500.00")}}
		if len(got.DeltaTotals) != 2 || got.DeltaTotals[0] != want[0] || got.DeltaTotals[1] != want[1] {
			t.Fatalf("totals: %+v", got.DeltaTotals)
		}
		if got.Delta[0].Currency != "USD" || got.Delta[1].Currency != "JPY" || got.DeltaTotal != fv("510.00") {
			t.Fatalf("lines: %+v total %+v", got.Delta, got.DeltaTotal)
		}
	})
	t.Run("gap USD line leaves the JPY total a number", func(t *testing.T) {
		t.Parallel()
		got := NewHoldings().Risk(blockapi.RiskIn{
			Stocks: []blockapi.HoldingIn{hold("A", "USD", "x", "1", "1", ""), hold("T", "JPY", "500", "1", "1", "")},
		})
		if len(got.DeltaTotals) != 2 || got.DeltaTotals[0] != (blockapi.RiskTotal{Currency: "USD", Value: gap("missing_input")}) ||
			got.DeltaTotals[1] != (blockapi.RiskTotal{Currency: "JPY", Value: fv("500.00")}) {
			t.Fatalf("totals: %+v", got.DeltaTotals)
		}
	})
	t.Run("empty currency reads USD, mixed and unsupported change no total", func(t *testing.T) {
		t.Parallel()
		got := NewHoldings().Risk(blockapi.RiskIn{
			Stocks: []blockapi.HoldingIn{hold("A", "", "10", "1", "1", ""), hold("M", "USD", "1", "1", "1", ""), hold("E", "EUR", "7", "1", "1", "")},
			Greeks: []blockapi.GreekIn{{Underlying: "M", Expiry: "2026-12-18", Delta: "1", Vega: "1", Currency: "JPY"}},
		})
		if len(got.DeltaTotals) != 1 || got.DeltaTotals[0] != (blockapi.RiskTotal{Currency: "USD", Value: fv("10.00")}) {
			t.Fatalf("totals: %+v", got.DeltaTotals)
		}
		if got.Delta[0].Currency != "USD" || got.Delta[1] != (blockapi.RiskLine{Key: "E", Currency: "EUR", Value: gap("unsupported_currency")}) ||
			got.Delta[2] != (blockapi.RiskLine{Key: "M", Currency: "", Value: gap("missing_input")}) {
			t.Fatalf("lines: %+v", got.Delta)
		}
	})
}
