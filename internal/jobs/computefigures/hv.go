package computefigures

import (
	"fmt"
	"strconv"

	"github.com/andrewrong/optdesk/internal/compute"
)

const (
	tablePriceBar = "obs.price_bar"
	basisTrades   = "trades"
	hvLookback    = 70 // calendar days of closes read before the as-of day
)

// hvOn is true when the job has a price bar reader; without one the HV30 step is skipped.
func (j *job) hvOn() bool { return j.stores.PriceBars != nil }

// runHv30 stores the HV30 of each underlying from its winner "trades" closes (never adjusted_last, never
// IBKR's own figure). Fewer closes than the rule needs, none, or a stale newest one is a missing reading.
func (r *runState) runHv30(ts []target) {
	for _, t := range ts {
		if r.ctx.Err() != nil {
			r.failStore(r.ctx.Err())
			return
		}
		rows, err := r.j.stores.PriceBars.PriceBars(r.ctx, t.subj.id, basisTrades, shiftDay(r.day, -hvLookback), r.day)
		if err != nil {
			r.failStore(fmt.Errorf("computefigures: price bars of %s: %w", t.subj.name, err))
			continue
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
		f := r.j.figs.HistoricalVol(t.rule.ID, closes, r.day)
		in := []input{{table: tablePriceBar, keys: []string{strconv.FormatInt(t.subj.id, 10) + "|" + basisTrades}}}
		r.store(t, f, hashLines(lines), in, hvParams(t.rule))
	}
}

// hvParams is data_day, observations and the window (the rule's N) of an HV reading.
func hvParams(rule compute.Rule) paramsOf {
	return func(f compute.Figure) []param {
		ps := parameters(rule, false, f)
		n := float64(rule.N)
		return append(ps, param{name: "window", num: &n})
	}
}
