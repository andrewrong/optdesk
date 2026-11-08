package contracttest

import (
	"encoding/json"
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
)

// Units runs the cases of units.json against u, each with the tolerance of the case.
func Units(t T, u compute.Units) {
	t.Helper()
	scalar := func(key string, fn func(float64) float64) func(T, Case) {
		return func(t T, c Case) {
			var in map[string]float64
			decode(t, c.Input, &in)
			var want float64
			decode(t, c.Expected, &want)
			if got := fn(in[key]); !near(got, want, c.Tolerance) {
				t.Errorf("%s(%v) = %v, want %v (tolerance %v)", c.Group, in[key], got, want, c.Tolerance)
			}
		}
	}
	runCases(t, load(t, "units.json"), map[string]func(T, Case){
		"theta_per_calendar_day": scalar("per_year", u.ThetaPerCalendarDay),
		"vega_per_vol_point":     scalar("per_unit_vol", u.VegaPerVolPoint),
		"iv_decimal":             scalar("percent", u.PercentToDecimal),
		"half_even": func(t T, c Case) {
			var in struct {
				Value float64 `json:"value"`
				Scale int     `json:"scale"`
			}
			decode(t, c.Input, &in)
			var want string
			decode(t, c.Expected, &want)
			got, err := u.RoundHalfEven(in.Value, in.Scale)
			if err != nil {
				t.Errorf("RoundHalfEven(%v, %d): unexpected error %v", in.Value, in.Scale, err)
				return
			}
			if got != want {
				t.Errorf("RoundHalfEven(%v, %d) = %q, want %q", in.Value, in.Scale, got, want)
			}
		},
	})
}

// decode reads raw into v; a case that does not decode fails its subtest.
func decode(t T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

// near reports whether got is within tol of want. NaN equals only NaN, so a warm-up entry
// that is 0 is not near.
func near(got, want, tol float64) bool {
	if math.IsNaN(got) || math.IsNaN(want) {
		return math.IsNaN(got) && math.IsNaN(want)
	}
	return math.Abs(got-want) <= tol
}
