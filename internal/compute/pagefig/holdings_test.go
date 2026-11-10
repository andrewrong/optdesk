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
