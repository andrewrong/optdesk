package figures

import (
	"math"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// HistoricalVol is HV(n): the sample stdev of the last N log returns times the square root
// of TradingDays. It needs N+1 closes, all above zero.
func (c calc) HistoricalVol(id string, closes compute.Series, asOf contracts.Day) compute.Figure {
	rule, f, ok := c.begin(id, asOf)
	if !ok {
		return f
	}
	if len(closes) == 0 {
		return missing(f, compute.NoInput)
	}
	f.DataDay = closes[len(closes)-1].Day
	f.Observations = min(len(closes)-1, rule.N)
	if f, ok = checkAge(f, rule); !ok {
		return f
	}
	if rule.N < 2 || len(closes) < rule.N+1 {
		return missing(f, compute.ShortHistory)
	}
	last := closes[len(closes)-rule.N-1:]
	for _, p := range last {
		if !(p.Value > 0) || math.IsInf(p.Value, 0) {
			return missing(f, compute.BadInput)
		}
	}
	sd, err := c.stats.StdDev(c.ops.Values(c.ops.LogReturns(last)))
	if err != nil {
		return missing(f, compute.BadInput)
	}
	f.Value = sd * math.Sqrt(float64(rule.TradingDays))
	return f
}

// MovingAverageGap is the newest close over the mean of the last N closes, minus 1; the mean
// comes from Rolling.Mean.
func (c calc) MovingAverageGap(id string, closes compute.Series, asOf contracts.Day) compute.Figure {
	rule, f, ok := c.begin(id, asOf)
	if !ok {
		return f
	}
	if len(closes) == 0 {
		return missing(f, compute.NoInput)
	}
	f.DataDay = closes[len(closes)-1].Day
	f.Observations = min(len(closes), rule.N)
	if f, ok = checkAge(f, rule); !ok {
		return f
	}
	if rule.N < 1 || len(closes) < rule.N {
		return missing(f, compute.ShortHistory)
	}
	vals := c.ops.Values(closes[len(closes)-rule.N:])
	means, err := c.rolling.Mean(vals, rule.N)
	if err != nil || len(means) == 0 {
		return missing(f, compute.BadInput)
	}
	mean := means[len(means)-1]
	if mean == 0 || math.IsNaN(mean) || math.IsInf(mean, 0) {
		return missing(f, compute.BadInput)
	}
	f.Value = vals[len(vals)-1]/mean - 1
	return f
}

// PercentileInYears is the share of the values in the Years calendar years before the newest
// point, that point excluded, below (or at or below, per the rule) the value of that point.
// Days with no value are skipped. Observations is the count of values in the window.
func (c calc) PercentileInYears(id string, s compute.Series, asOf contracts.Day) compute.Figure {
	rule, f, ok := c.begin(id, asOf)
	if !ok {
		return f
	}
	if len(s) == 0 {
		return missing(f, compute.NoInput)
	}
	today := s[len(s)-1]
	f.DataDay = today.Day
	if f, ok = checkAge(f, rule); !ok {
		return f
	}
	day, err := time.Parse(time.DateOnly, string(today.Day))
	if err != nil {
		return missing(f, compute.BadInput)
	}
	start := contracts.Day(day.AddDate(-rule.Years, 0, 0).Format(time.DateOnly))
	var window []float64
	for _, p := range s[:len(s)-1] {
		if p.Day >= start {
			window = append(window, p.Value)
		}
	}
	f.Observations = len(window)
	if s[0].Day > start {
		return missing(f, compute.ShortHistory)
	}
	if len(window) == 0 {
		return missing(f, compute.NoInput)
	}
	v, err := c.stats.PercentileRank(window, today.Value, rule.Compare)
	if err != nil {
		return missing(f, compute.BadInput)
	}
	f.Value = v
	return f
}

// Ratio is num over den, both at their newest day, which must be the same day; a zero
// denominator is bad_input. DataDay is the newest day of either input.
func (c calc) Ratio(id string, num, den compute.Series, asOf contracts.Day) compute.Figure {
	rule, f, ok := c.begin(id, asOf)
	if !ok {
		return f
	}
	if len(num) == 0 || len(den) == 0 {
		return missing(f, compute.NoInput)
	}
	n, d := num[len(num)-1], den[len(den)-1]
	f.DataDay = max(n.Day, d.Day)
	f.Observations = 1
	for _, in := range []compute.Point{n, d} {
		isStale, valid := stale(in.Day, asOf, rule.MaxAgeDays)
		if !valid {
			return missing(f, compute.BadInput)
		}
		if isStale {
			return missing(f, compute.StaleInput)
		}
	}
	if n.Day != d.Day || d.Value == 0 {
		return missing(f, compute.BadInput)
	}
	f.Value = n.Value / d.Value
	return f
}
