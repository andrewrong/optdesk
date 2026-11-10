package fredseries

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ NewFredSeriesFunc = New

type job struct {
	m   manifest
	src sources.Source
	w   store.SeriesWriter
	clk clock.Clock

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
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te
	}
	return &typedError{kind: kind, err: err}
}

// New builds the fred_series job from the embedded job.toml.
func New(src sources.Source, w store.SeriesWriter, clk clock.Clock) (contracts.Job, error) {
	if src == nil || w == nil || clk == nil {
		return nil, errors.New("fredseries: source, writer and clock are required")
	}
	return build(manifestBytes, src, w, clk)
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
	return &job{m: m, src: src, w: w, clk: clk, cal: cal, open: open, close: closeOff}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Run fetches the series of keys for day and writes the points in one call.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	var plain, windowed []string
	for _, k := range keys {
		if slices.Contains(j.m.Windowed, k.SubjectKey) {
			windowed = append(windowed, k.SubjectKey)
		} else {
			plain = append(plain, k.SubjectKey)
		}
	}
	var points []contracts.SeriesPoint
	for _, g := range []struct {
		ids  []string
		from contracts.Day
	}{{plain, day}, {windowed, daysBefore(day, j.m.LookbackDays)}} {
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
	if len(points) == 0 {
		return res
	}
	if err := j.w.InsertSeriesPoints(ctx, points); err != nil {
		res.Usable = 0
		res.Err = asTyped(err, contracts.Retryable)
	}
	return res
}

// daysBefore is the calendar day n days before day; an unparsable day is returned as it is.
func daysBefore(day contracts.Day, n int) contracts.Day {
	t, err := time.Parse("2006-01-02", string(day))
	if err != nil {
		return day
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format("2006-01-02"))
}
