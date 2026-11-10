package valuation

import (
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
)

func TestImpliedGrowthGolden(t *testing.T) {
	t.Parallel()
	for _, c := range loadFcfGolden(t) {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			want := c.Expected.Reverse
			if want.State != "ok" {
				got := ImpliedGrowth(filingsOf(c), "100")
				if got.State != valuationapi.StateMissingInput {
					t.Fatalf("state %q, want missing_input", got.State)
				}
				if !reflect.DeepEqual(got.Reasons, plainReasons(want.Reasons)) {
					t.Errorf("reasons %v, want %v", got.Reasons, plainReasons(want.Reasons))
				}
				if got.Primary != "" || len(got.Rates) != 0 {
					t.Errorf("a refused result holds growth: %+v", got)
				}
				return
			}
			if len(want.Implied) == 0 {
				t.Fatal("no spots in the golden case")
			}
			for _, sp := range want.Implied {
				got := ImpliedGrowth(filingsOf(c), sp.Spot)
				if len(got.Rates) != len(sp.Rates) {
					t.Fatalf("spot %s: %d rates, want %d", sp.Spot, len(got.Rates), len(sp.Rates))
				}
				var reasons []string
				for i, r := range sp.Rates {
					wantGrowth := ""
					if r.Growth != nil {
						wantGrowth = *r.Growth
					} else {
						reasons = append(reasons, r.Status+":"+r.Rate)
					}
					if got.Rates[i].Rate != r.Rate || got.Rates[i].Growth != wantGrowth {
						t.Errorf("spot %s rate %s: %+v, want growth %q", sp.Spot, r.Rate, got.Rates[i], wantGrowth)
					}
					if r.Rate == "0.10" && got.Primary != wantGrowth {
						t.Errorf("spot %s: primary %q, want %q", sp.Spot, got.Primary, wantGrowth)
					}
				}
				if len(reasons) == 0 {
					reasons = nil
				}
				if !reflect.DeepEqual(got.Reasons, reasons) {
					t.Errorf("spot %s: reasons %v, want %v", sp.Spot, got.Reasons, reasons)
				}
				wantState := valuationapi.StateOK
				if got.Primary == "" {
					wantState = valuationapi.StateMissingInput
				}
				if got.State != wantState {
					t.Errorf("spot %s: state %q, want %q", sp.Spot, got.State, wantState)
				}
			}
		})
	}
}

func TestImpliedGrowthBadSpot(t *testing.T) {
	t.Parallel()
	c := loadFcfGolden(t)[0]
	for _, spot := range []string{"", "abc", "0", "-5"} {
		got := ImpliedGrowth(filingsOf(c), spot)
		if got.State != valuationapi.StateMissingInput || !reflect.DeepEqual(got.Reasons, []string{reasonBadSpot}) {
			t.Errorf("spot %q: %+v", spot, got)
		}
	}
}
