package optionnotice

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func cashIn(ps ...compute.NoticePosition) compute.NoticeInput {
	return compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-09", Positions: ps}
}

func TestCashByDateSumsShortPuts(t *testing.T) {
	t.Parallel()
	r := eval(cashIn(shortOpt("AAA", "2026-12-18", "50", "P", "-1"), shortOpt("BBB", "2026-12-18", "40", "P", "-2")))
	want := []compute.CashDay{{Day: "2026-12-18", Currency: "USD", Amount: "13000"}}
	if !reflect.DeepEqual(r.CashByDate, want) {
		t.Fatalf("got %v", r.CashByDate)
	}
}

func TestCashByDatePerCurrency(t *testing.T) {
	t.Parallel()
	jpy := shortOpt("CCC", "2026-12-18", "2.5", "P", "-1")
	jpy.Currency = "EUR"
	r := eval(cashIn(shortOpt("AAA", "2026-12-18", "50", "P", "-1"), jpy, shortOpt("AAA", "2026-11-20", "10", "P", "-1")))
	want := []compute.CashDay{
		{Day: "2026-11-20", Currency: "USD", Amount: "1000"},
		{Day: "2026-12-18", Currency: "EUR", Amount: "250"},
		{Day: "2026-12-18", Currency: "USD", Amount: "5000"},
	}
	if !reflect.DeepEqual(r.CashByDate, want) {
		t.Fatalf("got %v", r.CashByDate)
	}
}

func TestCashIgnoresCallsAndLongs(t *testing.T) {
	t.Parallel()
	r := eval(cashIn(shortOpt("AAA", "2026-12-18", "50", "C", "-1"), shortOpt("BBB", "2026-12-18", "40", "P", "2"), stock("CCC", "-100")))
	if len(r.CashByDate) != 0 {
		t.Fatalf("got %v", r.CashByDate)
	}
}

func cashWith(vs []compute.NoticeValue, ps ...compute.NoticePosition) compute.NoticeInput {
	in := cashIn(ps...)
	in.Values = vs
	return in
}

func usdCash(v string) []compute.NoticeValue {
	return []compute.NoticeValue{{Tag: "TotalCashValue", Value: v, Currency: "USD"}}
}

func TestCashLeftAfterCumulativeDues(t *testing.T) {
	t.Parallel()
	r := eval(cashWith(usdCash("50000"), shortOpt("AAA", "2026-11-20", "340", "P", "-1"), shortOpt("BBB", "2026-12-18", "140", "P", "-1")))
	want := []compute.CashDay{
		{Day: "2026-11-20", Currency: "USD", Amount: "34000", Available: "50000", Left: "16000"},
		{Day: "2026-12-18", Currency: "USD", Amount: "14000", Available: "50000", Left: "2000"},
	}
	if !reflect.DeepEqual(r.CashByDate, want) {
		t.Fatalf("got %v", r.CashByDate)
	}
}

func TestCashLeftNegativeIsShortfall(t *testing.T) {
	t.Parallel()
	r := eval(cashWith(usdCash("10000"), shortOpt("AAA", "2026-12-18", "140", "P", "-1")))
	if len(r.CashByDate) != 1 || r.CashByDate[0].Left != "-4000" {
		t.Fatalf("got %v", r.CashByDate)
	}
}

func TestCashOtherCurrencyHasNoFigure(t *testing.T) {
	t.Parallel()
	jpy := shortOpt("CCC", "2026-12-18", "2.5", "P", "-1")
	jpy.Currency = "JPY"
	r := eval(cashWith(usdCash("50000"), jpy))
	if len(r.CashByDate) != 1 || r.CashByDate[0].Available != "" || r.CashByDate[0].Left != "" {
		t.Fatalf("got %v", r.CashByDate)
	}
}

func TestCashPicksCashRowOfSameCurrency(t *testing.T) {
	t.Parallel()
	vs := []compute.NoticeValue{
		{Tag: "TotalCashValue", Value: "5000000", Currency: "JPY"},
		{Tag: "TotalCashValue", Value: "50000", Currency: "USD"},
	}
	r := eval(cashWith(vs, shortOpt("AAA", "2026-11-20", "340", "P", "-1")))
	if len(r.CashByDate) != 1 || r.CashByDate[0].Available != "50000" || r.CashByDate[0].Left != "16000" {
		t.Fatalf("got %v", r.CashByDate)
	}
}
