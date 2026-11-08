package own

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
)

type rolling struct{ stats compute.Stats }

// NewRolling is a compute.NewRollingFunc: the window loop over the injected Stats.
func NewRolling(stats compute.Stats) compute.Rolling { return rolling{stats} }

// window runs fn on each full window of values (the window ends at the entry it fills);
// the entries before the first full window are NaN. A window under 1 is Permanent.
func window(values []float64, w int, fn func(win []float64) (float64, error)) ([]float64, error) {
	if w < 1 {
		return nil, permanent("rolling: window under 1")
	}
	out := make([]float64, len(values))
	for i := range out {
		if i < w-1 {
			out[i] = math.NaN()
			continue
		}
		v, err := fn(values[i-w+1 : i+1])
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (r rolling) Mean(values []float64, w int) ([]float64, error) {
	return window(values, w, r.stats.Mean)
}

func (r rolling) StdDev(values []float64, w int) ([]float64, error) {
	return window(values, w, r.stats.StdDev)
}

func (rolling) Min(values []float64, w int) ([]float64, error) {
	return window(values, w, func(win []float64) (float64, error) {
		m := win[0]
		for _, v := range win[1:] {
			m = math.Min(m, v)
		}
		return m, nil
	})
}

func (rolling) Max(values []float64, w int) ([]float64, error) {
	return window(values, w, func(win []float64) (float64, error) {
		m := win[0]
		for _, v := range win[1:] {
			m = math.Max(m, v)
		}
		return m, nil
	})
}

// PercentileRank ranks the newest value of each window within the window.
func (r rolling) PercentileRank(values []float64, w int, cmp compute.Compare) ([]float64, error) {
	return window(values, w, func(win []float64) (float64, error) {
		return r.stats.PercentileRank(win, win[len(win)-1], cmp)
	})
}
