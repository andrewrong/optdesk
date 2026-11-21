package marketstate

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/marketstateapi"
)

const (
	maPeriod   = 200 // the average line
	bandPeriod = 20  // the Bollinger window
	bandDev    = 2.0 // standard deviations each side
)

// chartLines is the close, 200-day simple average and Bollinger band (20, 2, population sd) over the whole SP500
// series, each cut to the chart window by line(). A day whose value is NaN has no point; a nil Indicators leaves
// the average and band empty and a nil Units leaves all four empty.
func chartLines(in marketstateapi.Input) (closeLine, ma200, bandLow, bandHigh marketstateapi.Line) {
	if in.Units == nil || len(in.SP500) == 0 {
		return
	}
	closes := make([]float64, len(in.SP500))
	for i, pt := range in.SP500 {
		closes[i] = pt.Value
	}
	closeLine = valueLine(in, closes)
	if in.Indicators == nil {
		return
	}
	if ma, err := in.Indicators.SMA(closes, maPeriod); err == nil {
		ma200 = valueLine(in, ma)
	}
	if up, _, low, err := in.Indicators.BBands(closes, bandPeriod, bandDev, bandDev, compute.MASMA); err == nil {
		bandLow, bandHigh = valueLine(in, low), valueLine(in, up)
	}
	return
}

// valueLine is values (one per SP500 day) as a window line of 2-decimal text.
func valueLine(in marketstateapi.Input, values []float64) marketstateapi.Line {
	return line(in.SP500, in.Params, func(i int) (string, bool) {
		v := values[i]
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", false
		}
		text, err := in.Units.RoundHalfEven(v, 2)
		return text, err == nil
	})
}
