// Package macro_rates builds the page.macro_rates.v1 view model from stored series points.
// It only formats stored values; it computes nothing, reads no clock and makes no request.
package macro_rates

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	dayLayout = "2006-01-02"

	// freshnessLimitDays is the freshness limit of a series: a weekend plus one holiday.
	freshnessLimitDays = 4
	// lookbackDays is how far before Day each series is read.
	lookbackDays = 30
	// defaultUnit is the unit of a series that has no point in range.
	defaultUnit = "percent"
)

// seriesIDs are the series the page lists (week 1).
var seriesIDs = []string{"DGS10"}

type point struct {
	Date  string   `json:"date"`
	Value *float64 `json:"value"`
}

type series struct {
	ID     string  `json:"id"`
	Unit   string  `json:"unit"`
	Points []point `json:"points"`
}

type gap struct {
	SeriesID string `json:"series_id"`
	Date     string `json:"date"`
	Reason   string `json:"reason"`
}

type viewModel struct {
	SchemaVersion string   `json:"schema_version"`
	GeneratedAt   string   `json:"generated_at"`
	SessionDay    string   `json:"session_day"`
	Series        []series `json:"series"`
	Gaps          []gap    `json:"gaps"`
}

// Build returns the page.macro_rates.v1 JSON for day, read from r. generatedAt is written
// in UTC, RFC 3339. A nil value is a no_value gap at its date; a series whose last point
// is more than the freshness limit before day, or that has no point, is one stale gap at day.
func Build(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error) {
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return nil, fmt.Errorf("macro_rates: bad day %q: %w", day, err)
	}
	from := contracts.Day(d.AddDate(0, 0, -lookbackDays).Format(dayLayout))
	vm := viewModel{
		SchemaVersion: "1",
		GeneratedAt:   generatedAt.UTC().Format(time.RFC3339),
		SessionDay:    string(day),
		Series:        []series{},
		Gaps:          []gap{},
	}
	for _, id := range seriesIDs {
		rows, err := r.SeriesPoints(ctx, id, from, day)
		if err != nil {
			return nil, fmt.Errorf("macro_rates: read %s: %w", id, err)
		}
		s, gaps := format(id, rows, d, day)
		vm.Series = append(vm.Series, s)
		vm.Gaps = append(vm.Gaps, gaps...)
	}
	return json.Marshal(vm)
}

func format(id string, rows []contracts.SeriesPoint, d time.Time, day contracts.Day) (series, []gap) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].ObservationDate.Before(rows[j].ObservationDate.Time)
	})
	s := series{ID: id, Unit: defaultUnit, Points: []point{}}
	var gaps []gap
	stale := len(rows) == 0
	for i, p := range rows {
		date := p.ObservationDate.Format(dayLayout)
		s.Unit = string(p.Unit)
		s.Points = append(s.Points, point{Date: date, Value: p.Value})
		last := i == len(rows)-1
		if last && d.Sub(p.ObservationDate.Time) > freshnessLimitDays*24*time.Hour {
			stale = true
			continue // a stale last point is the stale gap, whatever its value
		}
		if p.Value == nil {
			gaps = append(gaps, gap{SeriesID: id, Date: date, Reason: "no_value"})
		}
	}
	if stale {
		gaps = append(gaps, gap{SeriesID: id, Date: string(day), Reason: "stale"})
	}
	return s, gaps
}
