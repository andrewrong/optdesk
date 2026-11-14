package structures_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

func figure(v string) compute.FigureValue { return compute.FigureValue{Value: v} }

func TestCompareOnlySingleShort(t *testing.T) {
	t.Parallel()
	leg := compute.StructureLeg{Right: "P"}
	one := compute.Structure{Legs: []compute.StructureLeg{leg}, CaptureParam: "0.50", ExitDTEParam: "21"}
	if got := (cells{}).compare(one); got == nil || got.CapturedParam != "Earned ratio 50%" || got.ExitDTEParam != "Days before expiry 21" {
		t.Fatalf("single leg compare = %+v", got)
	}
	two := one
	two.Legs = []compute.StructureLeg{leg, leg}
	if (cells{}).compare(two) != nil {
		t.Fatal("two legs have a compare row")
	}
	none := compute.Structure{Legs: []compute.StructureLeg{leg}}
	if (cells{}).compare(none) != nil {
		t.Fatal("a structure with no parameters has a compare row")
	}
}

func TestMoneyCellsAreNumbersOrGaps(t *testing.T) {
	t.Parallel()
	c := cells{}
	if got, ok := c.money(figure("-12.5000")).(usd); !ok || got.Value != "-12.5000" || got.Unit != "usd" {
		t.Fatalf("money = %v", c.money(figure("-12.5000")))
	}
	if got := c.money(figure("x1")); got != mkGap("missing_input") {
		t.Fatalf("bad text = %v", got)
	}
	if got := c.money(compute.FigureValue{Gap: "weird"}); got != mkGap("no_data") {
		t.Fatalf("unknown reason = %v", got)
	}
	naked := compute.Structure{Kind: "naked_call", Figures: compute.StructureFigures{MaxLoss: compute.FigureValue{Gap: "no_rule"}}}
	if g := c.maxLossCell(naked).(gap); g.Text != "No limit" || g.Reason != "no_rule" {
		t.Fatalf("naked call max loss = %v", g)
	}
}

func TestListMoneyTextTwoDecimals(t *testing.T) {
	t.Parallel()
	c := cells{}
	cases := map[string]string{
		"-18295.0000": "−$18,295.00", "-12.5000": "−$12.50", "205.0000": "$205.00", "0.0050": "$0.01",
		"0.0150": "$0.02", "2.675": "$2.68", "7": "$7.00", "-0.0050": "−$0.01",
		"3150.0000": "$3,150.00", "-0.0001": "$0.00",
	}
	for in, want := range cases {
		got, ok := c.listMoney(figure(in)).(usd)
		if !ok || got.Text != want || string(got.Value) != in || got.Unit != "usd" {
			t.Fatalf("listMoney(%s) = %v, want text %s", in, c.listMoney(figure(in)), want)
		}
	}
	if got := c.listMoney(figure("x1")); got != mkGap("missing_input") {
		t.Fatalf("bad text = %v", got)
	}
	if got := c.listMoney(compute.FigureValue{Gap: "no_rule"}); got != mkGap("no_rule") {
		t.Fatalf("gap = %v", got)
	}
	naked := compute.Structure{Kind: "naked_call", Figures: compute.StructureFigures{MaxLoss: compute.FigureValue{Gap: "no_rule"}}}
	if g := c.listMaxLossCell(naked).(gap); g.Text != "No limit" || g.Reason != "no_rule" {
		t.Fatalf("naked call list max loss = %v", g)
	}
}

func TestMoneynessFromDistanceSign(t *testing.T) {
	t.Parallel()
	mk := func(right, dist string) compute.Structure {
		return compute.Structure{Legs: []compute.StructureLeg{{Right: right}}, Figures: compute.StructureFigures{ExpiryDistance: figure(dist)}}
	}
	c := cells{}
	for _, tc := range []struct{ right, dist, want string }{
		{"P", "-0.05", "out of the money"}, {"P", "0.05", "in the money"},
		{"C", "0.05", "out of the money"}, {"C", "-0.05", "in the money"},
		{"P", "0.000000", "at the money"},
	} {
		if got := c.moneyness(mk(tc.right, tc.dist)); got != tc.want {
			t.Errorf("%s %s = %v, want %s", tc.right, tc.dist, got, tc.want)
		}
	}
}
