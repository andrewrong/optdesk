package computefigures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	tableOptionEod   = "obs.option_eod"
	tableSeriesPoint = "obs.series_point"
	tableReadings    = "readings.reading"
)

// runState is one Run: what it has stored and the first failure.
type runState struct {
	j        *job
	ctx      context.Context
	day      contracts.Day
	returned int
	usable   int
	failed   int
	noTerms  int // option rows dropped for want of a current terms version
	firstErr contracts.TypedError
}

func (r *runState) fail(kind contracts.ErrorKind, err error) {
	r.failed++
	if r.firstErr == nil || (r.firstErr.Kind() != contracts.Retryable && kind == contracts.Retryable) {
		r.firstErr = &typedError{kind: kind, err: err}
	}
}

// failStore records a failed read or write: a store error is retryable, a cancelled context too.
func (r *runState) failStore(err error) { r.fail(contracts.Retryable, err) }

// runIv30 computes the IV30 of each underlying from the stored option rows.
func (r *runState) runIv30(ts []target) {
	for _, t := range ts {
		if r.ctx.Err() != nil {
			r.failStore(r.ctx.Err())
			return
		}
		in := inputs{ComputeStores: r.j.stores, ops: r.j.ops, units: r.j.units, rateSeries: r.j.m.RateSeries,
			rateMaxAge: t.rule.MaxAgeDays}
		chain, rate, noTerms, err := ivInputCounted(r.ctx, in, t.subj.id, r.day)
		r.noTerms += noTerms
		var f compute.Figure
		switch {
		case errors.Is(err, errNoRate), errors.Is(err, errNoSession):
			f = r.missing(t, compute.NoInput, "")
		case errors.Is(err, errStaleRate):
			f = r.missing(t, compute.StaleInput, rate.Day)
		case err != nil:
			r.failStore(err)
			continue
		default:
			f = r.j.figs.Iv30(t.rule.ID, chain, rate, r.day)
		}
		inputsOf := []input{
			{table: tableOptionEod, keys: []string{strconv.FormatInt(t.subj.id, 10) + "|" + string(r.day)}},
			{table: tableSeriesPoint, keys: []string{r.j.m.RateSeries}},
		}
		r.store(t, f, hashLines(chainLines(chain, rate)), inputsOf, figureParams(t.rule, true))
	}
}

func chainLines(chain []compute.ExpiryQuotes, rate compute.Point) []string {
	lines := []string{"rate|" + string(rate.Day) + "|" + fnum(rate.Value)}
	for _, e := range chain {
		for _, q := range e.Strikes {
			lines = append(lines, fmt.Sprintf("chain|%d|%s|%s|%s|%s|%s|%t|%t", e.MinutesToExpiry, fnum(q.Strike),
				fnum(q.CallIV), fnum(q.PutIV), fnum(q.CallMid), fnum(q.PutMid), q.HasCall, q.HasPut))
		}
	}
	return lines
}

func fnum(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// hashLines is the sha256 of the sorted lines, the rows_sha256 of a reading.
func hashLines(lines []string) string {
	sorted := append([]string(nil), lines...)
	sort.Strings(sorted)
	sum := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return hex.EncodeToString(sum[:])
}

// sessionLookbackDays is how far before the as-of day the newest session day is looked for.
const sessionLookbackDays = 10

// lookbackDays is the number of calendar days of inputs a series figure reads before the as-of day.
func lookbackDays(rule compute.Rule) int {
	if rule.Years > 0 {
		return rule.Years*366 + 7
	}
	return 2*max(rule.Window, rule.N) + 14
}

// runMarket computes the figures on FRED series.
//
// A figure whose newest FRED point is older than the newest session day of the calendar up to the as-of day is
// stored as it is and reported as a retryable failure: the point of that day may still arrive, and a rerun with a
// changed input hash stores the next rev.
func (r *runState) runMarket(ts []target) {
	var sessionDay contracts.Day // the newest session day up to the as-of day, "" when none is known
	sessionsRead := false
	for _, t := range ts {
		if r.ctx.Err() != nil {
			r.failStore(r.ctx.Err())
			return
		}
		from := shiftDay(r.day, -lookbackDays(t.rule))
		var series []compute.Series
		var lines []string
		bad := false
		for _, id := range t.spec.Series {
			pts, err := r.j.stores.Series.SeriesPoints(r.ctx, id, from, r.day)
			if err != nil {
				r.failStore(fmt.Errorf("computefigures: series %s: %w", id, err))
				bad = true
				break
			}
			s := r.j.ops.FromSeriesPoints(pts)
			series = append(series, s)
			for _, p := range s {
				lines = append(lines, id+"|"+string(p.Day)+"|"+fnum(p.Value))
			}
		}
		if bad {
			continue
		}
		if !sessionsRead {
			days, err := r.j.stores.SessionDays.SessionDays(r.ctx, calendarID, shiftDay(r.day, -sessionLookbackDays), r.day)
			if err != nil {
				r.failStore(fmt.Errorf("computefigures: session days: %w", err))
				continue
			}
			for _, d := range days {
				if dd := dayOf(d.SessionDay); d.IsSession && dd > sessionDay {
					sessionDay = dd
				}
			}
			sessionsRead = true
		}
		var f compute.Figure
		switch t.spec.Method {
		case methodRatio:
			f = r.j.figs.Ratio(t.rule.ID, series[0], series[1], r.day)
		case methodPercent:
			f = r.j.figs.PercentileInYears(t.rule.ID, series[0], r.day)
		default:
			f = r.j.figs.MovingAverageGap(t.rule.ID, series[0], r.day)
		}
		r.store(t, f, hashLines(lines), []input{{table: tableSeriesPoint, keys: t.spec.Series}}, figureParams(t.rule, false))
		for i, s := range series {
			if len(s) == 0 || sessionDay == "" || s[len(s)-1].Day >= sessionDay {
				continue
			}
			r.fail(contracts.Retryable, fmt.Errorf("computefigures: %s: series %s newest point %s, waits for %s",
				t.rule.ID, t.spec.Series[i], s[len(s)-1].Day, sessionDay))
		}
	}
}

// runIvStats computes IVP and IVR from the stored IV30 readings, and the IVP of market.ivp_ibkr from the stored
// market.iv_ibkr readings (never mixed with the own series); no ok IV30 reading of the as-of day gives no_input.
func (r *runState) runIvStats(ts []target) {
	for _, t := range ts {
		if r.ctx.Err() != nil {
			r.failStore(r.ctx.Err())
			return
		}
		from := shiftDay(r.day, -lookbackDays(t.rule))
		src := r.j.m.Iv30
		if t.rule.ID == r.j.m.IvpIbkr {
			src = r.j.m.IvIbkr
		}
		rows, err := r.j.stores.Readings.LatestReadings(r.ctx, src, kindUnderlying, t.subj.name, from, r.day)
		if err != nil {
			r.failStore(fmt.Errorf("computefigures: %s readings: %w", src, err))
			continue
		}
		var series compute.Series
		var lines []string
		todayOK := false
		for _, rec := range rows {
			if rec.State != "ok" || rec.Value == nil {
				continue
			}
			v, perr := strconv.ParseFloat(*rec.Value, 64)
			if perr != nil {
				continue
			}
			d := dayOf(rec.Day)
			series = append(series, compute.Point{Day: d, Value: v})
			lines = append(lines, string(d)+"|"+*rec.Value)
			if d == r.day {
				todayOK = true
			}
		}
		var f compute.Figure
		switch {
		case !todayOK:
			f = r.missing(t, compute.NoInput, "")
		case t.rule.ID == r.j.m.Ivp || t.rule.ID == r.j.m.IvpIbkr:
			f = r.j.figs.IvPercentile(t.rule.ID, series, r.day)
		default:
			f = r.j.figs.IvRank(t.rule.ID, series, r.day)
		}
		key := src + "|" + kindUnderlying + "|" + t.subj.name
		r.store(t, f, hashLines(lines), []input{{table: tableReadings, keys: []string{key}}}, figureParams(t.rule, false))
	}
}

// missing is the figure the job itself finds missing, before the core is asked.
func (r *runState) missing(t target, reason compute.Reason, dataDay contracts.Day) compute.Figure {
	return compute.Figure{ID: t.rule.ID, Version: t.rule.Version, Subject: t.subj.name, Day: r.day, DataDay: dataDay,
		Status: compute.StatusMissing, Reason: reason}
}

func reasonCode(r compute.Reason) int {
	switch r {
	case compute.NoInput:
		return 1
	case compute.ShortHistory:
		return 2
	case compute.StaleInput:
		return 3
	case compute.ZeroRange:
		return 4
	case compute.BadInput:
		return 5
	default:
		return 5
	}
}

// store builds the reading of f and writes it: an equal state, reason and rows_sha256 of the newest rev is
// skipped, anything else is written as the next rev.
func (r *runState) store(t target, f compute.Figure, rowsHash string, in []input, ps paramsOf) {
	rec, err := r.record(t, f, rowsHash, in, ps)
	if err != nil {
		r.fail(contracts.Permanent, err)
		return
	}
	prev, err := r.j.stores.Readings.LatestReadings(r.ctx, rec.Name, rec.SubjectKind, rec.Subject, r.day, r.day)
	if err != nil {
		r.failStore(fmt.Errorf("computefigures: readings: %w", err))
		return
	}
	rec.Rev = 1
	if n := len(prev); n > 0 {
		p := prev[n-1]
		if p.State == rec.State && p.RowsSha256 == rec.RowsSha256 && sameReason(p.ReasonCode, rec.ReasonCode) {
			r.usable++
			return
		}
		rec.Rev = p.Rev + 1
		if p.State == "ok" && rec.State == "missing_input" {
			// v3's winner rule keeps showing the earlier ok; the change is only visible here.
			slog.Warn("computefigures: a missing reading is hidden behind an earlier ok", "name", rec.Name,
				"subject", rec.Subject, "day", string(r.day), "rev", rec.Rev)
		}
	}
	if _, err := r.j.stores.Readings.InsertReadings(r.ctx, []contracts.ReadingRecord{rec}); err != nil {
		r.failStore(fmt.Errorf("computefigures: insert readings: %w", err))
		return
	}
	r.usable++
}

func sameReason(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// record builds the reading of f at rev 0. A value the units cannot write is stored as bad_input, and ps
// then sees the figure as missing.
func (r *runState) record(t target, f compute.Figure, rowsHash string, in []input, ps paramsOf) (contracts.ReadingRecord, error) {
	rule := t.rule
	rec := contracts.ReadingRecord{
		Name: rule.ID, Version: rule.Version, SubjectKind: t.subj.kind, Subject: t.subj.name,
		Day: dateOf(r.day), Currency: "",
		Origin: "compute_figures", Cutoff: slotName,
		JobRunID:   uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(r.day)+"|"+r.j.m.ID+"|"+slotName)).String(),
		RecordedAt: r.j.clk.Now().UTC(), RowsSha256: rowsHash,
	}
	if f.Status == compute.StatusOK {
		text, err := r.j.units.RoundHalfEven(f.Value, rule.Scale)
		if err != nil {
			f.Status, f.Reason = compute.StatusMissing, compute.BadInput
		} else {
			rec.State, rec.Value = "ok", &text
		}
	}
	if f.Status != compute.StatusOK {
		code := reasonCode(f.Reason)
		rec.State, rec.ReasonCode = "missing_input", &code
	}
	setString(&rec.Unit, rule.Unit)
	for _, i := range in {
		e := grow(&rec.Inputs)
		e.InputTable, e.Keys, e.CodeVersion, e.ConventionVersion = i.table, i.keys, rule.Hash, rule.Convention
	}
	for _, p := range ps(f) {
		e := grow(&rec.Parameters)
		e.Param, e.ValueNum, e.ValueTag = p.name, p.num, p.tag
	}
	return rec, nil
}

type input struct {
	table string
	keys  []string
}

type param struct {
	name string
	num  *float64
	tag  *string
}

// grow appends a zero element to *s and returns it, so the generated element types need no name here.
func grow[E any](s *[]E) *E {
	var z E
	*s = append(*s, z)
	return &(*s)[len(*s)-1]
}

// setString sets a string-kinded enum field from text.
func setString[T ~string](dst *T, v string) { *dst = T(v) }

// paramsOf builds the parameters of a reading from the figure as stored.
type paramsOf func(f compute.Figure) []param

// figureParams is the parameter list of an IV30 or a market figure.
func figureParams(rule compute.Rule, iv30 bool) paramsOf {
	return func(f compute.Figure) []param { return parameters(rule, iv30, f) }
}

func parameters(rule compute.Rule, iv30 bool, f compute.Figure) []param {
	num := func(name string, v int) param {
		x := float64(v)
		return param{name: name, num: &x}
	}
	tag := func(name, v string) param { return param{name: name, tag: &v} }
	var ps []param
	if f.DataDay != "" {
		ps = append(ps, tag("data_day", string(f.DataDay)))
	}
	ps = append(ps, num("observations", f.Observations))
	if iv30 && f.Status == compute.StatusOK {
		ps = append(ps, tag("stale_rate", strconv.FormatBool(f.StaleRate)))
	}
	if rule.Window > 0 {
		ps = append(ps, num("window", rule.Window))
	}
	if rule.Years > 0 {
		ps = append(ps, num("years", rule.Years))
	}
	return ps
}
