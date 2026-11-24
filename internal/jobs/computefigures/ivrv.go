package computefigures

import (
	"errors"
	"fmt"
	"sort"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// isIvRv20Key is true for the key of one of the manifest's iv_rv20 names.
func (j *job) isIvRv20Key(name, kind, subj string) bool {
	if j.m.IvRv20 == "" || name != j.m.IvRv20 || kind != kindUnderlying {
		return false
	}
	_, ok := j.m.IvRv20Series[subj]
	return ok
}

// ivRv20Names are the index names of the manifest's iv_rv20 table, sorted.
func (j *job) ivRv20Names() []string {
	names := make([]string, 0, len(j.m.IvRv20Series))
	for n := range j.m.IvRv20Series {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// runIvRv20 stores market.iv_rv20 of each name: the FRED series of the name over 100 less the stored market.rv20
// of the same day, joined by Combine. The day D' is the newest day not after the as-of day on which the series has
// a point and an rv20 reading is stored; the rv20 part counts only when that newest-rev reading is ok and its
// data_day is D'. No such day, a part that does not count or a D' too old is a stored missing reading.
//
// A data_day before the session day is no failure: FRED publishes a day late and the figure carries its data_day.
func (r *runState) runIvRv20(names []string) {
	rule := r.j.rule[r.j.m.IvRv20]
	for _, name := range names {
		if r.ctx.Err() != nil {
			r.failStore(r.ctx.Err())
			return
		}
		t := target{rule: rule, subj: subject{kind: kindUnderlying, name: name}}
		seriesID := r.j.m.IvRv20Series[name]
		from := shiftDay(r.day, -lookbackDays(rule))
		pts, err := r.j.stores.Series.SeriesPoints(r.ctx, seriesID, from, r.day)
		if err != nil {
			r.failStore(fmt.Errorf("computefigures: series %s: %w", seriesID, err))
			continue
		}
		series := r.j.ops.FromSeriesPoints(pts)
		rows, err := r.j.stores.Readings.LatestReadings(r.ctx, r.j.m.Rv20, kindUnderlying, name, from, r.day)
		if err != nil {
			r.failStore(fmt.Errorf("computefigures: %s readings: %w", r.j.m.Rv20, err))
			continue
		}
		var lines []string
		for _, p := range series {
			lines = append(lines, seriesID+"|"+string(p.Day)+"|"+fnum(p.Value))
		}
		byDay := map[contracts.Day]contracts.ReadingRecord{}
		for _, rec := range rows {
			d := dayOf(rec.Day)
			if dd := contracts.Day(tagParam(rec, "data_day")); dd != "" {
				if old, ok := byDay[dd]; !ok || dayOf(old.Day) < d {
					byDay[dd] = rec
				}
			}
			val := ""
			if rec.Value != nil {
				val = *rec.Value
			}
			lines = append(lines, "rv20|"+string(d)+"|"+string(rec.State)+"|"+val+"|"+tagParam(rec, "data_day"))
		}
		f := r.ivRv20Figure(t, series, byDay)
		in := []input{{table: tableSeriesPoint, keys: []string{seriesID}},
			{table: tableReadings, keys: []string{r.j.m.Rv20 + "|" + kindUnderlying + "|" + name}}}
		r.store(t, f, hashLines(lines), in, volParams(rule, nil))

	}
}

// ivRv20Figure joins the series point and the rv20 reading of the newest day that has both.
func (r *runState) ivRv20Figure(t target, series compute.Series, byDay map[contracts.Day]contracts.ReadingRecord) compute.Figure {
	for i := len(series) - 1; i >= 0; i-- {
		p := series[i]
		rec, ok := byDay[p.Day]
		if !ok {
			continue
		}
		rv := compute.Figure{ID: r.j.m.Rv20, Subject: t.subj.name, Day: r.day, Status: compute.StatusMissing, Reason: compute.NoInput}
		if v, perr := strconv.ParseFloat(derefText(rec.Value), 64); rec.State == "ok" && perr == nil &&
			contracts.Day(tagParam(rec, "data_day")) == p.Day {
			rv = compute.Figure{ID: r.j.m.Rv20, Version: rec.Version, Subject: t.subj.name, Day: r.day, DataDay: p.Day,
				Value: v, Status: compute.StatusOK, Observations: 1}
		}
		iv := compute.Figure{ID: t.rule.ID, Subject: t.subj.name, Day: r.day, DataDay: p.Day, Value: p.Value / 100,
			Status: compute.StatusOK, Observations: 1}
		return r.j.figs.Combine(t.rule.ID, []compute.Figure{iv, rv}, r.day)
	}
	return r.missing(t, compute.NoInput, "")
}

func derefText(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// tagParam is the text of the named parameter of a reading, "" when it has none.
func tagParam(rec contracts.ReadingRecord, name string) string {
	for _, p := range rec.Parameters {
		if p.Param == name && p.ValueTag != nil {
			return *p.ValueTag
		}
	}
	return ""
}

var errNoIvRv20 = errors.New("computefigures: manifest: no iv_rv20 id or series")
