package fredseries

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

var _ jobs.NewFredSeriesFunc = New

type job struct {
	m   manifest
	src sources.Source
	w   store.SeriesWriter
	clk clock.Clock
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
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	return &job{m: m, src: src, w: w, clk: clk}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// Run fetches the series of keys for day and writes the points in one call.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, k.SubjectKey)
	}
	points, err := j.src.Fetch(ctx, sources.FetchPlan{SeriesIDs: ids, From: day, To: day})
	if err != nil {
		return contracts.JobResult{Err: asTyped(err, contracts.Permanent)}
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
