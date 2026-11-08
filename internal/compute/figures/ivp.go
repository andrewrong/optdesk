package figures

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// ivWindow gives the figure start (DataDay and Observations set), today's value and the
// window of valid sessions before today: the last rule.Window finite values, today excluded.
// ok is false when f is already a missing Figure to return.
func (c calc) ivWindow(id string, s compute.Series, asOf contracts.Day) (compute.Figure, float64, []float64, bool) {
	rule, f, ok := c.begin(id, asOf)
	if !ok {
		return f, 0, nil, false
	}
	if len(s) == 0 {
		return missing(f, compute.NoInput), 0, nil, false
	}
	today := s[len(s)-1]
	f.DataDay = today.Day
	if f, ok = checkAge(f, rule); !ok {
		return f, 0, nil, false
	}
	var win []float64
	for _, p := range s[:len(s)-1] {
		if !math.IsNaN(p.Value) && !math.IsInf(p.Value, 0) {
			win = append(win, p.Value)
		}
	}
	if rule.Window > 0 && len(win) > rule.Window {
		win = win[len(win)-rule.Window:]
	}
	f.Observations = len(win)
	switch {
	case math.IsNaN(today.Value) || math.IsInf(today.Value, 0):
		return missing(f, compute.BadInput), 0, nil, false
	case len(win) < rule.MinValid || len(win) == 0:
		return missing(f, compute.ShortHistory), 0, nil, false
	}
	return f, today.Value, win, true
}

// IvPercentile is the share of the past valid sessions whose IV30 is at or below today's,
// today excluded (ivp_v1, Decision 9: ties count, whatever Compare the rule carries). It is
// short_history under MinValid sessions.
func (c calc) IvPercentile(id string, iv30 compute.Series, asOf contracts.Day) compute.Figure {
	f, today, win, ok := c.ivWindow(id, iv30, asOf)
	if !ok {
		return f
	}
	v, err := c.stats.PercentileRank(win, today, compute.AtOrBelow)
	if err != nil {
		return missing(f, compute.BadInput)
	}
	f.Value = v
	return f
}

// IvRank is (IV - min) / (max - min) over the same window as IvPercentile, today excluded;
// zero_range when max equals min.
func (c calc) IvRank(id string, iv30 compute.Series, asOf contracts.Day) compute.Figure {
	f, today, win, ok := c.ivWindow(id, iv30, asOf)
	if !ok {
		return f
	}
	lo, hi := win[0], win[0]
	for _, v := range win[1:] {
		lo, hi = math.Min(lo, v), math.Max(hi, v)
	}
	if hi == lo {
		return missing(f, compute.ZeroRange)
	}
	f.Value = (today - lo) / (hi - lo)
	return f
}
