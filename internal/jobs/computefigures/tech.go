package computefigures

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/blockapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	techVersion    = 1
	techConvention = "tech_state_v1"
	techUnit       = "text_tag"
	techLookback   = 140 // calendar days of closes read before the as-of day
)

// techStep is the tech step of a job: the classifier and its trial constants.
type techStep struct {
	classify blockapi.TechFunc
	params   blockapi.TechParams
}

// rankFigures holds the figure of each rank as a constant: the value is the rank itself and no number is computed.
var rankFigures = map[int]compute.Figure{
	1: {Value: 1}, 0: {Value: 0}, -1: {Value: -1}, -2: {Value: -2},
}

// WithTech adds the key market.tech_state|market|NDX_SPX and its step to a computefigures job (or to the
// valuation job that wraps one). It returns the job unchanged when Classify is nil, when PriceBars or IndexIDs
// is nil, or for a job of another type.
func WithTech(j contracts.Job, d TechDeps) contracts.Job {
	if d.Classify == nil {
		return j
	}
	switch v := j.(type) {
	case *job:
		if c := v.withTech(d); c != nil {
			return c
		}
	case *valuationJob:
		if c := v.withTech(d); c != nil {
			cp := *v
			cp.job = c
			return &cp
		}
	}
	return j
}

// withTech is a copy of j with the tech step, nil when j cannot have one.
func (j *job) withTech(d TechDeps) *job {
	if j.stores.PriceBars == nil || j.stores.IndexIDs == nil || j.m.Tech == "" || len(j.m.TechIndices) != 2 {
		return nil
	}
	c := *j
	c.tech = &techStep{classify: d.Classify, params: d.Params}
	c.rule = make(map[string]compute.Rule, len(j.rule)+1)
	for k, r := range j.rule {
		c.rule[k] = r
	}
	p := d.Params
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%s|%s|%d|%d|%d|%d", j.m.Tech, techVersion, p.Band, p.Hard,
		p.Short, p.Long, p.Fold, p.MaxAgeDays)))
	c.rule[j.m.Tech] = compute.Rule{ID: j.m.Tech, Version: techVersion, Convention: techConvention, Unit: techUnit,
		Hash: hex.EncodeToString(sum[:])}
	return &c
}

// techSubject is the subject of the tech key: the index names joined by an underscore.
func (j *job) techSubject() string { return strings.Join(j.m.TechIndices, "_") }

// isTechKey is true for the key of the tech step, which a job without the step does not have.
func (j *job) isTechKey(name, kind, subj string) bool {
	return j.tech != nil && name == j.m.Tech && kind == kindMarket && subj == j.techSubject()
}

// runTech reads the trades closes of both indices, classifies them and stores one reading. A name with no
// instrument is retryable, an ambiguous one permanent, a failed read retryable; nothing is stored for them.
func (r *runState) runTech(t target) {
	if r.ctx.Err() != nil {
		r.failStore(r.ctx.Err())
		return
	}
	names := r.j.m.TechIndices
	id, ok := r.techIDs(names)
	if !ok {
		return
	}
	var in blockapi.TechIn
	var lines, keys []string
	series := [2]*blockapi.CloseIn{&in.NDX, &in.SPX}
	for i, n := range names {
		bars, err := r.j.stores.PriceBars.PriceBars(r.ctx, id[i], basisTrades, shiftDay(r.day, -techLookback), r.day)
		if err != nil {
			r.failStore(fmt.Errorf("computefigures: price bars of %s: %w", n, err))
			return
		}
		sort.Slice(bars, func(a, b int) bool { return dayOf(bars[a].SessionDay) > dayOf(bars[b].SessionDay) })
		s := series[i]
		s.Name, s.Symbol = n, n
		for _, b := range bars {
			d := dayOf(b.SessionDay)
			s.Closes, s.Days = append(s.Closes, b.Close), append(s.Days, d)
			lines = append(lines, n+"|"+string(d)+"|"+b.Close)
		}
		keys = append(keys, strconv.FormatInt(id[i], 10)+"|"+basisTrades)
	}
	in.Day, in.Params = r.day, r.j.tech.params
	res := r.j.tech.classify(in)
	f := compute.Figure{Status: compute.StatusMissing, Reason: res.Reason, DataDay: res.DataDay}
	if res.Reason == "" {
		rf, ok := rankFigures[res.Rank]
		if !ok || res.State == "" {
			r.fail(contracts.Permanent, fmt.Errorf("computefigures: %s: classifier gave rank %d, state %q", t.rule.ID, res.Rank, res.State))
			return
		}
		f.Value, f.Status = rf.Value, compute.StatusOK
	}
	r.store(t, f, hashLines(lines), []input{{table: tablePriceBar, keys: keys}}, techParams(res, r.j.tech.params))
}

// techIDs resolves the two index names to instrument ids; not ok when either fails (already recorded).
func (r *runState) techIDs(names []string) (id [2]int64, ok bool) {
	rows, err := r.j.stores.IndexIDs.IndexIDs(r.ctx, names)
	if err != nil {
		r.failStore(fmt.Errorf("computefigures: index instruments: %w", err))
		return id, false
	}
	ids := map[string][]int64{}
	for _, row := range rows {
		ids[row.Name] = append(ids[row.Name], row.InstrumentID)
	}
	ok = true
	for i, n := range names {
		switch got := ids[n]; len(got) {
		case 0:
			r.fail(contracts.Retryable, fmt.Errorf("computefigures: no index instrument named %s", n))
			ok = false
		case 1:
			id[i] = got[0]
		default:
			r.fail(contracts.Permanent, fmt.Errorf("computefigures: index instrument %s is ambiguous: %d rows", n, len(got)))
			ok = false
		}
	}
	return id, ok
}

// techParams is the evidence of a tech reading, all as text; a missing reading keeps the data day and the trial flag.
func techParams(res blockapi.TechResult, p blockapi.TechParams) paramsOf {
	return func(f compute.Figure) []param {
		var ps []param
		add := func(name, v string) { ps = append(ps, param{name: name, tag: &v}) }
		if f.Status != compute.StatusOK {
			if f.DataDay != "" {
				add("data_day", string(f.DataDay))
			}
			add("trial", "true")
			return ps
		}
		add("state", res.State)
		add("raw", res.Raw)
		add("data_day", string(res.DataDay))
		add("ndx_1m", res.NDX1m)
		add("spx_1m", res.SPX1m)
		add("gap_1m", res.Gap1m)
		add("ndx_3m", res.NDX3m)
		add("spx_3m", res.SPX3m)
		add("gap_3m", res.Gap3m)
		add("band", p.Band)
		add("hard", p.Hard)
		add("sessions_1m", strconv.Itoa(p.Short))
		add("sessions_3m", strconv.Itoa(p.Long))
		add("raw_days", strconv.Itoa(res.RawDays))
		add("trial", "true")
		return ps
	}
}
