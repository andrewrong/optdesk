package computefigures

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// volLookback is the calendar days of closes the vol step reads before the as-of day: the 141 closes of the
// forecast error are about 215 calendar days.
const volLookback = 240

// pastSessions is how many sessions back the forecast of the error is made.
const pastSessions = 20

// isVolID is true for the id of one of the six figures of the vol step.
func (j *job) isVolID(id string) bool {
	switch id {
	case j.m.Rv20, j.m.Rv60, j.m.Rv120, j.m.VolForecast, j.m.VolEdge, j.m.VolForecastErr:
		return id != ""
	}
	return false
}

// volIDs are the ids the vol step plans for a name; market.rv20 is planned by runRv20 for an index name.
func (j *job) volIDs(name string) []string {
	ids := []string{j.m.Rv60, j.m.Rv120, j.m.VolForecast, j.m.VolEdge, j.m.VolForecastErr}
	for _, n := range j.m.Rv20Indices {
		if n == name {
			return ids
		}
	}
	return append([]string{j.m.Rv20}, ids...)
}

// volGroup is the planned vol targets of one subject by figure id.
type volGroup struct {
	subj subject
	by   map[string]target
}

// runVol reads the closes of each subject once and stores the figures planned for it: rv20, rv60 and rv120 with
// HistoricalVol, the forecast, the edge against the stored market.iv_ibkr of the day, and the forecast error
// (the forecast of 20 sessions ago, each part called with the day of that session, less today's rv20). A missing
// part gives a stored missing reading, never a fill.
func (r *runState) runVol(ts []target) {
	var groups []*volGroup
	idx := map[string]*volGroup{}
	for _, t := range ts {
		g := idx[t.subj.name]
		if g == nil {
			g = &volGroup{subj: t.subj, by: map[string]target{}}
			idx[t.subj.name] = g
			groups = append(groups, g)
		}
		g.by[t.rule.ID] = t
	}
	for _, g := range groups {
		if r.ctx.Err() != nil {
			r.failStore(r.ctx.Err())
			return
		}
		r.volSubject(g)
	}
}

func (r *runState) volSubject(g *volGroup) {
	m := r.j.m
	rows, err := r.j.stores.PriceBars.PriceBars(r.ctx, g.subj.id, basisTrades, shiftDay(r.day, -volLookback), r.day)
	if err != nil {
		r.failStore(fmt.Errorf("computefigures: price bars of %s: %w", g.subj.name, err))
		return
	}
	closes := make(compute.Series, 0, len(rows))
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		v, perr := strconv.ParseFloat(row.Close, 64)
		if perr != nil {
			v = 0 // the core answers bad_input for a close that is not above zero
		}
		d := dayOf(row.SessionDay)
		closes = append(closes, compute.Point{Day: d, Value: v})
		lines = append(lines, string(d)+"|"+row.Close)
	}
	barIn := input{table: tablePriceBar, keys: []string{strconv.FormatInt(g.subj.id, 10) + "|" + basisTrades}}
	hv := func(id string, c compute.Series, d contracts.Day) compute.Figure {
		return r.j.figs.HistoricalVol(id, c, d)
	}
	rv := [3]compute.Figure{hv(m.Rv20, closes, r.day), hv(m.Rv60, closes, r.day), hv(m.Rv120, closes, r.day)}
	forecast := r.j.figs.Combine(m.VolForecast, rv[:], r.day)
	hash := hashLines(lines)
	store := func(id string, f compute.Figure, h string, in []input, days ...dayParam) {
		t, ok := g.by[id]
		if !ok {
			return
		}
		r.store(t, f, h, in, volParams(t.rule, days))
	}
	for i, id := range []string{m.Rv20, m.Rv60, m.Rv120} {
		if t, ok := g.by[id]; ok {
			r.store(t, rv[i], hash, []input{barIn}, hvParams(t.rule))
		}
	}
	store(m.VolForecast, forecast, hash, []input{barIn},
		dayParam{"rv20_day", rv[0].DataDay}, dayParam{"rv60_day", rv[1].DataDay}, dayParam{"rv120_day", rv[2].DataDay})

	// edge: implied minus forecast
	if t, ok := g.by[m.VolEdge]; ok {
		imp, line, found := r.impliedOf(g.subj.name)
		if !found {
			r.store(t, r.missing(t, compute.NoInput, ""), hash, []input{barIn}, volParams(t.rule, []dayParam{{"forecast_day", forecast.DataDay}}))
		} else {
			f := r.j.figs.Combine(m.VolEdge, []compute.Figure{imp, forecast}, r.day)
			ivKey := m.IvIbkr + "|" + kindUnderlying + "|" + g.subj.name
			r.store(t, f, hashLines(append(append([]string{}, lines...), line)),
				[]input{barIn, {table: tableReadings, keys: []string{ivKey}}}, volParams(t.rule,
					[]dayParam{{"implied_day", imp.DataDay}, {"forecast_day", forecast.DataDay}}))
		}
	}

	// error: the forecast of 20 sessions ago less today's rv20
	if t, ok := g.by[m.VolForecastErr]; ok {
		past := r.pastForecast(closes)
		f := r.j.figs.Combine(m.VolForecastErr, []compute.Figure{past, rv[0]}, r.day)
		r.store(t, f, hash, []input{barIn}, volParams(t.rule,
			[]dayParam{{"forecast_day", past.DataDay}, {"rv20_day", rv[0].DataDay}}))
	}
}

// pastForecast is the forecast the same method gives on the closes up to 20 sessions ago: HistoricalVol for the
// three windows on closes 0 to n-21, each called with the day of close n-21, then Combine with the same day.
func (r *runState) pastForecast(closes compute.Series) compute.Figure {
	m := r.j.m
	var past compute.Series
	if len(closes) > pastSessions {
		past = closes[:len(closes)-pastSessions]
	}
	d := r.day
	if len(past) > 0 {
		d = past[len(past)-1].Day
	}
	parts := make([]compute.Figure, 0, 3)
	for _, id := range []string{m.Rv20, m.Rv60, m.Rv120} {
		parts = append(parts, r.j.figs.HistoricalVol(id, past, d))
	}
	return r.j.figs.Combine(m.VolForecast, parts, d)
}

// impliedOf is the ok market.iv_ibkr reading dated the as-of day as a figure, and its row line for the hash.
func (r *runState) impliedOf(name string) (compute.Figure, string, bool) {
	rows, err := r.j.stores.Readings.LatestReadings(r.ctx, r.j.m.IvIbkr, kindUnderlying, name, r.day, r.day)
	if err != nil {
		r.failStore(fmt.Errorf("computefigures: %s readings: %w", r.j.m.IvIbkr, err))
		return compute.Figure{}, "", false
	}
	for i := len(rows) - 1; i >= 0; i-- {
		rec := rows[i]
		if rec.State != "ok" || rec.Value == nil || dayOf(rec.Day) != r.day {
			continue
		}
		v, perr := strconv.ParseFloat(*rec.Value, 64)
		if perr != nil {
			continue
		}
		return compute.Figure{ID: r.j.m.IvIbkr, Version: rec.Version, Subject: name, Day: r.day, DataDay: r.day,
			Value: v, Status: compute.StatusOK, Observations: 1}, string(r.day) + "|" + *rec.Value, true
	}
	return compute.Figure{}, "", false
}

// dayParam is the day of one part of a combined reading.
type dayParam struct {
	name string
	day  contracts.Day
}

// volParams is data_day, observations and the day of each part that has one.
func volParams(rule compute.Rule, days []dayParam) paramsOf {
	return func(f compute.Figure) []param {
		ps := parameters(rule, false, f)
		for _, d := range days {
			if d.day != "" {
				v := string(d.day)
				ps = append(ps, param{name: d.name, tag: &v})
			}
		}
		return ps
	}
}

// volRuleIDs are the ids of the six figures of the vol step less market.rv20, which the job always loads; an
// empty manifest key is an error.
func (j *job) volRuleIDs() ([]string, error) {
	ids := []string{j.m.Rv60, j.m.Rv120, j.m.VolForecast, j.m.VolEdge, j.m.VolForecastErr}
	for _, id := range ids {
		if id == "" {
			return nil, errors.New("computefigures: manifest: no vol ids")
		}
	}
	return ids, nil
}
