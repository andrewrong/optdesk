package pagefig

import (
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func usdNet(v string) blockapi.ValueIn {
	return blockapi.ValueIn{Tag: "NetLiquidation", Value: v, Currency: "USD"}
}

func runMovers(in blockapi.MoversIn) blockapi.MoversResult { return NewHoldings().Movers(in) }

func shortPut() blockapi.MoverIn {
	return blockapi.MoverIn{
		Name: "AAA", SecType: "option", Right: "put", Strike: "240.00", Expiry: "2026-11-20", Currency: "USD",
		Quantity: "-2", Multiplier: "100", PricePrev: "3.10", PriceLast: "2.40", BasisPrev: "mid", BasisLast: "mid",
	}
}

func stockRow(name, q, p0, p1 string) blockapi.MoverIn {
	return blockapi.MoverIn{
		Name: name, SecType: "stock", Currency: "USD", Quantity: q, Multiplier: "1",
		PricePrev: p0, PriceLast: p1, BasisPrev: "close", BasisLast: "close",
	}
}

func oneOption(r blockapi.MoverIn) blockapi.MoverLine {
	return runMovers(blockapi.MoversIn{NetPrev: usdNet("1"), NetLast: usdNet("1"), Rows: []blockapi.MoverIn{r}}).Options[0]
}

func TestMoverLineShortPut(t *testing.T) {
	t.Parallel()
	l := oneOption(shortPut())
	if l.Amount != (compute.FigureValue{Value: "140.00"}) {
		t.Fatalf("Amount = %+v", l.Amount)
	}
	if l.PricePrev.Value != "3.1000" || l.PriceLast.Value != "2.4000" {
		t.Fatalf("prices = %+v %+v", l.PricePrev, l.PriceLast)
	}
}

func TestMoverMissingQuoteLeavesOther(t *testing.T) {
	t.Parallel()
	bad := stockRow("BAD", "10", "5", "")
	r := runMovers(blockapi.MoversIn{
		NetPrev: usdNet("1000"), NetLast: usdNet("1100"),
		Rows: []blockapi.MoverIn{stockRow("OK", "10", "5", "6"), bad},
	})
	gapQ := compute.FigureValue{Gap: "missing_quote"}
	var l blockapi.MoverLine
	for _, x := range r.Stocks {
		if x.Name == "BAD" {
			l = x
		}
	}
	if l.PriceLast != gapQ || l.PricePrev != gapQ || l.Amount != gapQ {
		t.Fatalf("line = %+v", l)
	}
	if r.Total.Value != "100.00" || r.Other.Value != "90.00" {
		t.Fatalf("total %+v other %+v", r.Total, r.Other)
	}
}

func TestMoverBasisMismatchIsGap(t *testing.T) {
	t.Parallel()
	gapQ := compute.FigureValue{Gap: "missing_quote"}
	for _, b := range [][2]string{{"mid", "last"}, {"none", "none"}, {"mid", "none"}} {
		r := shortPut()
		r.BasisPrev, r.BasisLast = b[0], b[1]
		l := oneOption(r)
		if l.PricePrev != gapQ || l.PriceLast != gapQ || l.Amount != gapQ {
			t.Fatalf("bases %v: line = %+v", b, l)
		}
	}
}

func TestMoverOptionEmptyMultiplierIsGap(t *testing.T) {
	t.Parallel()
	r := shortPut()
	r.Multiplier = ""
	if l := oneOption(r); l.Amount != (compute.FigureValue{Gap: "missing_input"}) {
		t.Fatalf("option Amount = %+v", l.Amount)
	}
	s := runMovers(blockapi.MoversIn{NetPrev: usdNet("1"), NetLast: usdNet("1"), Rows: []blockapi.MoverIn{
		func() blockapi.MoverIn { x := stockRow("S", "3", "1.00", "1.50"); x.Multiplier = ""; return x }(),
	}}).Stocks[0]
	if s.Amount.Value != "1.50" {
		t.Fatalf("stock Amount = %+v", s.Amount)
	}
}

func TestMoverJpyNeedsRate(t *testing.T) {
	t.Parallel()
	r := stockRow("JP", "100", "1000", "1030")
	r.Currency = "JPY"
	in := blockapi.MoversIn{NetPrev: usdNet("1"), NetLast: usdNet("1"), Rows: []blockapi.MoverIn{r}}
	if a := runMovers(in).Stocks[0].Amount; a != (compute.FigureValue{Gap: "missing_input"}) {
		t.Fatalf("no rate: %+v", a)
	}
	in.UsdJpy = "150"
	if a := runMovers(in).Stocks[0].Amount; a.Value != "20.00" {
		t.Fatalf("rate 150: %+v", a)
	}
	r.Currency = "EUR"
	in.Rows = []blockapi.MoverIn{r}
	if a := runMovers(in).Stocks[0].Amount; a != (compute.FigureValue{Gap: "unsupported_currency"}) {
		t.Fatalf("EUR: %+v", a)
	}
}

func TestMoversTotalIsNetChange(t *testing.T) {
	t.Parallel()
	r := runMovers(blockapi.MoversIn{NetPrev: usdNet("10000.50"), NetLast: usdNet("10250.75"), Rows: []blockapi.MoverIn{shortPut()}})
	if r.Total.Value != "250.25" || r.Other.Value != "110.25" {
		t.Fatalf("total %+v other %+v", r.Total, r.Other)
	}
}

func TestMoversNetJpyConverted(t *testing.T) {
	t.Parallel()
	r := runMovers(blockapi.MoversIn{
		UsdJpy:  "150",
		NetPrev: blockapi.ValueIn{Tag: "NetLiquidation", Value: "15000000", Currency: "JPY"},
		NetLast: blockapi.ValueIn{Tag: "NetLiquidation", Value: "15150000", Currency: "JPY"},
	})
	if r.Total.Value != "1000.00" || r.Other.Value != "1000.00" {
		t.Fatalf("total %+v other %+v", r.Total, r.Other)
	}
}

func TestMoversNetMissingGap(t *testing.T) {
	t.Parallel()
	gapI := compute.FigureValue{Gap: "missing_input"}
	jpy := blockapi.ValueIn{Value: "1500", Currency: "JPY"}
	for name, in := range map[string]blockapi.MoversIn{
		"empty prev":  {NetPrev: usdNet(""), NetLast: usdNet("5")},
		"empty last":  {NetPrev: usdNet("5"), NetLast: usdNet("")},
		"jpy no rate": {NetPrev: jpy, NetLast: jpy},
	} {
		in.Rows = []blockapi.MoverIn{shortPut()}
		r := runMovers(in)
		if r.Total != gapI || r.Other != gapI {
			t.Fatalf("%s: total %+v other %+v", name, r.Total, r.Other)
		}
		if len(r.Options) != 1 || r.Options[0].Amount.Value != "140.00" {
			t.Fatalf("%s: lines %+v", name, r.Options)
		}
	}
}

func TestMoversNetOtherCurrencyGap(t *testing.T) {
	t.Parallel()
	gapU := compute.FigureValue{Gap: "unsupported_currency"}
	eur := blockapi.ValueIn{Value: "100", Currency: "EUR"}
	for name, in := range map[string]blockapi.MoversIn{
		"prev": {NetPrev: eur, NetLast: usdNet("5")},
		"last": {NetPrev: usdNet("5"), NetLast: eur},
	} {
		in.Rows = []blockapi.MoverIn{shortPut()}
		in.UsdJpy = "150"
		r := runMovers(in)
		if r.Total != gapU || r.Other != gapU || len(r.Options) != 1 {
			t.Fatalf("%s: %+v", name, r)
		}
	}
}

func TestMoversSortAndLimit(t *testing.T) {
	t.Parallel()
	var rows []blockapi.MoverIn
	for i := 1; i <= 61; i++ {
		rows = append(rows, stockRow(fmt.Sprintf("S%02d", i), "1", "10", fmt.Sprintf("%d", 10+i)))
	}
	// a gap line and a tie with S61 by amount (61): Name decides.
	rows = append(rows, stockRow("GAP", "1", "10", ""), stockRow("A00", "1", "10", "71"))
	// 61 lines of amounts 1..61 plus A00 (61): 62 numbered lines; the smallest two (1 and 2) are cut.
	r := runMovers(blockapi.MoversIn{NetPrev: usdNet("0"), NetLast: usdNet("2000"), Rows: rows})
	if len(r.Stocks) != 60 {
		t.Fatalf("len = %d", len(r.Stocks))
	}
	if r.Stocks[0].Name != "A00" || r.Stocks[1].Name != "S61" {
		t.Fatalf("tie order: %s %s", r.Stocks[0].Name, r.Stocks[1].Name)
	}
	if last := r.Stocks[59]; last.Name != "S03" {
		t.Fatalf("last = %+v", last)
	}
	// sum of listed = 61+61+sum(3..60) = 122+1827 = 1949; Other = 2000 - 1949.
	if r.Other.Value != "51.00" {
		t.Fatalf("Other = %+v", r.Other)
	}
}

func TestMoversGapSortsLast(t *testing.T) {
	t.Parallel()
	r := runMovers(blockapi.MoversIn{NetPrev: usdNet("0"), NetLast: usdNet("0"), Rows: []blockapi.MoverIn{
		stockRow("AAA", "1", "10", ""), stockRow("ZZZ", "1", "10", "11"), stockRow("MMM", "1", "10", "5"),
	}})
	got := []string{r.Stocks[0].Name, r.Stocks[1].Name, r.Stocks[2].Name}
	if got[0] != "MMM" || got[1] != "ZZZ" || got[2] != "AAA" {
		t.Fatalf("order = %v", got)
	}
}

func TestMoverNotes(t *testing.T) {
	t.Parallel()
	r := runMovers(blockapi.MoversIn{NetPrev: usdNet("1"), NetLast: usdNet("1"), Rows: []blockapi.MoverIn{
		stockRow("S", "100", "1", "2"), shortPut(),
	}})
	if n := r.Stocks[0].Note; n != "100 sh" {
		t.Fatalf("stock note %q", n)
	}
	if n := r.Options[0].Note; n != "short 2 put 240 11-20" {
		t.Fatalf("option note %q", n)
	}
	long := shortPut()
	long.Quantity, long.Right, long.Strike = "3", "call", "12.50"
	if n := oneOption(long).Note; n != "long 3 call 12.5 11-20" {
		t.Fatalf("long note %q", n)
	}
}
