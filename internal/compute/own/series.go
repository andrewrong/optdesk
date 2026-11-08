package own

import (
	"math"
	"sort"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const dateLayout = "2006-01-02"

type seriesOps struct{}

// NewSeriesOps is a compute.NewSeriesOpsFunc.
func NewSeriesOps() compute.SeriesOps { return seriesOps{} }

// FromSeriesPoints keeps, per observation date, the present, non-null point of the latest
// realtime_start (the first of equal vintages); the result is ascending with one point a day.
func (seriesOps) FromSeriesPoints(points []contracts.SeriesPoint) compute.Series {
	type pick struct {
		vintage string
		value   float64
	}
	best := make(map[contracts.Day]pick)
	for _, p := range points {
		if p.Status != "present" || p.Value == nil {
			continue
		}
		day := contracts.Day(p.ObservationDate.Time.Format(dateLayout))
		vintage := p.RealtimeStart.Time.Format(dateLayout)
		if old, ok := best[day]; ok && old.vintage >= vintage {
			continue
		}
		best[day] = pick{vintage, *p.Value}
	}
	out := make(compute.Series, 0, len(best))
	for day, p := range best {
		out = append(out, compute.Point{Day: day, Value: p.value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out
}

// Align keeps the days both series have; both results hold the same days.
func (seriesOps) Align(a, b compute.Series) (compute.Series, compute.Series) {
	outA, outB := compute.Series{}, compute.Series{}
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i].Day < b[j].Day:
			i++
		case a[i].Day > b[j].Day:
			j++
		default:
			outA = append(outA, a[i])
			outB = append(outB, b[j])
			i++
			j++
		}
	}
	return outA, outB
}

// LogReturns is ln(next/previous) of each pair of neighbours, dated by the later point. A
// pair with a value that is not positive has no log return and is left out.
func (seriesOps) LogReturns(s compute.Series) compute.Series {
	out := compute.Series{}
	for i := 1; i < len(s); i++ {
		prev, cur := s[i-1].Value, s[i].Value
		if prev <= 0 || cur <= 0 {
			continue
		}
		out = append(out, compute.Point{Day: s[i].Day, Value: math.Log(cur / prev)})
	}
	return out
}

func (seriesOps) Values(s compute.Series) []float64 {
	out := make([]float64, len(s))
	for i, p := range s {
		out[i] = p.Value
	}
	return out
}
