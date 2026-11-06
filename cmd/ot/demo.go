package main

import (
	"context"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// demoReader is the series reader of the demo profile: three typed points of DGS10.
type demoReader struct{}

func demoPoint(day string, v *float64) contracts.SeriesPoint {
	d, _ := time.Parse(dayLayout, day)
	return contracts.SeriesPoint{
		SeriesID:        "DGS10",
		ObservationDate: types.SerializableDate{Time: d},
		Unit:            contracts.SeriesUnit("percent"),
		Value:           v,
	}
}

// SeriesPoints returns the demo points of DGS10 between from and to; other series are empty.
func (demoReader) SeriesPoints(_ context.Context, seriesID string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	if seriesID != "DGS10" {
		return nil, nil
	}
	a, b := 4.1, 4.2
	all := []struct {
		day string
		v   *float64
	}{{"2026-11-02", &a}, {"2026-11-03", nil}, {"2026-11-04", &b}}
	var out []contracts.SeriesPoint
	for _, p := range all {
		if d := contracts.Day(p.day); d >= from && d <= to {
			out = append(out, demoPoint(p.day, p.v))
		}
	}
	return out, nil
}
