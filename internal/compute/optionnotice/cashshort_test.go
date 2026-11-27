package optionnotice

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func csRow(day, cur, avail, left string) compute.CashDay {
	return compute.CashDay{Day: contracts.Day(day), Currency: cur, Available: avail, Left: left}
}

func csNotices(rows []compute.CashDay, build string) []compute.Notice {
	d, ok := parseDay(build)
	return cashShortNotices(rows, d, ok, DefaultParams())
}

func csFact(n compute.Notice, label string) string {
	for _, f := range n.Facts {
		if f.Label == label {
			return f.Value
		}
	}
	return "<none>"
}

func TestCashShortFirstDayPerCurrency(t *testing.T) {
	t.Parallel()
	ns := csNotices([]compute.CashDay{
		csRow("2026-11-20", "USD", "35553", "5000"),
		csRow("2026-11-27", "USD", "35553", "-30553"),
		csRow("2026-12-04", "USD", "35553", "-40000"),
	}, "2026-11-09")
	if len(ns) != 1 {
		t.Fatalf("got %v", ns)
	}
	n := ns[0]
	if n.Code != "cash_short" || n.Expiry != "2026-11-27" || n.Symbol != "" || n.Contract != "" {
		t.Fatalf("got %+v", n)
	}
	want := []compute.NoticeFact{{Label: "currency", Value: "USD"}, {Label: "available", Value: "35553"},
		{Label: "due", Value: "66106"}, {Label: "left", Value: "-30553"}, {Label: "short_days", Value: "2"},
		{Label: "reason", Value: "cash_short_cumulative"}}
	if !reflect.DeepEqual(n.Facts, want) {
		t.Fatalf("got %v", n.Facts)
	}
}

func TestCashShortDue(t *testing.T) {
	t.Parallel()
	ns := csNotices([]compute.CashDay{csRow("2026-11-20", "USD", "100000", "-30553.5")}, "2026-11-09")
	if len(ns) != 1 || csFact(ns[0], "due") != "130553.5" {
		t.Fatalf("got %v", ns)
	}
}

func TestCashShortNoCashNoNotice(t *testing.T) {
	t.Parallel()
	ns := csNotices([]compute.CashDay{csRow("2026-11-20", "USD", "", ""), csRow("2026-11-21", "USD", "1", "x")}, "2026-11-09")
	if len(ns) != 0 {
		t.Fatalf("got %v", ns)
	}
}

func TestCashShortZeroOrAbove(t *testing.T) {
	t.Parallel()
	if ns := csNotices([]compute.CashDay{csRow("2026-11-20", "USD", "100", "0")}, "2026-11-09"); len(ns) != 0 {
		t.Fatalf("got %v", ns)
	}
}

func TestCashShortCurrenciesApart(t *testing.T) {
	t.Parallel()
	one := csNotices([]compute.CashDay{csRow("2026-11-20", "USD", "1", "-5"), csRow("2026-11-20", "JPY", "9", "3")}, "2026-11-09")
	if len(one) != 1 || csFact(one[0], "currency") != "USD" {
		t.Fatalf("got %v", one)
	}
	two := csNotices([]compute.CashDay{csRow("2026-11-20", "USD", "1", "-5"), csRow("2026-11-20", "JPY", "1", "-5")}, "2026-11-09")
	if len(two) != 2 {
		t.Fatalf("got %v", two)
	}
}

func TestCashShortSeverity(t *testing.T) {
	t.Parallel()
	rows := []compute.CashDay{csRow("2026-11-14", "USD", "1", "-5")}
	if ns := csNotices(rows, "2026-11-09"); ns[0].Severity != sevAct {
		t.Fatalf("5 days: %v", ns)
	}
	rows = []compute.CashDay{csRow("2026-11-15", "USD", "1", "-5")}
	if ns := csNotices(rows, "2026-11-09"); ns[0].Severity != sevWarn {
		t.Fatalf("6 days: %v", ns)
	}
	if ns := csNotices(rows, "bad"); ns[0].Severity != sevWarn {
		t.Fatalf("bad day: %v", ns)
	}
}

func TestEvaluateCashShort(t *testing.T) {
	t.Parallel()
	in := cashWith([]compute.NoticeValue{{Tag: "TotalCashValue", Value: "3000", Currency: "USD"}},
		shortOpt("AAA", "2026-11-09", "10", "P", "-1"), shortOpt("BBB", "2026-11-20", "50", "P", "-1"))
	r := eval(in)
	var got []string
	for _, n := range r.Notices {
		got = append(got, n.Code+":"+n.Severity)
	}
	if want := []string{"expiry_today:act", "cash_short:warn"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
	day, ok := parseDay(string(in.Day))
	if want := cashByDate(parseOptions(in.Positions), day, ok, in.Values); !reflect.DeepEqual(r.CashByDate, want) {
		t.Fatalf("got %v want %v", r.CashByDate, want)
	}
}
