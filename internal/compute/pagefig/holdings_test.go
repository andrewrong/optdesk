package pagefig

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func hold(und, cur, qty, mult, cost, mark string) blockapi.HoldingIn {
	return blockapi.HoldingIn{Underlying: und, SecType: "STK", Currency: cur, Quantity: qty, Multiplier: mult, Cost: cost, Mark: mark}
}

func fv(v string) compute.FigureValue { return compute.FigureValue{Value: v} }

func TestHoldingsValueUnrealisedShare(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Holdings(blockapi.HoldingsIn{
		Currency: "USD", NetLiquidation: "10000",
		Rows: []blockapi.HoldingIn{
			hold("AAPL", "USD", "100", "1", "150", "180"),
			{Underlying: "AAPL", SecType: "OPT", Currency: "USD", Quantity: "-2", Multiplier: "100", Cost: "3", Mark: "1.5"},
		},
	})
	a, o := got.Rows[0], got.Rows[1]
	if a.Value != fv("18000.00") || a.Unrealised != fv("3000.00") || a.Share != fv("180.00") {
		t.Fatalf("stock: %+v", a)
	}
	if o.Value != fv("-300.00") || o.Unrealised != fv("300.00") || o.Share != fv("-3.00") {
		t.Fatalf("option: %+v", o)
	}
	if got.TotalValue != fv("17700.00") || got.TotalUnrealised != fv("3300.00") {
		t.Fatalf("totals: %+v %+v", got.TotalValue, got.TotalUnrealised)
	}
}

func TestHoldingsMissingMarkLeavesOutOfTotals(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Holdings(blockapi.HoldingsIn{
		Currency: "USD", NetLiquidation: "1000",
		Rows: []blockapi.HoldingIn{hold("A", "USD", "1", "1", "1", "2"), hold("B", "USD", "1", "1", "1", "")},
	})
	g := gap("missing_quote")
	b := got.Rows[1]
	if b.Value != g || b.Unrealised != g || b.Share != g {
		t.Fatalf("row: %+v", b)
	}
	if got.Rows[0].Value != fv("2.00") || got.TotalValue != g || got.TotalUnrealised != g {
		t.Fatalf("totals: %+v", got)
	}
}

func TestHoldingsForeignCurrencyShare(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Holdings(blockapi.HoldingsIn{
		Currency: "USD", NetLiquidation: "1000",
		Rows: []blockapi.HoldingIn{hold("A", "USD", "1", "1", "1", "2"), hold("T", "JPY", "1", "1", "1", "2")},
	})
	if got.Rows[1].Share != gap("unsupported_currency") || got.TotalValue != gap("unsupported_currency") {
		t.Fatalf("%+v", got)
	}
	if got.Rows[0].Share != fv("0.20") {
		t.Fatalf("usd share: %+v", got.Rows[0])
	}
}

func TestHoldingsEmptyNetLiquidationSharesNoData(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Holdings(blockapi.HoldingsIn{Currency: "USD", Rows: []blockapi.HoldingIn{hold("A", "USD", "1", "1", "1", "2")}})
	if got.Rows[0].Share != gap("no_data") || got.Rows[0].Value != fv("2.00") || got.TotalValue != fv("2.00") {
		t.Fatalf("%+v", got)
	}
}

func TestJpyRowConvertedWithRate(t *testing.T) {
	t.Parallel()
	got := NewHoldings().Holdings(blockapi.HoldingsIn{
		Currency: "USD", NetLiquidation: "100000", UsdJpy: "150",
		Rows: []blockapi.HoldingIn{hold("T", "JPY", "1", "1", "3000000", "3474600")},
	})
	r := got.Rows[0]
	if r.Currency != "USD" || r.Value != fv("23164.00") || r.Unrealised != fv("3164.00") || r.Share != fv("23.16") {
		t.Fatalf("row: %+v", r)
	}
	if got.TotalValue != fv("23164.00") || got.TotalUnrealised != fv("3164.00") {
		t.Fatalf("totals: %+v", got)
	}
}

func TestJpyRowKeepsYenWithoutRate(t *testing.T) {
	t.Parallel()
	for _, rate := range []string{"", "0", "-1"} {
		got := NewHoldings().Holdings(blockapi.HoldingsIn{
			Currency: "USD", NetLiquidation: "1000", UsdJpy: rate,
			Rows: []blockapi.HoldingIn{hold("T", "JPY", "1", "1", "1", "3474600")},
		})
		g := gap("unsupported_currency")
		r := got.Rows[0]
		if r.Currency != "JPY" || r.Value != fv("3474600.00") || r.Share != g || got.TotalValue != g || got.TotalUnrealised != g {
			t.Fatalf("rate %q: %+v", rate, got)
		}
	}
}

func TestUnsupportedPairStaysGap(t *testing.T) {
	t.Parallel()
	g := gap("unsupported_currency")
	for _, in := range []blockapi.HoldingsIn{
		{Currency: "JPY", NetLiquidation: "1000", UsdJpy: "150", Rows: []blockapi.HoldingIn{hold("A", "USD", "1", "1", "1", "2")}},
		{Currency: "USD", NetLiquidation: "1000", UsdJpy: "150", Rows: []blockapi.HoldingIn{hold("A", "EUR", "1", "1", "1", "2")}},
	} {
		got := NewHoldings().Holdings(in)
		r := got.Rows[0]
		if r.Value != g || r.Unrealised != g || r.Share != g || got.TotalValue != g || r.Currency != in.Rows[0].Currency {
			t.Fatalf("%+v", got)
		}
	}
	m := NewHoldings().Holdings(blockapi.HoldingsIn{Currency: "USD", Rows: []blockapi.HoldingIn{hold("A", "USD", "1", "1", "1", "2")}})
	if m.Rows[0].Currency != "USD" {
		t.Fatalf("matching row: %+v", m.Rows[0])
	}
}
