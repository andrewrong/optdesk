package fxrefresh

import (
	"context"
	"strconv"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
)

// fredSeries is the FRED H.10 series of JPY per USD.
const fredSeries = "DEXJPUS"

var _ AttachSecondaryFunc = AttachSecondary

// AttachSecondary attaches the fill source to a job New built and reports true; any other job is left alone (false).
func AttachSecondary(j contracts.Job, second sources.Source) bool {
	jb, ok := j.(*job)
	if !ok {
		return false
	}
	jb.second = second
	return true
}

// fill asks the attached source for DEXJPUS over [day - fill_lookback_days, day] and inserts each point with a
// positive value as source fred_h10. InsertFxRates leaves a stored key alone, so nothing is replaced. Every failure
// is logged and never reaches the result of Run. No source attached means no call.
func (j *job) fill(ctx context.Context, day contracts.Day) {
	if j.second == nil {
		return
	}
	d, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return
	}
	from := contracts.Day(d.AddDate(0, 0, -j.m.FillLookbackDays).Format(dayLayout))
	pts, err := j.second.Fetch(ctx, sources.FetchPlan{SeriesIDs: []string{fredSeries}, From: from, To: day})
	if err != nil {
		j.log.Warn("fx_refresh: fill source failed", "day", string(day), "error", err.Error())
		return
	}
	now := j.clk.Now().UTC()
	var rows []contracts.FxRate
	for _, p := range pts {
		if p.Value == nil || !(*p.Value > 0) {
			continue
		}
		rows = append(rows, contracts.FxRate{
			Pair: pair, Rate: strconv.FormatFloat(*p.Value, 'f', -1, 64),
			RateDay: types.SerializableDate{Time: p.ObservationDate.Time}, ReceivedAt: now, Source: sourceFred,
		})
	}
	if len(rows) == 0 {
		return
	}
	if _, err := j.w.InsertFxRates(ctx, rows); err != nil {
		j.log.Warn("fx_refresh: fill insert failed", "day", string(day), "error", err.Error())
	}
}
