package optioneod

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs"
)

func inside() jobs.Need {
	return jobs.Need{Strategy: "s", Group: "watch:options", Rights: []string{"P", "C"}, MinDTE: 14, MaxDTE: 56,
		Ref: "buy_price", Low: "0.80", High: "1.05", Expiry: "all"}
}

func TestCoversEachDimension(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		f    func(n *jobs.Need)
		dim  string // empty: accepted
	}{
		{"inside buy_price", func(n *jobs.Need) {}, ""},
		{"inside spot", func(n *jobs.Need) { n.Ref, n.Low, n.High = "spot", "0.80", "1.05" }, ""},
		{"narrower dte", func(n *jobs.Need) { n.MinDTE, n.MaxDTE = 21, 45 }, ""},
		{"group", func(n *jobs.Need) { n.Group = "watch:other" }, "group"},
		{"index group", func(n *jobs.Need) { n.Group = "index:SPX" }, "group"},
		{"rights", func(n *jobs.Need) { n.Rights = []string{"X"} }, "rights"},
		{"min dte", func(n *jobs.Need) { n.MinDTE = 10 }, "dte"},
		{"max dte", func(n *jobs.Need) { n.MaxDTE = 57 }, "dte"},
		{"expiry", func(n *jobs.Need) { n.Expiry = "monthly" }, "expiry kind"},
		{"trading class", func(n *jobs.Need) { n.TradingClass = "225" }, "trading class"},
		{"buy low", func(n *jobs.Need) { n.Low = "0.79" }, "reach"},
		{"buy high", func(n *jobs.Need) { n.High = "1.06" }, "reach"},
		{"spot low", func(n *jobs.Need) { n.Ref, n.Low, n.High = "spot", "0.79", "1.00" }, "reach"},
		{"spot high", func(n *jobs.Need) { n.Ref, n.Low, n.High = "spot", "1.00", "1.06" }, "reach"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			n := inside()
			c.f(&n)
			err := Covers(n)
			if c.dim == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.dim) || !strings.Contains(err.Error(), `"s"`) {
				t.Fatalf("err %v", err)
			}
		})
	}
}

func TestCoversRefusesAnUnknownRef(t *testing.T) {
	t.Parallel()
	n := inside()
	n.Ref = "delta"
	if err := Covers(n); err == nil || !strings.Contains(err.Error(), "ref") {
		t.Fatalf("err %v", err)
	}
}
