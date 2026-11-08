package contracttest

import (
	"encoding/json"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// dayPoint is one point of a series in a fixture.
type dayPoint struct {
	Day   string  `json:"day"`
	Value float64 `json:"value"`
}

func toSeries(pts []dayPoint) compute.Series {
	s := make(compute.Series, len(pts))
	for i, p := range pts {
		s[i] = compute.Point{Day: contracts.Day(p.Day), Value: p.Value}
	}
	return s
}

// SeriesOps runs the from_series_points, align and log_returns cases of series.json against ops.
func SeriesOps(t T, ops compute.SeriesOps) {
	t.Helper()
	runCases(t, load(t, "series.json"), map[string]func(T, Case){
		"from_series_points": func(t T, c Case) {
			var in struct {
				Points []struct {
					ObservationDate string   `json:"observation_date"`
					RealtimeStart   string   `json:"realtime_start"`
					Status          string   `json:"status"`
					Value           *float64 `json:"value"`
				} `json:"points"`
			}
			decode(t, c.Input, &in)
			pts := make([]contracts.SeriesPoint, len(in.Points))
			for i, p := range in.Points {
				pts[i] = contracts.SeriesPoint{
					ObservationDate: date(t, p.ObservationDate),
					RealtimeStart:   date(t, p.RealtimeStart),
					Status:          p.Status,
					Value:           p.Value,
				}
			}
			var want []dayPoint
			decode(t, c.Expected, &want)
			compareSeries(t, c, "FromSeriesPoints", ops.FromSeriesPoints(pts), want)
		},
		"align": func(t T, c Case) {
			inA, inB := decodeAB(t, c.Input)
			wantA, wantB := decodeAB(t, c.Expected)
			a, b := ops.Align(toSeries(inA), toSeries(inB))
			compareSeries(t, c, "Align a", a, wantA)
			compareSeries(t, c, "Align b", b, wantB)
		},
		"log_returns": func(t T, c Case) {
			var in struct {
				Series []dayPoint `json:"series"`
			}
			decode(t, c.Input, &in)
			var want []dayPoint
			decode(t, c.Expected, &want)
			s := toSeries(in.Series)
			compareSeries(t, c, "LogReturns", ops.LogReturns(s), want)
			vals := ops.Values(s)
			if len(vals) != len(s) {
				t.Fatalf("Values: got %d values for %d points", len(vals), len(s))
			}
			for i, p := range s {
				if vals[i] != p.Value {
					t.Errorf("Values[%d] = %v, want %v", i, vals[i], p.Value)
				}
			}
		},
	})
}

func decodeAB(t T, raw json.RawMessage) (a, b []dayPoint) {
	t.Helper()
	var m struct {
		A []dayPoint `json:"a"`
		B []dayPoint `json:"b"`
	}
	decode(t, raw, &m)
	return m.A, m.B
}

func date(t T, s string) types.SerializableDate {
	t.Helper()
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return types.SerializableDate{Time: d}
}

// compareSeries fails t when got differs from want in length, day or value (by the case's tolerance).
func compareSeries(t T, c Case, what string, got compute.Series, want []dayPoint) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d points, want %d", what, len(got), len(want))
		return
	}
	for i, w := range want {
		if string(got[i].Day) != w.Day || !near(got[i].Value, w.Value, c.Tolerance) {
			t.Errorf("%s[%d] = %s %v, want %s %v (tolerance %v)", what, i, got[i].Day, got[i].Value, w.Day, w.Value, c.Tolerance)
		}
	}
}
