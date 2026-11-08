package main

import (
	"context"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// demoReader is the series reader of the demo profile: three typed points of each of the seven
// series of the macro page, with the unit the loader stores (percent for the two rates, index_points
// for the five indices).
type demoReader struct{}

// demoUnits maps each demo series to its stored unit.
var demoUnits = map[string]string{
	"DGS10": "percent", "DGS3MO": "percent",
	"SP500": "index_points", "VIXCLS": "index_points", "VXNCLS": "index_points",
	"RVXCLS": "index_points", "VXVCLS": "index_points",
}

func demoPoint(id, unit, day string, v *float64) contracts.SeriesPoint {
	d, _ := time.Parse(dayLayout, day)
	return contracts.SeriesPoint{
		SeriesID:        id,
		ObservationDate: types.SerializableDate{Time: d},
		Unit:            contracts.SeriesUnit(unit),
		Value:           v,
	}
}

// SeriesPoints returns the demo points of one of the seven series between from and to; other series are empty.
func (demoReader) SeriesPoints(_ context.Context, seriesID string, from, to contracts.Day) ([]contracts.SeriesPoint, error) {
	unit, ok := demoUnits[seriesID]
	if !ok {
		return nil, nil
	}
	a, b := 4.1, 4.2
	if unit == "index_points" {
		a, b = 5000.25, 5010.5
	}
	all := []struct {
		day string
		v   *float64
	}{{"2026-11-02", &a}, {"2026-11-03", nil}, {"2026-11-04", &b}}
	var out []contracts.SeriesPoint
	for _, p := range all {
		if d := contracts.Day(p.day); d >= from && d <= to {
			out = append(out, demoPoint(seriesID, unit, p.day, p.v))
		}
	}
	return out, nil
}
