package contracttest

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
)

// statIn is the input of every case of stats.json; a case sets the keys its group needs.
type statIn struct {
	Values  []float64       `json:"values"`
	X       float64         `json:"x"`
	Window  int             `json:"window"`
	Compare compute.Compare `json:"compare"`
}

// Stats runs the mean, std_dev and percent rank cases of stats.json against s.
func Stats(t T, s compute.Stats) {
	t.Helper()
	one := func(fn func(in statIn) (float64, error)) func(T, Case) {
		return func(t T, c Case) {
			var in statIn
			decode(t, c.Input, &in)
			var want float64
			decode(t, c.Expected, &want)
			got, err := fn(in)
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.Group, err)
				return
			}
			if !near(got, want, c.Tolerance) {
				t.Errorf("%s(%v) = %v, want %v (tolerance %v)", c.Group, in.Values, got, want, c.Tolerance)
			}
		}
	}
	runCases(t, load(t, "stats.json"), map[string]func(T, Case){
		"mean":    one(func(in statIn) (float64, error) { return s.Mean(in.Values) }),
		"std_dev": one(func(in statIn) (float64, error) { return s.StdDev(in.Values) }),
		"percentile_rank": one(func(in statIn) (float64, error) {
			return s.PercentileRank(in.Values, in.X, in.Compare)
		}),
	})
}

// Rolling runs the rolling_* cases of stats.json against r. A warm-up entry is null in the
// fixture and must be NaN.
func Rolling(t T, r compute.Rolling) {
	t.Helper()
	one := func(fn func(in statIn) ([]float64, error)) func(T, Case) {
		return func(t T, c Case) {
			var in statIn
			decode(t, c.Input, &in)
			var raw []*float64
			decode(t, c.Expected, &raw)
			want := make([]float64, len(raw))
			for i, p := range raw {
				want[i] = math.NaN()
				if p != nil {
					want[i] = *p
				}
			}
			got, err := fn(in)
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.Group, err)
				return
			}
			compareFloats(t, c.Group, got, want, c.Tolerance)
		}
	}
	runCases(t, load(t, "stats.json"), map[string]func(T, Case){
		"rolling_mean":    one(func(in statIn) ([]float64, error) { return r.Mean(in.Values, in.Window) }),
		"rolling_std_dev": one(func(in statIn) ([]float64, error) { return r.StdDev(in.Values, in.Window) }),
		"rolling_min":     one(func(in statIn) ([]float64, error) { return r.Min(in.Values, in.Window) }),
		"rolling_max":     one(func(in statIn) ([]float64, error) { return r.Max(in.Values, in.Window) }),
		"rolling_percentile_rank": one(func(in statIn) ([]float64, error) {
			return r.PercentileRank(in.Values, in.Window, in.Compare)
		}),
	})
}

// compareFloats fails t when got differs from want in length or by more than tol at any index.
func compareFloats(t T, what string, got, want []float64, tol float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d values, want %d", what, len(got), len(want))
		return
	}
	for i := range want {
		if !near(got[i], want[i], tol) {
			t.Errorf("%s[%d] = %v, want %v (tolerance %v)", what, i, got[i], want[i], tol)
		}
	}
}
