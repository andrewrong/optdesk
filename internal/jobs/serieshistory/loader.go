// Package serieshistory loads the history of series once, outside the daily job.
package serieshistory

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

const dayLayout = "2006-01-02"

type typedError struct {
	kind contracts.ErrorKind
	msg  string
}

func (e *typedError) Error() string             { return e.msg }
func (e *typedError) Kind() contracts.ErrorKind { return e.kind }

func permanent(format string, a ...any) contracts.TypedError {
	return &typedError{kind: contracts.Permanent, msg: "serieshistory: " + fmt.Sprintf(format, a...)}
}

// classOf is the kind of err when it is a TypedError, else fallback.
func classOf(err error, fallback contracts.ErrorKind) contracts.ErrorKind {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return te.Kind()
	}
	return fallback
}

type loader struct {
	src sources.Source
	w   store.SeriesWriter
}

// New builds the loader (jobs.NewSeriesHistoryFunc).
func New(src sources.Source, w store.SeriesWriter) (jobs.SeriesHistoryLoader, error) {
	if src == nil || w == nil {
		return nil, errors.New("serieshistory: source and writer are required")
	}
	return &loader{src: src, w: w}, nil
}

// Load fetches From..To once per series, then writes each series with one InsertSeriesPoints call.
// Nothing is written when a fetch fails. Only the class of a source or writer error is passed on.
func (l *loader) Load(ctx context.Context, plan sources.FetchPlan) ([]jobs.SeriesLoaded, error) {
	if len(plan.SeriesIDs) == 0 {
		return nil, permanent("no series given")
	}
	from, err := time.Parse(dayLayout, string(plan.From))
	if err != nil {
		return nil, permanent("from is not a day")
	}
	to, err := time.Parse(dayLayout, string(plan.To))
	if err != nil {
		return nil, permanent("to is not a day")
	}
	if from.After(to) {
		return nil, permanent("from is after to")
	}
	if from.Before(to.AddDate(-5, 0, 0)) {
		return nil, permanent("range is over 5 years")
	}
	seen := map[string]bool{}
	var ids []string
	for _, id := range plan.SeriesIDs {
		if id == "" {
			return nil, permanent("empty series id")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	points := make([][]contracts.SeriesPoint, len(ids))
	for i, id := range ids {
		pts, err := l.src.Fetch(ctx, sources.FetchPlan{SeriesIDs: []string{id}, From: plan.From, To: plan.To})
		if err != nil {
			return nil, &typedError{kind: classOf(err, contracts.Permanent), msg: "serieshistory: fetch failed"}
		}
		points[i] = pts
	}
	out := make([]jobs.SeriesLoaded, len(ids))
	for i, id := range ids {
		if err := l.w.InsertSeriesPoints(ctx, points[i]); err != nil {
			return nil, &typedError{kind: classOf(err, contracts.Retryable), msg: "serieshistory: write failed"}
		}
		out[i] = jobs.SeriesLoaded{SeriesID: id, Fetched: len(points[i])}
	}
	return out, nil
}
