package macro_page

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/macro-page-v2.md#decisions 5, 6 and 10.

const (
	// monthlyFreshnessDays is the freshness limit of a monthly series.
	monthlyFreshnessDays = 45
	// weeklyFreshnessDays is the freshness limit of a weekly series: published on the Wednesday after the week's Friday.
	weeklyFreshnessDays = 10
	// changeBack is the number of observations a daily change looks back.
	changeBack = 5
	// groupLookbackDays is how far before the page day a grouped series is read: a monthly series needs two points.
	groupLookbackDays = 120

	unitPercent = "percent"
	unitIndex   = "index_points"
	unitJPY     = "jpy_per_usd"
)

// Group is one group of the Indicators card: Key is rates, credit or dollar_japan, Indicators its rows in order.
type Group struct {
	Key        string
	Indicators []Indicator
}

// seriesSpec is how the producer prints a series: its unit, whether it is monthly or weekly, and its Chinese hint.
type seriesSpec struct {
	unit    string
	monthly bool
	weekly  bool
	hint    string
}

// seriesSpecs is the one table of the grouped series. A series not in it is a daily percent series without a hint.
var seriesSpecs = map[string]seriesSpec{
	"DFF":             {unit: unitPercent, hint: hintDFF},
	"DGS3MO":          {unit: unitPercent, hint: hintTbill},
	"DGS2":            {unit: unitPercent, hint: hintDGS2},
	"DGS10":           {unit: unitPercent, hint: hintDGS10},
	"T10Y2Y":          {unit: unitPercent, hint: hintT10Y2Y},
	"T10Y3M":          {unit: unitPercent, hint: hintT10Y3M},
	"T10YIE":          {unit: unitPercent, hint: hintT10YIE},
	"BAMLH0A0HYM2":    {unit: unitPercent, hint: hintHySpread},
	"NFCI":            {unit: unitIndex, weekly: true, hint: hintNFCI},
	"DTWEXBGS":        {unit: unitIndex, hint: hintDTWEXBGS},
	"DEXJPUS":         {unit: unitJPY, hint: hintDEXJPUS},
	"IRLTLT01JPM156N": {unit: unitPercent, monthly: true, hint: hintIRLTJP},
}

type obsPoint struct {
	day   time.Time
	value float64
}

// observations returns the points of rows with a value that were published by d, one per observation day, oldest first.
func observations(rows []contracts.SeriesPoint, d time.Time) []obsPoint {
	kept := map[string]contracts.SeriesPoint{}
	for _, p := range rows {
		if p.RealtimeStart.After(d) {
			continue
		}
		k := p.ObservationDate.Format(dayLayout)
		if cur, ok := kept[k]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[k] = p
		}
	}
	out := make([]obsPoint, 0, len(kept))
	for _, p := range kept {
		if p.Value != nil {
			out = append(out, obsPoint{day: p.ObservationDate.Time, value: *p.Value})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].day.Before(out[j].day) })
	return out
}

// valueText prints v with its unit: a percent as the page prints it today, a level with two decimals.
func valueText(v float64, unit string) string {
	switch unit {
	case unitPercent:
		return strconv.FormatFloat(v, 'f', -1, 64) + "%"
	case unitJPY:
		return strconv.FormatFloat(v, 'f', 2, 64) + " JPY per USD"
	default:
		return strconv.FormatFloat(v, 'f', 2, 64)
	}
}

// changeText is the change of the last point against the one back observations earlier, "" when that point is missing.
func changeText(obs []obsPoint, back int, unit, words string) string {
	if len(obs) <= back {
		return ""
	}
	delta := obs[len(obs)-1].value - obs[len(obs)-1-back].value
	sign := ""
	if delta > 0 {
		sign = "+"
	}
	text := sign + strconv.FormatFloat(delta, 'f', 2, 64)
	switch unit {
	case unitPercent:
		text += " pp"
	case unitJPY:
		text += " JPY per USD"
	default:
		text += " index points"
	}
	return text + " vs " + words
}

// changeOf is the change text of a series: five observations back for a daily one, the previous point for a monthly one.
func changeOf(obs []obsPoint, sp seriesSpec) string {
	if sp.monthly {
		return changeText(obs, 1, sp.unit, "previous month")
	}
	return changeText(obs, changeBack, sp.unit, "five sessions")
}

func specOf(id string) seriesSpec {
	if sp, ok := seriesSpecs[id]; ok {
		return sp
	}
	return seriesSpec{unit: unitPercent}
}

// readObs reads id from back days before d through d and returns its observations. An error is wrapped with the id.
func readObs(ctx context.Context, r store.SeriesReader, id string, d time.Time, back int) ([]obsPoint, error) {
	from := contracts.Day(d.AddDate(0, 0, -back).Format(dayLayout))
	pts, err := r.SeriesPoints(ctx, id, from, contracts.Day(d.Format(dayLayout)))
	if err != nil {
		return nil, fmt.Errorf("macro_page: read %s: %w", id, err)
	}
	return observations(pts, d), nil
}

// groupDay is the day of the latest point of a grouped series.
type groupDay struct {
	id      string
	day     time.Time
	monthly bool
	weekly  bool
}

// buildGroups reads the series of every group and returns their rows in order, and the day of each series that has a
// point. An error of a read is an error.
func buildGroups(ctx context.Context, r store.SeriesReader, groups []Group, d time.Time) ([]indicatorRow, []groupDay, error) {
	var rows []indicatorRow
	var days []groupDay
	for _, g := range groups {
		for _, ind := range g.Indicators {
			obs, err := readObs(ctx, r, ind.SeriesID, d, groupLookbackDays)
			if err != nil {
				return nil, nil, err
			}
			rows = append(rows, groupRow(g.Key, ind, obs, d))
			if len(obs) > 0 {
				sp := specOf(ind.SeriesID)
				days = append(days, groupDay{ind.SeriesID, obs[len(obs)-1].day, sp.monthly, sp.weekly})
			}
		}
	}
	return rows, days, nil
}

// groupRow is one row of a group: the old fields of row plus group, change, data day and hint.
func groupRow(key string, ind Indicator, obs []obsPoint, d time.Time) indicatorRow {
	sp := specOf(ind.SeriesID)
	out := indicatorRow{Name: ind.Name, SourceText: ind.SourceText, Group: key, Hint: sp.hint}
	if ind.DetailHref != "" {
		h := ind.DetailHref
		out.DetailHref = &h
	}
	if len(obs) == 0 {
		out.Latest = gap{Reason: "no_data", Text: "no data yet"}
		out.Status, out.Label = "missing", "n/a"
		return out
	}
	last := obs[len(obs)-1]
	out.Latest = figure{Value: last.value, Text: valueText(last.value, sp.unit), Unit: sp.unit}
	day := last.day.Format(dayLayout)
	out.DataDay = &day
	if c := changeOf(obs, sp); c != "" {
		out.ChangeText = &c
	}
	limit := freshnessLimitDays
	if sp.monthly {
		limit = monthlyFreshnessDays
	} else if sp.weekly {
		limit = weeklyFreshnessDays
	}
	if d.Sub(last.day) > time.Duration(limit)*24*time.Hour {
		out.Status, out.Label = "late", "Late"
	} else {
		out.Status, out.Label = "good", "On time"
	}
	return out
}
