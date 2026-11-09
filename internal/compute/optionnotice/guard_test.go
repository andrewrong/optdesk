package optionnotice

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func callIn(status string) compute.NoticeInput {
	return compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-09",
		Positions: []compute.NoticePosition{shortOpt("AAA", "2026-12-18", "50", "C", "-1")},
		Guards: []compute.NoticeGuard{{Symbol: "AAA", Expiry: "2026-12-18", Strike: "50.0", Status: status,
			Reasons: []string{"ex_dividend", "deep_itm"}}}}
}

func TestCallGuardFromBlockedRow(t *testing.T) {
	t.Parallel()
	r := eval(callIn("blocked"))
	if got := codes(r); !reflect.DeepEqual(got, []string{"call_guard:warn"}) {
		t.Fatalf("got %v", got)
	}
	var reasons []string
	for _, f := range r.Notices[0].Facts {
		if f.Label == "reason" {
			reasons = append(reasons, f.Value)
		}
	}
	if !reflect.DeepEqual(reasons, []string{"ex_dividend", "deep_itm"}) {
		t.Fatalf("reasons %v", reasons)
	}
}

func TestCallGuardOkRowNone(t *testing.T) {
	t.Parallel()
	if got := codes(eval(callIn("ok"))); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	// a blocked row of a call that is not held gives nothing
	in := callIn("blocked")
	in.Positions = nil
	if got := codes(eval(in)); len(got) != 0 {
		t.Fatalf("not held: %v", got)
	}
}

func earnIn(next *contracts.Day) compute.NoticeInput {
	return compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-09",
		Positions: []compute.NoticePosition{shortOpt("AAA", "2026-12-18", "50", "P", "-1")},
		Earnings:  []compute.NoticeEarnings{{Symbol: "AAA", Next: next}}}
}

func TestEarningsInsideBeforeExpiry(t *testing.T) {
	t.Parallel()
	for _, d := range []contracts.Day{"2026-11-20", "2026-12-18"} {
		d := d
		r := eval(earnIn(&d))
		if got := codes(r); !reflect.DeepEqual(got, []string{"earnings_inside:warn"}) {
			t.Fatalf("%s: got %v", d, got)
		}
		if v := factOf(r.Notices[0], "earnings"); v != string(d) {
			t.Fatalf("fact %q", v)
		}
	}
}

func TestEarningsAfterExpiryNone(t *testing.T) {
	t.Parallel()
	d := contracts.Day("2026-12-19")
	if got := codes(eval(earnIn(&d))); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	past := contracts.Day("2026-11-01")
	if got := codes(eval(earnIn(&past))); len(got) != 0 {
		t.Fatalf("past earnings: %v", got)
	}
}

func TestEarningsMissingNone(t *testing.T) {
	t.Parallel()
	if got := codes(eval(earnIn(nil))); len(got) != 0 {
		t.Fatalf("nil next: %v", got)
	}
	in := earnIn(nil)
	in.Earnings = nil
	if got := codes(eval(in)); len(got) != 0 {
		t.Fatalf("no row: %v", got)
	}
}
