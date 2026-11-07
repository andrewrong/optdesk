package snapshot

import "math"

// screenSentinel returns the value to store for v: nil for NaN, an infinity and -1 (the broker's "no value"),
// v itself otherwise. It is the one screen for every numeric column, delta and theta included.
func screenSentinel(v *float64) *float64 {
	if v == nil || *v == -1 || math.IsNaN(*v) || math.IsInf(*v, 0) {
		return nil
	}
	return v
}
