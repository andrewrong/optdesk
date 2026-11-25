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
			EndDay: in.Day, Drawdown: n, NasdaqDrawdown: n, Funds: []marketstateapi.Fund{fund}, Industry: []marketstateapi.Fund{fund},
			History: marketstateapi.History{Sp500Vs200d: line, Drawdown: line, NasdaqDrawdown: line, VIX: line, VIXTerm: line, HighYield: line},
		}
	}
	in := marketstateapi.Input{
		Day: "2026-11-13", SP500: compute.Series{{Day: "2026-11-13", Value: 1}},
		NDX:    compute.Series{{Day: "2026-11-13", Value: 2}},
		Params: marketstateapi.Params{HighSessions: 60},
	}
	r := f(in)
	h := r.History
	if r.EndDay != in.Day || r.Drawdown.Aux != in.Day || r.Funds[0].Rank21.Of != 10 || r.Funds[0].Rank63.Reason != "missing_input" ||
		r.Industry[0].Rotation[0].Y != "2.00" || h.Sp500Vs200d.Total != 9 || h.Drawdown.Points[0].Value != "1" ||
		r.NasdaqDrawdown.Aux != in.Day || h.NasdaqDrawdown.Points[0].Value != "1" || len(in.NDX) != 1 ||
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

// stubIndicators is a caller-side double of compute.Indicators; only the embedded nil interface is needed to fill the field.
type stubIndicators struct{ compute.Indicators }

// TestMarketStateAPIChartLines: the caller sets Indicators and reads the four chart lines; Compute with a nil
// Indicators returns them empty.
func TestMarketStateAPIChartLines(t *testing.T) {
	t.Parallel()
	var f marketstateapi.Func = func(in marketstateapi.Input) marketstateapi.Result {
		if in.Indicators == nil {
			return marketstateapi.Result{}
		}
		l := marketstateapi.Line{Points: []marketstateapi.Point{{Day: in.Day, Value: "5400.00"}}, Total: 1}
		return marketstateapi.Result{History: marketstateapi.History{Close: l, Ma200: l, BandLow: l, BandHigh: l}}
	}
	h := f(marketstateapi.Input{Day: "2026-11-13", Indicators: stubIndicators{}}).History
	if h.Close.Total != 1 || h.Ma200.Points[0].Value != "5400.00" || len(h.BandLow.Points) != 1 || len(h.BandHigh.Points) != 1 {
		t.Fatalf("history %+v", h)
	}

	in := marketstateapi.Input{
		Day: "2026-11-13", SP500: compute.Series{{Day: "2026-11-13", Value: 1}},
		Params: marketstate.TrialParams(),
	}
	h = marketstate.Compute(in).History
	if len(h.Ma200.Points) != 0 || len(h.BandLow.Points) != 0 || len(h.BandHigh.Points) != 0 {
		t.Fatalf("nil Indicators left lines %+v", h)
	}
}
