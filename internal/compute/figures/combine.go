package figures

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Combine joins the parts by the convention of the rule: vol_blend_v1 is the mean of three
// parts, vol_diff_v1 is parts[0] minus parts[1]. A wrong count or an unknown convention is
// bad_input; a part that is not ok is no_input. DataDay is the newest DataDay of the parts,
// Observations the count of parts, and the result is stale_input when asOf is more than
// MaxAgeDays after DataDay.
func (c calc) Combine(id string, parts []compute.Figure, asOf contracts.Day) compute.Figure {
	rule, f, ok := c.begin(id, asOf)
	if !ok {
		return f
	}
	want := 0
	switch rule.Convention {
	case "vol_blend_v1":
		want = 3
	case "vol_diff_v1":
		want = 2
	}
	if want == 0 || len(parts) != want {
		return missing(f, compute.BadInput)
	}
	f.Observations = len(parts)
	for _, p := range parts {
		if p.Status != compute.StatusOK {
			return missing(f, compute.NoInput)
		}
	}
	for _, p := range parts {
		if p.DataDay > f.DataDay {
			f.DataDay = p.DataDay
		}
	}
	if f, ok = checkAge(f, rule); !ok {
		return f
	}
	if rule.Convention == "vol_diff_v1" {
		f.Value = parts[0].Value - parts[1].Value
	} else {
		sum := 0.0
		for _, p := range parts {
			sum += p.Value
		}
		f.Value = sum / float64(len(parts))
	}
	if math.IsNaN(f.Value) || math.IsInf(f.Value, 0) {
		return missing(f, compute.BadInput)
	}
	return f
}
