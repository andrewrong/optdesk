package computefigures

import (
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute"
)

// easeSeriesIDs are the seven FRED series of compute.EaseSeries, in the order of its fields.
var easeSeriesIDs = []string{"WALCL", "WDTGAL", "RRPONTSYD", "DGS10", "DFF", "SP500", "BAMLH0A0HYM2"}

// checkEaseSpec requires the [ease] table to name the figure and the seven series, in the order of the fields.
func checkEaseSpec(e easeSpec) error {
	if e.ID == "" || len(e.Series) != len(easeSeriesIDs) {
		return fmt.Errorf("computefigures: manifest: [ease] needs an id and %d series", len(easeSeriesIDs))
	}
	for i, id := range easeSeriesIDs {
		if e.Series[i] != id {
			return fmt.Errorf("computefigures: manifest: [ease] series %d is %q, want %q", i, e.Series[i], id)
		}
	}
	return nil
}

// isEaseKey is true for the key of the ease figure; it is never a [[market]] key.
func (j *job) isEaseKey(name, subj string) bool {
	return j.ease != nil && name == j.m.Ease.ID && subj == j.m.Ease.Series[0]
}

// runEase reads the seven series, asks the Ease for the figure and stores it. A failed read stores nothing and
// stops no other key.
func (r *runState) runEase(t target) {
	if r.ctx.Err() != nil {
		r.failStore(r.ctx.Err())
		return
	}
	from := shiftDay(r.day, -lookbackDays(t.rule))
	got := make([]compute.Series, len(easeSeriesIDs))
	var lines []string
	for i, id := range easeSeriesIDs {
		pts, err := r.j.stores.Series.SeriesPoints(r.ctx, id, from, r.day)
		if err != nil {
			r.failStore(fmt.Errorf("computefigures: series %s: %w", id, err))
			return
		}
		got[i] = r.j.ops.FromSeriesPoints(pts)
		for _, p := range got[i] {
			lines = append(lines, id+"|"+string(p.Day)+"|"+fnum(p.Value))
		}
	}
	series := compute.EaseSeries{WALCL: got[0], WDTGAL: got[1], RRPONTSYD: got[2], DGS10: got[3], DFF: got[4],
		SP500: got[5], BAMLH0A0HYM2: got[6]}
	ef := r.j.ease.Ease(t.rule.ID, series, r.day)
	r.store(t, ef.Figure, hashLines(lines), []input{{table: tableSeriesPoint, keys: r.j.m.Ease.Series}}, easeParams(ef))
}

// easeParams is the parameter list of an ease reading: the band when the value is ok, and per input its
// data_day, figure and score when present or its reason when missing.
func easeParams(ef compute.EaseFigure) paramsOf {
	return func(f compute.Figure) []param {
		tag := func(name, v string) param { return param{name: name, tag: &v} }
		num := func(name string, v float64) param { return param{name: name, num: &v} }
		var ps []param
		if f.Status == compute.StatusOK && ef.Band != "" {
			ps = append(ps, tag("band", string(ef.Band)))
		}
		for _, p := range ef.Parts {
			if p.Status == compute.StatusOK {
				ps = append(ps, tag(p.Input+".data_day", string(p.DataDay)), num(p.Input+".figure", p.Figure),
					num(p.Input+".score", p.Score))
			} else {
				ps = append(ps, tag(p.Input+".reason", string(p.Reason)))
			}
		}
		return ps
	}
}
