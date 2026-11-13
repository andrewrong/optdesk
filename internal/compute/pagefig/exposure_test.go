package pagefig

import (
	"fmt"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
)

func accountCells(in blockapi.HouseholdInput) (stock, option blockapi.MoneyLine) {
	a := New().Household(in).Accounts
	return a[2], a[3]
}

func posIn(cur string, ps ...blockapi.PositionIn) blockapi.HouseholdInput {
	return blockapi.HouseholdInput{Values: vals("100000", "5000", cur), Positions: ps, PositionsRead: true}
}

func stkPos(und, cur, qty, mark string) blockapi.PositionIn {
	return blockapi.PositionIn{Underlying: und, SecType: "stock", Currency: cur, Quantity: qty, Multiplier: "1", Mark: mark}
}

func optPos(und, cur, qty, mark string) blockapi.PositionIn {
	return blockapi.PositionIn{Underlying: und, SecType: "option", Currency: cur, Quantity: qty, Multiplier: "100", Mark: mark}
}

func TestStockAndOptionValueFromMarks(t *testing.T) {
	t.Parallel()
	s, o := accountCells(posIn("USD", stkPos("AAPL", "USD", "10", "180.5"), optPos("TSLA", "USD", "-1", "2.50")))
	if s.Amount != fv("1805.00") || o.Amount != fv("-250.00") || s.Currency != "USD" {
		t.Fatalf("%+v %+v", s, o)
	}
	s, o = accountCells(posIn("USD"))
	if s.Amount != fv("0.00") || o.Amount != fv("0.00") {
		t.Fatalf("empty: %+v %+v", s, o)
	}
}

func TestUnmarkedPositionNamedInNote(t *testing.T) {
	t.Parallel()
	s, o := accountCells(posIn("USD", stkPos("AAPL", "USD", "10", ""), stkPos("MSFT", "USD", "1", "5")))
	if s.Amount != gap("missing_quote") || s.Note != "no mark: AAPL" || o.Amount != fv("0.00") {
		t.Fatalf("%+v %+v", s, o)
	}
}

func TestJpyPositionNeedsRate(t *testing.T) {
	t.Parallel()
	in := posIn("USD", stkPos("T", "JPY", "100", "3000"))
	s, _ := accountCells(in)
	if s.Amount != gap("unsupported_currency") || s.Note != "no USDJPY rate" {
		t.Fatalf("no rate: %+v", s)
	}
	in.UsdJpy = "150"
	s, _ = accountCells(in)
	if s.Amount != fv("2000.00") {
		t.Fatalf("rate: %+v", s)
	}
}

func TestOtherPairIsGap(t *testing.T) {
	t.Parallel()
	in := posIn("JPY", stkPos("A", "USD", "1", "2"))
	in.UsdJpy = "150"
	s, _ := accountCells(in)
	if s.Amount != gap("unsupported_currency") {
		t.Fatalf("usd in jpy net: %+v", s)
	}
	s, _ = accountCells(posIn("USD", stkPos("A", "EUR", "1", "2")))
	if s.Amount != gap("unsupported_currency") {
		t.Fatalf("third currency: %+v", s)
	}
}

func TestIndexPositionIsNamedGap(t *testing.T) {
	t.Parallel()
	s, o := accountCells(posIn("USD", stkPos("AAPL", "USD", "1", "2"), blockapi.PositionIn{Underlying: "SPX", SecType: "index", Currency: "USD"}))
	g := gap("missing_input")
	if s.Amount != g || o.Amount != g || s.Note != "unsupported type: SPX" || o.Note != s.Note {
		t.Fatalf("%+v %+v", s, o)
	}
}

func TestPositionsNotReadKeepsMissingQuote(t *testing.T) {
	t.Parallel()
	in := posIn("USD", stkPos("AAPL", "USD", "1", "2"))
	in.PositionsRead = false
	s, o := accountCells(in)
	g := compute.FigureValue{Gap: "missing_quote"}
	if s.Amount != g || o.Amount != g || s.Note != "" {
		t.Fatalf("%+v %+v", s, o)
	}
}

func TestUnmarkedNoteIsBounded(t *testing.T) {
	t.Parallel()
	var ps []blockapi.PositionIn
	for i := range 20 {
		ps = append(ps, optPos(fmt.Sprintf("SYMBOL%02d", i), "USD", "-1", ""))
	}
	_, o := accountCells(posIn("USD", ps...))
	want := "no mark: SYMBOL00, SYMBOL01, SYMBOL02, SYMBOL03, SYMBOL04 and 15 more"
	if o.Amount != gap("missing_quote") || o.Note != want || len(o.Note) > 200 {
		t.Fatalf("%q", o.Note)
	}
}
