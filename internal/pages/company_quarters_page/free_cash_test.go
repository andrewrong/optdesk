package company_quarters_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// TestFreeCashCopiedFromCalculator: the free cash flow cell is the Calculator's FreeCash, not a difference.
func TestFreeCashCopiedFromCalculator(t *testing.T) {
	t.Parallel()
	qs := quarters()
	qs[0].OperatingCashFlow, qs[0].Capex = val("5"), val("3")
	qs[0].FreeCash = val("777")
	i, _ := in("US:AAPL", []underlying.Row{aapl}, qs)
	m := build(t, i)
	c := m["quarters"].([]any)[0].(map[string]any)["cells"].(map[string]any)["free_cash"].(map[string]any)
	if c["text"] != "777" {
		t.Fatalf("free cash = %v", c)
	}
}
