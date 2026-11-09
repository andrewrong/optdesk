package market_page

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	// termLookbackDays is how far before the request day the series points are read.
	termLookbackDays = 40
	// termFreshDays is the display limit of a point: a weekend plus one holiday.
	termFreshDays = 4
	// termGapDays is the distance from the today point to the older point.
	termGapDays = 5
)

type termPoint struct {
	Label       string `json:"label"`
	Today       any    `json:"today"`
	FiveDaysAgo any    `json:"five_days_ago"`
}

type term struct {
	Title    string      `json:"title"`
	Sentence string      `json:"sentence"`
	Points   []termPoint `json:"points"`
}

type point struct {
	date time.Time
	text string
}

var termSeries = []string{seriesVIX, seriesVXV}

// buildTerm returns the term block (one Gap when all four cells are Gaps), the number of
// cells that are Gaps and fills days with the newest stored point of each series.
func buildTerm(ctx context.Context, c Config, r store.SeriesReader, d time.Time, ratio reading, days dataDays) (any, int, error) {
	t := term{Title: termTitle, Sentence: termSentence(ratio), Points: []termPoint{}}
	gaps := 0
	for i, id := range termSeries {
		rows, err := r.SeriesPoints(ctx, id, contracts.Day(d.AddDate(0, 0, -termLookbackDays).Format(dayLayout)), contracts.Day(d.Format(dayLayout)))
		if err != nil {
			return nil, 0, fmt.Errorf("market_page: read %s: %w", id, err)
		}
		pts := storedPoints(rows, d)
		tp := termPoint{Label: termLabels[i]}
		tp.Today, tp.FiveDaysAgo = termCells(pts, d)
		for _, cell := range []any{tp.Today, tp.FiveDaysAgo} {
			if _, isGap := cell.(gap); isGap {
				gaps++
			}
		}
		if len(pts) > 0 {
			days.add(id, pts[0].date)
		}
		t.Points = append(t.Points, tp)
	}
	if gaps == 2*len(termSeries) {
		return mkGap(reasonNoData, termNoPoint), gaps, nil
	}
	return t, gaps, nil
}

// storedPoints keeps one point per observation date, the one with the greatest
// RealtimeStart not after d, drops points without a value or after d, and returns them
// newest first.
func storedPoints(rows []contracts.SeriesPoint, d time.Time) []point {
	kept := map[time.Time]contracts.SeriesPoint{}
	for _, p := range rows {
		obs := dateOf(p.ObservationDate.Time)
		if obs.After(d) || dateOf(p.RealtimeStart.Time).After(d) {
			continue
		}
		if cur, ok := kept[obs]; !ok || p.RealtimeStart.After(cur.RealtimeStart.Time) {
			kept[obs] = p
		}
	}
	out := []point{}
	for obs, p := range kept {
		if p.Value != nil {
			out = append(out, point{date: obs, text: pointText(*p.Value)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].date.After(out[j].date) })
	return out
}

// termCells picks today and five_days_ago from points, newest first.
func termCells(pts []point, d time.Time) (today, older any) {
	if len(pts) == 0 {
		g := mkGap(reasonNoData, termNoPoint)
		return g, g
	}
	newest := pts[0]
	today = pointCell(newest)
	if daysBetween(newest.date, d) > termFreshDays {
		today = mkGap(reasonStale, termStaleToday)
	}
	target := newest.date.AddDate(0, 0, -termGapDays)
	older = mkGap(reasonNoData, termNoOlder)
	for _, p := range pts {
		if p.date.After(target) {
			continue
		}
		if daysBetween(p.date, target) <= termFreshDays {
			older = pointCell(p)
		}
		break
	}
	return today, older
}

func pointCell(p point) figure {
	return mkFigure(p.text, p.text, unitIndexPoints)
}

func dateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(from, to time.Time) int { return int(to.Sub(from) / (24 * time.Hour)) }
