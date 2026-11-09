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
