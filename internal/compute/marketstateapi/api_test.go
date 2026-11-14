package marketstateapi_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/liquidityapi"
	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

// TestMarketStateFuncContract: a fake Func is handed an Input and the caller reads every field of Result.
func TestMarketStateFuncContract(t *testing.T) {
	t.Parallel()
	var f marketstateapi.Func = func(in marketstateapi.Input) marketstateapi.Result {
		n := liquidityapi.Num{Value: "-1.00", Day: in.Day, Aux: in.Day}
		fund := marketstateapi.Fund{
			Symbol: "XLK", Name: "Technology", Gap21: n, Gap63: n,
			Rank21: marketstateapi.Position{At: 1, Of: 10}, Rank63: marketstateapi.Position{Reason: liquidityapi.MissingInput},
			Rotation: []marketstateapi.Spot{{Day: in.Day, X: "1.00", Y: "2.00"}},
		}
		line := marketstateapi.Line{Points: []marketstateapi.Point{{Day: in.Day, Value: "1"}}, Total: 9}
		return marketstateapi.Result{
			EndDay: in.Day, Drawdown: n, Funds: []marketstateapi.Fund{fund}, Industry: []marketstateapi.Fund{fund},
			History: marketstateapi.History{Sp500Vs200d: line, Drawdown: line, VIX: line, VIXTerm: line, HighYield: line},
		}
	}
	in := marketstateapi.Input{
		Day: "2026-11-13", SP500: compute.Series{{Day: "2026-11-13", Value: 1}},
		Params: marketstateapi.Params{HighSessions: 60},
	}
	r := f(in)
	h := r.History
	if r.EndDay != in.Day || r.Drawdown.Aux != in.Day || r.Funds[0].Rank21.Of != 10 || r.Funds[0].Rank63.Reason != "missing_input" ||
		r.Industry[0].Rotation[0].Y != "2.00" || h.Sp500Vs200d.Total != 9 || h.Drawdown.Points[0].Value != "1" ||
		len(h.VIX.Points) != 1 || len(h.VIXTerm.Points) != 1 || len(h.HighYield.Points) != 1 {
		t.Fatalf("result %+v", r)
	}
}

// TestMarketStateComputeAssignableToFunc: the implementation satisfies the declared type.
func TestMarketStateComputeAssignableToFunc(t *testing.T) {
	t.Parallel()
	var f marketstateapi.Func = marketstate.Compute
	in := marketstateapi.Input{Day: "2026-11-13", Params: marketstate.TrialParams()}
	if r := f(in); r.Drawdown.Reason != compute.NoInput || r.EndDay != "" {
		t.Fatalf("empty input %+v", r)
	}
}
