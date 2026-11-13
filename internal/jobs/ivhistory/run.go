package ivhistory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/readingkey"
)

const (
	originIbkr   = "ibkr"
	cutoff       = "session_close"
	inputTable   = "ibkr.bars"
	reasonNoIn   = 1
	reasonBadIn  = 5
	unitDecimal  = "decimal"
	stateOk      = "ok"
	stateMissing = "missing_input"
)

// setString sets a string-kinded enum field of the generated record from text.
func setString[T ~string](dst *T, v string) { *dst = T(v) }

func grow[E any](s *[]E) *E {
	var z E
	*s = append(*s, z)
	return &(*s)[len(*s)-1]
}

// isNoData reports a broker answer of "no data" (error 162). The code is only in the error text of Bars.
func isNoData(err error) bool { return strings.Contains(err.Error(), "broker error 162") }

func isOk(r contracts.ReadingRecord) bool { return string(r.State) == stateOk }

// keyRun is what runKey leaves for one key: the rows to write, the stored readings it saw and its failure.
type keyRun struct {
	rows []contracts.ReadingRecord
	have map[string]contracts.ReadingRecord
	err  contracts.TypedError
}

// Run pulls the bars of the names of keys and stores the readings of all of them in one UpsertReadings call.
// A key's failure does not stop the others; the result carries the first failure, a Retryable one before a
// Permanent one. Returned is the number of keys that hold a reading of the day (ok or missing) and Usable those
// whose reading of the day is ok. A failed write fails the keys that had rows in it.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	up, ok := j.w.(readingkey.ReadingUpserter)
	if !ok {
		return contracts.JobResult{Err: newErr(contracts.Retryable, "ivhistory: the readings store has no UpsertReadings method")}
	}
	dayTime, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return contracts.JobResult{Err: newErr(contracts.Permanent, "bad day")}
	}
	var res contracts.JobResult
	fail := func(e contracts.TypedError) {
		if e != nil && (res.Err == nil || (res.Err.Kind() != contracts.Retryable && e.Kind() == contracts.Retryable)) {
			res.Err = e
		}
	}
	runs := make([]keyRun, len(keys))
	var all []contracts.ReadingRecord
	for i, k := range keys {
		runs[i] = j.runKey(ctx, day, dayTime, k)
		all = append(all, runs[i].rows...)
	}
	var werr contracts.TypedError
	if len(all) > 0 {
		if _, err := up.UpsertReadings(ctx, all); err != nil {
			werr = asTyped(err, contracts.Retryable)
		}
	}
	for _, kr := range runs {
		if kr.err != nil {
			fail(kr.err)
			continue
		}
		if werr != nil && len(kr.rows) > 0 {
			fail(werr)
			continue
		}
		for _, r := range kr.rows {
			kr.have[r.Day.Format(dayLayout)] = r
		}
		res.Returned++
		if today, ok := kr.have[string(day)]; ok && isOk(today) {
			res.Usable++
		}
	}
	return res
}

func (j *job) runKey(ctx context.Context, day contracts.Day, dayTime time.Time, k contracts.ExpectedKey) keyRun {
	name, ok := j.nameOf(k)
	if !ok {
		return keyRun{err: newErr(contracts.Permanent, "unknown key %q", k.SubjectKey)}
	}
	from := contracts.Day(dayTime.AddDate(0, 0, -j.m.LookbackDays).Format(dayLayout))
	stored, err := j.r.LatestReadings(ctx, j.m.Reading, kindUnderlying, name, from, day)
	if err != nil {
		return keyRun{err: asTyped(err, contracts.Retryable)}
	}
	have := map[string]contracts.ReadingRecord{}
	anyOk := false
	for _, s := range stored {
		have[s.Day.Format(dayLayout)] = s
		anyOk = anyOk || isOk(s)
	}
	days := j.m.DailyDays
	if !anyOk {
		days = j.m.BackfillDays
	}
	bars, err := j.h.Bars(ctx, broker.BarQuery{Contract: j.contract[name], Kind: broker.BarOptionImpliedVol, DurationDays: days})
	if err != nil && !isNoData(err) {
		return keyRun{err: asTyped(err, contracts.Retryable)}
	}
	now := j.clk.Now().UTC()
	runID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+j.m.ID+"|"+cutoff)).String()
	var rows []contracts.ReadingRecord
	arrived := false
	for _, b := range bars {
		d, err := time.Parse(dayLayout, b.Day)
		if err != nil {
			return keyRun{err: newErr(contracts.Permanent, "bad bar day")}
		}
		if b.Day > string(day) {
			continue
		}
		arrived = arrived || b.Day == string(day)
		rows = j.add(rows, have, j.reading(name, d, b.Close, now, runID))
	}
	if !arrived {
		rows = j.add(rows, have, j.missing(name, dayTime, "", reasonNoIn, now, runID))
	}
	return keyRun{rows: rows, have: have}
}

// add appends rec unless a reading of its day is stored already: a stored ok one is kept, a stored missing
// one is replaced by an ok one at the next rev, and a missing one is never stored over a missing one. A day
// is pending once: a later bar of a day replaces the pending row of that day, so no key is twice in a call.
func (j *job) add(rows []contracts.ReadingRecord, have map[string]contracts.ReadingRecord, rec contracts.ReadingRecord) []contracts.ReadingRecord {
	old, ok := have[rec.Day.Format(dayLayout)]
	if ok {
		if isOk(old) || !isOk(rec) {
			return rows
		}
		rec.Rev = old.Rev + 1
	}
	for i, r := range rows {
		if r.Day.Equal(rec.Day.Time) {
			rows[i] = rec
			return rows
		}
	}
	return append(rows, rec)
}

// reading is the reading of one bar: ok with the stored text of its close, else missing bad_input.
func (j *job) reading(name string, day time.Time, close string, now time.Time, runID string) contracts.ReadingRecord {
	text, err := j.ivText(close)
	if err != nil {
		return j.missing(name, day, close, reasonBadIn, now, runID)
	}
	rec := j.base(name, day, close, now, runID)
	setString(&rec.State, stateOk)
	rec.Value = &text
	return rec
}

func (j *job) missing(name string, day time.Time, close string, code int, now time.Time, runID string) contracts.ReadingRecord {
	rec := j.base(name, day, close, now, runID)
	setString(&rec.State, stateMissing)
	rec.ReasonCode = &code
	return rec
}

// base is the reading of name and day without its state: one input row, the bar of the day.
func (j *job) base(name string, day time.Time, close string, now time.Time, runID string) contracts.ReadingRecord {
	d := day.Format(dayLayout)
	sum := sha256.Sum256([]byte(inputTable + "|" + name + "|" + d + "|" + close))
	rec := contracts.ReadingRecord{
		Name: j.m.Reading, Version: j.m.ReadingVersion, SubjectKind: kindUnderlying, Subject: name,
		Day: types.SerializableDate{Time: day}, Cutoff: cutoff, Rev: 1,
		JobRunID: runID, RecordedAt: now, RowsSha256: hex.EncodeToString(sum[:]),
	}
	setString(&rec.Unit, unitDecimal)
	setString(&rec.Origin, originIbkr)
	in := grow(&rec.Inputs)
	in.InputTable, in.Keys = inputTable, []string{name + "|" + d}
	in.CodeVersion, in.ConventionVersion = j.m.ID+"_v1", j.m.Convention
	return rec
}
