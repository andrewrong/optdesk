package optionnotice

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func shortOpt(sym, expiry, strike, right, qty string) compute.NoticePosition {
	return compute.NoticePosition{Symbol: sym, SecType: "OPT", Expiry: expiry, Strike: strike,
		Right: right, Multiplier: "100", Quantity: qty, Currency: "USD"}
}

func stock(sym, qty string) compute.NoticePosition {
	return compute.NoticePosition{Symbol: sym, SecType: "STK", Quantity: qty, Currency: "USD"}
}

func eval(in compute.NoticeInput) compute.NoticeResult {
	return New().Evaluate(in, DefaultParams())
}

func codes(r compute.NoticeResult) []string {
	var out []string
	for _, n := range r.Notices {
		out = append(out, n.Code+":"+n.Severity)
	}
	return out
}

func factOf(n compute.Notice, label string) string {
	for _, f := range n.Facts {
		if f.Label == label {
			return f.Value
		}
	}
	return ""
}

func TestCaptureStaleInfo(t *testing.T) {
	t.Parallel()
	// 2026-11-09 is a Monday: Friday's capture is one session day old, Thursday's is two.
	fresh := eval(compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-06"})
	if len(fresh.Notices) != 0 {
		t.Fatalf("friday capture on monday: %v", codes(fresh))
	}
	stale := eval(compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-05"})
	if got := codes(stale); !reflect.DeepEqual(got, []string{"capture_stale:info"}) {
		t.Fatalf("got %v", got)
	}
}

func TestOrderSeverityExpiryContract(t *testing.T) {
	t.Parallel()
	in := compute.NoticeInput{
		Day: "2026-11-09", CaptureDay: "2026-11-09",
		Positions: []compute.NoticePosition{
			shortOpt("ZZZ", "2026-11-12", "10", "P", "-1"),
			shortOpt("AAA", "2026-11-12", "10", "P", "-1"),
			shortOpt("BBB", "2026-11-11", "10", "P", "-1"),
			shortOpt("CCC", "2026-11-09", "10", "P", "-1"),
		},
		Values: []compute.NoticeValue{{Tag: "Cushion", Value: "0.01"}},
	}
	var got []string
	for _, n := range eval(in).Notices {
		got = append(got, n.Code+":"+n.Severity+":"+n.Symbol)
	}
	want := []string{
		"expiry_today:act:CCC",
		"cushion_low:act:",
		"expiry_near:warn:BBB",
		"expiry_near:warn:AAA",
		"expiry_near:warn:ZZZ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestSameInputSameResult(t *testing.T) {
	t.Parallel()
	d := contracts.Day("2026-11-20")
	in := compute.NoticeInput{
		Day: "2026-11-09", CaptureDay: "2026-11-09",
		Positions: []compute.NoticePosition{shortOpt("AAA", "2026-11-11", "10", "C", "-1"), shortOpt("BBB", "2026-11-13", "5", "P", "-2")},
		Guards:    []compute.NoticeGuard{{Symbol: "AAA", Expiry: "2026-11-11", Strike: "10", Status: "blocked", Reasons: []string{"ex_dividend"}}},
		Earnings:  []compute.NoticeEarnings{{Symbol: "BBB", Next: &d}},
	}
	a, b := eval(in), eval(in)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("results differ:\n%v\n%v", a, b)
	}
	if a.RuleVersion != 1 {
		t.Fatalf("rule version %d", a.RuleVersion)
	}
}

func TestBadDecimalIsSkippedNotPanic(t *testing.T) {
	t.Parallel()
	in := compute.NoticeInput{
		Day: "2026-11-09", CaptureDay: "bad",
		Positions: []compute.NoticePosition{
			shortOpt("AAA", "2026-11-11", "x", "P", "-1"),
			shortOpt("BBB", "2026-11-11", "10", "P", "?"),
			shortOpt("CCC", "not-a-day", "10", "P", "-1"),
			{Symbol: "DDD", SecType: "OPT", Expiry: "2026-11-11", Strike: "10", Right: "P", Multiplier: "", Quantity: "-1"},
		},
		PrevDay:       "2026-11-06",
		PrevPositions: []compute.NoticePosition{shortOpt("EEE", "2026-11-09", "1e", "P", "-1")},
		Values:        []compute.NoticeValue{{Tag: "Cushion", Value: "n/a"}},
	}
	r := eval(in)
	for _, n := range r.Notices {
		if n.Code == "cushion_low" || n.Code == "assignment_seen" {
			t.Fatalf("unexpected %v", n)
		}
	}
	if len(r.CashByDate) != 0 {
		t.Fatalf("cash from bad rows: %v", r.CashByDate)
	}
}
