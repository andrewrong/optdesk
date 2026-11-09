package optionnotice

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func cushionIn(v string) compute.NoticeInput {
	return compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-09",
		Values: []compute.NoticeValue{{Tag: "Cushion", Value: v, Currency: "USD"}}}
}

func TestCushionWarnAndAct(t *testing.T) {
	t.Parallel()
	cases := []struct{ v, want string }{
		{"0.16", ""}, {"0.15", "cushion_low:warn"}, {"0.0500", "cushion_low:act"}, {"0.05", "cushion_low:act"},
		{"0.051", "cushion_low:warn"}, {"-0.01", "cushion_low:act"},
	}
	for _, c := range cases {
		r := eval(cushionIn(c.v))
		got := ""
		if len(r.Notices) == 1 {
			got = r.Notices[0].Code + ":" + r.Notices[0].Severity
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.v, got, c.want)
		}
		if r.Cushion != c.v {
			t.Errorf("%s: Cushion %q", c.v, r.Cushion)
		}
	}
}

func TestCushionAbsentNoNotice(t *testing.T) {
	t.Parallel()
	r := eval(compute.NoticeInput{Day: "2026-11-09", CaptureDay: "2026-11-09",
		Values: []compute.NoticeValue{{Tag: "NetLiquidation", Value: "0.01"}}})
	if len(r.Notices) != 0 || r.Cushion != "" {
		t.Fatalf("got %+v", r)
	}
}
