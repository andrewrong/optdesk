package talibind

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
)

// wrapped is raw with our own corrected function for each one that diverges from TA-Lib
// (see testdata/divergence.json). A function that matches is not wrapped.
type wrapped struct{ raw }

var _ compute.Indicators = wrapped{}

// emaFrom is an exponential moving average of in[start:] with smoothing 2/(p+1), seeded
// with the simple average of in[start : start+p]. The first defined value is at
// start+p-1; the entries before it are NaN. The result has len(in) entries.
func emaFrom(in []float64, start, p int) []float64 {
	out := make([]float64, len(in))
	for i := range out {
		out[i] = math.NaN()
	}
	seed := start + p - 1
	if start < 0 || seed >= len(in) {
		return out
	}
	var sum float64
	for _, v := range in[start : seed+1] {
		sum += v
	}
	k := 2 / float64(p+1)
	prev := sum / float64(p)
	out[seed] = prev
	for i := seed + 1; i < len(in); i++ {
		prev = (in[i]-prev)*k + prev
		out[i] = prev
	}
	return out
}

// MACD follows TA-Lib's seeding, which go-talib does not: the fast average is seeded at
// the first index the slow one is defined (the simple average of the fast period ending
// there), not at the start of the input, and the signal line is seeded on the first
// defined macd values. The three results are NaN before slow+signal-2.
func (w wrapped) MACD(values []float64, fast, slow, signal int) (macd, sig, hist []float64, err error) {
	if err := macdPeriods(fast, slow, signal); err != nil {
		return nil, nil, nil, err
	}
	n := len(values)
	macd, sig, hist = nan(n), nan(n), nan(n)
	fastEMA := emaFrom(values, slow-fast, fast)
	slowEMA := emaFrom(values, 0, slow)
	line := nan(n)
	for i := slow - 1; i < n; i++ {
		line[i] = fastEMA[i] - slowEMA[i]
	}
	signalEMA := emaFrom(line, slow-1, signal)
	for i := slow + signal - 2; i < n; i++ {
		macd[i], sig[i], hist[i] = line[i], signalEMA[i], line[i]-signalEMA[i]
	}
	return macd, sig, hist, nil
}

func nan(n int) []float64 { return pad(make([]float64, n), n) }
