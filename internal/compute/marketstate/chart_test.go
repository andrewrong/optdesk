package marketstate_test

import (
	"math"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/marketstate"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/compute/talibind"
)

func chartInput(n int, value func(i int) float64) marketstateapi.Input {
	return marketstateapi.Input{
		SP500: series(n, value), Units: own.NewUnits(), Indicators: talibind.New(), Params: marketstate.TrialParams(),
	}
}

func TestChartLinesPopulationSD(t *testing.T) {
	t.Parallel()
	h := marketstate.Compute(chartInput(250, func(i int) float64 { return 100 + float64(i%7) })).History
	if len(h.Close.Points) != 250 {
		t.Fatalf("close points = %d, want 250", len(h.Close.Points))
	}
	if len(h.Ma200.Points) != 51 {
		t.Fatalf("average points = %d, want 51", len(h.Ma200.Points))
	}
	if len(h.BandLow.Points) != 231 || len(h.BandHigh.Points) != 231 {
		t.Fatalf("band points = %d, %d, want 231", len(h.BandLow.Points), len(h.BandHigh.Points))
	}
	if got, want := h.BandLow.Points[0].Day, dayN(19); got != want {
		t.Fatalf("first band day = %s, want the 20th close %s", got, want)
	}
	if got, want := h.Ma200.Points[0].Day, dayN(199); got != want {
		t.Fatalf("first average day = %s, want %s", got, want)
	}

	// Closes 1..20: mean 10.5, population sd sqrt((20^2-1)/12) = 5.76628, so the band is 10.5 -+ 11.53256.
	// The sample sd (5.91608) would give 22.33 at the top.
	h = marketstate.Compute(chartInput(20, func(i int) float64 { return float64(i + 1) })).History
	if len(h.BandLow.Points) != 1 || len(h.BandHigh.Points) != 1 {
		t.Fatalf("band points = %d, %d, want 1", len(h.BandLow.Points), len(h.BandHigh.Points))
	}
	if got := h.BandLow.Points[0].Value; got != "-1.03" {
		t.Errorf("band low = %s, want -1.03", got)
	}
	if got := h.BandHigh.Points[0].Value; got != "22.03" {
		t.Errorf("band high = %s, want 22.03", got)
	}
	if got := h.Close.Points[19].Value; got != "20.00" {
		t.Errorf("close = %s, want 20.00", got)
	}
	if len(h.Ma200.Points) != 0 {
		t.Errorf("average points = %d, want none before 200 closes", len(h.Ma200.Points))
	}
}

func TestChartLinesNeedIndicatorsAndUnits(t *testing.T) {
	t.Parallel()
	in := chartInput(250, func(i int) float64 { return 100 + float64(i%7) })
	in.Indicators = nil
	h := marketstate.Compute(in).History
	if len(h.Ma200.Points)+len(h.BandLow.Points)+len(h.BandHigh.Points) != 0 {
		t.Errorf("nil Indicators gave average or band points")
	}
	in = chartInput(250, func(i int) float64 { return 100 })
	in.Units = nil
	h = marketstate.Compute(in).History
	if len(h.Close.Points)+len(h.Ma200.Points)+len(h.BandLow.Points)+len(h.BandHigh.Points) != 0 {
		t.Errorf("nil Units gave points")
	}
}

func TestChartLinesSkipNaNClose(t *testing.T) {
	t.Parallel()
	in := chartInput(30, func(i int) float64 { return float64(i + 1) })
	in.SP500[25].Value = math.NaN()
	h := marketstate.Compute(in).History
	for _, p := range h.Close.Points {
		if p.Day == dayN(25) {
			t.Errorf("NaN close has a point")
		}
	}
}
