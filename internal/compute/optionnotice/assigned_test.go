package optionnotice

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func assignedIn(stockNow string) compute.NoticeInput {
	return compute.NoticeInput{
		Day: "2026-11-09", CaptureDay: "2026-11-09",
		PrevDay:       "2026-11-06",
		PrevPositions: []compute.NoticePosition{shortOpt("AAA", "2026-11-09", "50", "P", "-2"), stock("AAA", "0")},
		Positions:     []compute.NoticePosition{stock("AAA", stockNow)},
	}
}

func TestAssignmentSeen(t *testing.T) {
	t.Parallel()
	r := eval(assignedIn("200"))
	if got := codes(r); !reflect.DeepEqual(got, []string{"assignment_seen:act"}) {
		t.Fatalf("got %v", got)
	}
	n := r.Notices[0]
	if v := factOf(n, "cash"); v != "10000.00" {
		t.Fatalf("cash %q", v)
	}
	if n.Symbol != "AAA" || n.Expiry != "2026-11-09" {
		t.Fatalf("notice %+v", n)
	}
}

func TestAssignmentNeedsStockRise(t *testing.T) {
	t.Parallel()
	if got := codes(eval(assignedIn("0"))); len(got) != 0 {
		t.Fatalf("no rise: %v", got)
	}
	// the option is still held on its expiry day: not assigned
	in := assignedIn("200")
	in.Positions = append(in.Positions, shortOpt("AAA", "2026-11-09", "50", "P", "-2"))
	for _, c := range codes(eval(in)) {
		if c == "assignment_seen:act" {
			t.Fatal("still held")
		}
	}
}
