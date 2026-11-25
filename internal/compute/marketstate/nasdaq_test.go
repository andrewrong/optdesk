package marketstate_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

func ndxSeries80() compute.Series {
	return series(80, func(i int) float64 {
		if i <= 70 {
			return 2000 + float64(i)
		}
		return 2070 - float64(i-70)
	})
}

func TestNasdaqDrawdown(t *testing.T) {
	t.Parallel()
	in := marketstateapi.Input{Day: dayN(79), NDX: ndxSeries80(), Params: marketstate.TrialParams()}
	r := marketstate.Compute(in)
	d := r.NasdaqDrawdown
	if d.Value != "-0.43" || d.Aux != dayN(70) || d.Day != dayN(79) || d.Reason != "" {
		t.Fatalf("nasdaq drawdown %+v", d)
	}
	if n := len(r.History.NasdaqDrawdown.Points); n != 21 || r.History.NasdaqDrawdown.Points[n-1].Value != "-0.43" {
		t.Fatalf("history %+v", r.History.NasdaqDrawdown)
	}
	in.NDX = series(80, func(i int) float64 { return 2000 + float64(i) })
	d = marketstate.Compute(in).NasdaqDrawdown
	if d.Value != "0.00" || d.Aux != dayN(79) || d.Reason != "" {
		t.Fatalf("at high %+v", d)
	}
}

func TestNasdaqDrawdownGaps(t *testing.T) {
	t.Parallel()
	s := ndxSeries80()
	in := marketstateapi.Input{Day: dayN(79), Params: marketstate.TrialParams()}
	if d := marketstate.Compute(in).NasdaqDrawdown; d.Reason != compute.NoInput || d.Value != "" {
		t.Fatalf("empty %+v", d)
	}
	in.NDX = s
	in.Day = dayN(79 + in.Params.MaxAgeDays + 1)
	if d := marketstate.Compute(in).NasdaqDrawdown; d.Reason != compute.StaleInput || d.Value != "" {
		t.Fatalf("stale %+v", d)
	}
	in.NDX, in.Day = s[:59], dayN(58)
	if d := marketstate.Compute(in).NasdaqDrawdown; d.Reason != compute.ShortHistory {
		t.Fatalf("short %+v", d)
	}
}

func TestNasdaqKeepsSP500(t *testing.T) {
	t.Parallel()
	in := marketstateapi.Input{Day: dayN(79), SP500: ndxSeries80(), Params: marketstate.TrialParams()}
	base := marketstate.Compute(in)
	in.NDX = series(80, func(i int) float64 { return 9000 - float64(i) })
	with := marketstate.Compute(in)
	if with.Drawdown != base.Drawdown || len(with.History.Drawdown.Points) != len(base.History.Drawdown.Points) {
		t.Fatalf("sp500 changed %+v vs %+v", with.Drawdown, base.Drawdown)
	}
	if with.NasdaqDrawdown.Value == "" {
		t.Fatalf("nasdaq missing")
	}
}
