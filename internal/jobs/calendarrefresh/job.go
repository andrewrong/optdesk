// Package calendarrefresh is the job that refreshes master.session_day from the session source.
package calendarrefresh

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

//go:embed job.toml
var manifestBytes []byte

const (
	datasetID  = "master.session_day"
	daysBefore = 7
	daysAfter  = 400
	dayLayout  = "2006-01-02"
)

var _ NewCalendarRefreshFunc = New

type manifest struct {
	Version  int           `toml:"version"`
	ID       string        `toml:"id"`
	Schedule scheduleTable `toml:"schedule"`
}

type job struct {
	m     manifest
	sched schedule
	src   clock.SessionSource
	st    store.SessionDayStore
	diff  clock.SessionDayDiffer
	clk   clock.Clock
	obs   CalendarObserver
	log   *slog.Logger
}

type typedError struct {
	kind contracts.ErrorKind
	err  error
}

func (e *typedError) Error() string             { return e.err.Error() }
func (e *typedError) Unwrap() error             { return e.err }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// logFailure logs the step and the error kind of a failed run; the kind is unknown when err has none. It
// never logs the error text, which can carry a store address or a key.
func (j *job) logFailure(step string, err error) {
	kind := "unknown"
	var k interface{ Kind() contracts.ErrorKind }
	if errors.As(err, &k) {
		kind = string(k.Kind())
	}
	j.log.Warn("run failed", "step", step, "kind", kind)
}

// New builds the calendar_refresh job from the embedded job.toml. A nil observer is allowed.
func New(src clock.SessionSource, st store.SessionDayStore, diff clock.SessionDayDiffer, clk clock.Clock, obs CalendarObserver) (contracts.Job, error) {
	if src == nil || st == nil || diff == nil || clk == nil {
		return nil, errors.New("calendarrefresh: source, store, differ and clock are required")
	}
	var m manifest
	if err := toml.Unmarshal(manifestBytes, &m); err != nil {
		return nil, fmt.Errorf("calendarrefresh: manifest: %w", err)
	}
	if m.ID == "" {
		return nil, errors.New("calendarrefresh: manifest needs an id")
	}
	s, err := scheduleOf(m.Schedule)
	if err != nil {
		return nil, err
	}
	return &job{m: m, sched: s, src: src, st: st, diff: diff, clk: clk, obs: obs, log: slog.Default()}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Plan returns the one key {master.session_day, calendar} and the hash of the plan.
func (j *job) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	cal := j.sched.calendar
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s\n%s", j.m.ID, j.m.Version, day, cal)))
	return []contracts.ExpectedKey{{DatasetID: datasetID, SubjectKey: cal}}, hex.EncodeToString(sum[:]), nil
}

func shift(day time.Time, days int) contracts.Day {
	return contracts.Day(day.AddDate(0, 0, days).Format(dayLayout))
}

// Run reads the stored and the fresh rows over today-7..today+400, upserts the added and changed
// rows, tells the observer of each change before the write and deletes nothing.
func (j *job) Run(ctx context.Context, _ contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	cal := j.sched.calendar
	now := j.clk.Now().In(j.sched.loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from, to := shift(today, -daysBefore), shift(today, daysAfter)

	stored, err := j.st.SessionDays(ctx, cal, from, to)
	if err != nil {
		j.logFailure("read_store", err)
		return contracts.JobResult{Err: asTyped(err, contracts.Retryable)}
	}
	fresh, err := j.src.SessionDays(ctx, cal, from, to)
	if err != nil {
		j.logFailure("read_source", err)
		return contracts.JobResult{Err: asTyped(err, contracts.Permanent)}
	}
	if len(fresh) == 0 {
		err := &typedError{kind: contracts.Permanent, err: errors.New("calendarrefresh: the source returned no session days")}
		j.logFailure("empty_source", err)
		return contracts.JobResult{Err: err}
	}
	res := contracts.JobResult{Returned: len(fresh), Usable: len(fresh)}
	var rows []contracts.SessionDay
	for _, c := range j.diff.Diff(stored, fresh) {
		if j.obs != nil {
			j.obs(c)
		}
		if c.After != nil {
			rows = append(rows, *c.After)
		}
	}
	if len(rows) == 0 {
		return res
	}
	if err := j.st.UpsertSessionDays(ctx, rows); err != nil {
		j.logFailure("write", err)
		res.Usable = 0
		res.Err = asTyped(err, contracts.Retryable)
	}
	return res
}

// Postcondition is true when the store holds one row for every calendar day from the as-of day
// to the span end (as-of day plus 400 days) of the key.
func (j *job) Postcondition(ctx context.Context, day, _, subjectKey string) (bool, error) {
	d, err := time.Parse(dayLayout, day)
	if err != nil {
		j.logFailure("postcondition_day", err)
		return false, asTyped(err, contracts.Permanent)
	}
	rows, err := j.st.SessionDays(ctx, subjectKey, contracts.Day(day), shift(d, daysAfter))
	if err != nil {
		j.logFailure("postcondition_read", err)
		return false, asTyped(err, contracts.Retryable)
	}
	have := make(map[string]bool, len(rows))
	for _, r := range rows {
		have[r.SessionDay.Format(dayLayout)] = true
	}
	for i := 0; i <= daysAfter; i++ {
		if !have[string(shift(d, i))] {
			return false, nil
		}
	}
	return true, nil
}
