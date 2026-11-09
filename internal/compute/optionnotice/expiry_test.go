package optionnotice

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func expiryIn(day, expiry, qty string) compute.NoticeInput {
	return compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-09",
		Positions: []compute.NoticePosition{shortOpt("AAA", expiry, "50", "P", qty)}}
}

func TestExpiryTodayIsAct(t *testing.T) {
	t.Parallel()
	r := eval(expiryIn("", "2026-11-09", "-1"))
	if got := codes(r); !reflect.DeepEqual(got, []string{"expiry_today:act"}) {
		t.Fatalf("got %v", got)
	}
	n := r.Notices[0]
	if n.Symbol != "AAA" || n.Expiry != "2026-11-09" || n.Contract != "AAA 2026-11-09 50 P" {
		t.Fatalf("notice %+v", n)
	}
}

func TestExpiryNearWithinDays(t *testing.T) {
	t.Parallel()
	r := eval(expiryIn("", "2026-11-14", "-1"))
	if got := codes(r); !reflect.DeepEqual(got, []string{"expiry_near:warn"}) {
		t.Fatalf("got %v", got)
	}
	if v := factOf(r.Notices[0], "days"); v != "5" {
		t.Fatalf("days fact %q", v)
	}
}

func TestExpiryBeyondDaysNone(t *testing.T) {
	t.Parallel()
	if got := codes(eval(expiryIn("", "2026-11-15", "-1"))); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
	if got := codes(eval(expiryIn("", "2026-11-08", "-1"))); len(got) != 0 {
		t.Fatalf("past expiry: %v", got)
	}
}

func TestLongPositionNoNotice(t *testing.T) {
	t.Parallel()
	if got := codes(eval(expiryIn("", "2026-11-09", "3"))); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
