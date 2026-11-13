package fredseries

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

// secondaryNYFed is the registry value of secondary_source this job can ask.
const secondaryNYFed = "nyfed"

// fetchGroup is one primary fetch: the series ids and the From day; To is the run day.
type fetchGroup struct {
	ids  []string
	from contracts.Day
}

// NewWithSecondary builds the job like New and, when second is not nil, lets Run
// ask second for a series whose registry row names "nyfed" and whose run day the primary
// fetch gave no value for (missing or no_value). The points are written with FillSeriesPoints only.
func NewWithSecondary(src, second sources.Source, w store.SeriesWriter, clk clock.Clock) (contracts.Job, error) {
	if src == nil || w == nil || clk == nil {
		return nil, errors.New("fredseries: source, writer and clock are required")
	}
	j, err := build(manifestBytes, src, w, clk)
	if err != nil {
		return nil, err
	}
	j.second = second
	return j, nil
}

// fillGaps is the fallback. It never changes the result of Run: every failure is logged.
// A writer without SeriesRegistry or SeriesFiller means no fallback.
func (j *job) fillGaps(ctx context.Context, day contracts.Day, groups []fetchGroup, primary []contracts.SeriesPoint) {
	if j.second == nil {
		return
	}
	reg, ok := j.w.(seriesdef.SeriesRegistry)
	if !ok {
		return
	}
	defs, err := reg.ActiveSeries(ctx)
	if err != nil {
		j.log.Warn("fred_series: fallback skipped, registry unreadable", "day", string(day), "error", err.Error())
		return
	}
	named := map[string]bool{}
	for _, d := range defs {
		if d.SecondarySource == secondaryNYFed {
			named[d.SeriesID] = true
		}
	}
	hasValue := map[string]bool{}
	for _, p := range primary {
		if p.Value != nil && p.ObservationDate.Format("2006-01-02") == string(day) {
			hasValue[p.SeriesID] = true
		}
	}
	var plans []sources.FetchPlan
	for _, g := range groups {
		var ids []string
		for _, id := range g.ids {
			if named[id] && !hasValue[id] {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			plans = append(plans, sources.FetchPlan{SeriesIDs: ids, From: g.from, To: day})
		}
	}
	if len(plans) == 0 {
		return
	}
	fill, ok := j.w.(seriesdef.SeriesFiller)
	if !ok {
		j.log.Warn("fred_series: fallback skipped, the writer has no FillSeriesPoints method", "day", string(day))
		return
	}
	for _, plan := range plans {
		pts, err := j.second.Fetch(ctx, plan)
		if err != nil {
			j.log.Warn("fred_series: second source failed", "day", string(day), "series", plan.SeriesIDs, "error", err.Error())
			continue
		}
		if len(pts) == 0 {
			continue
		}
		if _, err := fill.FillSeriesPoints(ctx, pts, "fred_series:"+string(day)+":"+secondaryNYFed); err != nil {
			j.log.Warn("fred_series: fill failed", "day", string(day), "series", plan.SeriesIDs, "error", err.Error())
		}
	}
}
