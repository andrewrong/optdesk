package valuation

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuationapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestRefusalsGolden(t *testing.T) {
	t.Parallel()
	n := 0
	for _, c := range loadReturnGolden(t) {
		if c.Expected.State != "refused" {
			continue
		}
		n++
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			res := ReturnAtPrice(filingsOfReturn(c), c.Input.Costs, nil, contracts.Day(c.Input.PriceDate))
			if res.State != valuationapi.StateMissingInput || len(res.Reasons) != 1 || res.Reasons[0] != c.Expected.Reason {
				t.Fatalf("state %q reasons %v, want missing_input %s", res.State, res.Reasons, c.Expected.Reason)
			}
			if len(res.Lines) != 0 {
				t.Errorf("a refusal holds lines: %+v", res.Lines)
			}
		})
	}
	if n != 6 {
		t.Fatalf("%d refusal cases, want 6", n)
	}
}

func TestSpendingGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		omit string // reason the earnings lines (2 and 4) are left out; "" when formed
	}{
		{"steady_no_analyst", ""},
		{"gate_spending_omits_earnings", "GATE_SPENDING"},
		{"r_below_min_omits_earnings", "R_BELOW_MIN"},
		{"plant_not_reported", ""}, // plant reported in no year marks the result, it omits nothing
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := goldenCase(t, tc.name)
			res := ReturnAtPrice(filingsOfReturn(c), c.Input.Costs[:1], nil, contracts.Day(c.Input.PriceDate))
			if res.State != valuationapi.StateOK || len(res.Lines) != 4 {
				t.Fatalf("state %q, %d lines", res.State, len(res.Lines))
			}
			earns := res.Lines[1]
			if earns.Key != valuationapi.LineSpendingEarns {
				t.Fatalf("line 2 is %s", earns.Key)
			}
			if tc.omit == "" {
				if earns.State != valuationapi.StateOK {
					t.Errorf("earnings line %+v, want formed", earns)
				}
			} else if earns.State != valuationapi.StateMissingInput || earns.Reason != tc.omit {
				t.Errorf("earnings line %+v, want left out for %s", earns, tc.omit)
			}
			if res.Lines[0].State != valuationapi.StateOK {
				t.Errorf("the strict line is not formed: %+v", res.Lines[0])
			}
		})
	}
}
