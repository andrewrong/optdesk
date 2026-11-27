package fredseries

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/seriesdef"
)

var _ NewFredSeriesFunc = New

type job struct {
	m      manifest
	src    sources.Source
	second sources.Source // nil: no fallback
	w      store.SeriesWriter
	clk    clock.Clock
	log    *slog.Logger

	cal         string
	open, close time.Duration
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
	if te, ok := errors.AsType[contracts.TypedError](err); ok {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// New builds the fred_series job from the embedded job.toml, without a second source.
func New(src sources.Source, w store.SeriesWriter, clk clock.Clock) (contracts.Job, error) {
	return NewWithSecondary(src, nil, w, clk)
}

func build(data []byte, src sources.Source, w store.SeriesWriter, clk clock.Clock) (*job, error) {
	m, err := parseManifest(data)
	if err != nil {
		return nil, err
	}
	cal, open, closeOff, err := windowOf(m)
	if err != nil {
		return nil, err
	}
	return &job{m: m, src: src, w: w, clk: clk, log: slog.Default(), cal: cal, open: open, close: closeOff}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Run fetches the series of keys for day and upserts the points in one call.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var plain, windowed, long []string
	for _, k := range keys {
		switch {
		case slices.Contains(j.m.Long, k.SubjectKey):
			long = append(long, k.SubjectKey)
		case slices.Contains(j.m.Windowed, k.SubjectKey):
			windowed = append(windowed, k.SubjectKey)
		default:
			plain = append(plain, k.SubjectKey)
		}
	}
	var points []contracts.SeriesPoint
	windowedFrom := j.sizedFrom(ctx, windowed, daysBefore(day, j.m.LookbackDays), day)
	longFrom := j.sizedFrom(ctx, long, daysBefore(day, j.m.LongLookbackDays), day)
	groups := []fetchGroup{{plain, day}, {windowed, windowedFrom}, {long, longFrom}}
	for _, g := range groups {
		if len(g.ids) == 0 {
			continue
		}
		got, err := j.src.Fetch(ctx, sources.FetchPlan{SeriesIDs: g.ids, From: g.from, To: day})
		if err != nil {
			return contracts.JobResult{Err: asTyped(err, contracts.Permanent)}
		}
		points = append(points, got...)
	}
	res := contracts.JobResult{Returned: len(points)}
	for _, p := range points {
		if p.Value != nil {
			res.Usable++
		}
	}
	defer j.fillGaps(ctx, day, groups, points)
	if len(points) == 0 {
		return res
	}
	up, ok := j.w.(seriesdef.SeriesUpserter)
	if !ok {
		res.Usable = 0
		res.Err = asTyped(errors.New("fredseries: the writer has no UpsertSeriesPoints method"), contracts.Retryable)
		return res
	}
	if _, err := up.UpsertSeriesPoints(ctx, points, "fred_series:"+string(day)); err != nil {
		res.Usable = 0
		res.Err = asTyped(err, contracts.Retryable)
	}
	return res
}

// sizedFrom is the first day to fetch for ids. A run for a day before the UTC date of the clock
// starts after the newest stored points when the writer also reads points; any other run, and a
// reader error, keep groupFrom.
func (j *job) sizedFrom(ctx context.Context, ids []string, groupFrom, day contracts.Day) contracts.Day {
	r, ok := j.w.(store.SeriesReader)
	if !ok || len(ids) == 0 || string(day) >= j.clk.Now().UTC().Format(dayLayout) {
		return groupFrom
	}
	from, err := gapFrom(ctx, r, ids, groupFrom, day)
	if err != nil {
		j.log.Warn("fred_series: window not sized, stored points unreadable", "day", string(day), "error", err.Error())
		return groupFrom
	}
	return from
}

// daysBefore is the calendar day n days before day; an unparsable day is returned as it is.
func daysBefore(day contracts.Day, n int) contracts.Day {
	t, err := time.Parse("2006-01-02", string(day))
	if err != nil {
		return day
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format("2006-01-02"))
}
