package macroevents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/macroevent"
)

var _ NewMacroEventsFunc = New

const (
	sourceFRED = "fred"
	sourceFile = "file"
)

type job struct {
	m   manifest
	src ReleaseSource
	w   macroevent.Writer
	clk clock.Clock
	log *slog.Logger
}

// New builds the macro_events job from the embedded job.toml.
func New(src ReleaseSource, w macroevent.Writer, clk clock.Clock) (contracts.Job, error) {
	m, err := parseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}
	if _, err := parseHand(handBytes); err != nil {
		return nil, err
	}
	return &job{m: m, src: src, w: w, clk: clk, log: slog.Default()}, nil
}

// ID is the job id of the manifest.
func (j *job) ID() string { return j.m.ID }

// report collects what one run did, kind by kind.
type report struct {
	returned, written int
	lines, held       []string
	firstErr          error
	failed            []string
}

func (r *report) fail(kind string, err error) {
	r.failed = append(r.failed, kind)
	if r.firstErr == nil {
		r.firstErr = fmt.Errorf("macroevents: %s: %w", kind, err)
	}
}

// replace calls ReplaceFuture for one kind and notes the last date and a hold.
func (j *job) replace(ctx context.Context, r *report, day contracts.Day, kind, src string, evs []macroevent.Event) {
	out, err := j.w.ReplaceFuture(ctx, kind, src, day, evs)
	if err != nil {
		r.fail(kind, err)
		return
	}
	r.returned += len(evs)
	r.written += out.Written
	last := "none"
	for _, e := range evs {
		if last == "none" || string(e.Date) > last {
			last = string(e.Date)
		}
	}
	r.lines = append(r.lines, kind+" last "+last)
	if out.Held {
		r.held = append(r.held, kind)
	}
}

// Run fetches the FRED kinds one at a time, then writes the file kinds. A kind whose fetch fails is left
// unchanged and reported; the others are still written.
func (j *job) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	want := map[string]bool{}
	for _, k := range keys {
		want[k.SubjectKey] = true
	}
	wanted := func(kind string) bool { return len(want) == 0 || want[kind] }
	rep := &report{}
	for _, rl := range releases {
		if !wanted(rl.kind) {
			continue
		}
		dates, err := j.src.Dates(ctx, rl.id, day)
		if err != nil {
			rep.fail(rl.kind, err)
			continue
		}
		evs := make([]macroevent.Event, 0, len(dates))
		for _, d := range dates {
			evs = append(evs, macroevent.Event{Kind: rl.kind, Source: sourceFRED, SourceRef: rl.id, Date: d, RunDay: day})
		}
		j.replace(ctx, rep, day, rl.kind, sourceFRED, evs)
	}
	files, err := fileEvents()
	if err != nil {
		rep.fail("file", err)
	}
	for _, kind := range []string{kindFOMC, kindMinutes, kindBOJ, kindQuarterEnd, kindUSTax} {
		if wanted(kind) && len(files[kind]) > 0 {
			j.replace(ctx, rep, day, kind, sourceFile, files[kind])
		}
	}
	text := "macro_events " + string(day) + ": " + strings.Join(rep.lines, "; ")
	for _, k := range rep.held {
		text += "; kept stored rows of " + k + ": answer shorter or empty"
	}
	if len(rep.failed) > 0 {
		text += "; not fetched: " + strings.Join(rep.failed, ", ")
	}
	j.log.Info(text)
	res := contracts.JobResult{Returned: rep.returned, Usable: rep.written}
	if rep.firstErr != nil {
		res.Err = asTyped(rep.firstErr, contracts.Retryable)
	}
	return res
}

// fileEvents are the rows of the hand-kept file by kind, with RunDay the read_day of the row and the minutes
// derived from the fomc dates.
func fileEvents() (map[string][]macroevent.Event, error) {
	srcs, err := parseHand(handBytes)
	if err != nil {
		return nil, err
	}
	out := map[string][]macroevent.Event{}
	add := func(kind string, s handSource, d contracts.Day) {
		out[kind] = append(out[kind], macroevent.Event{
			Kind: kind, Source: sourceFile, SourceRef: s.URL, Date: d, RunDay: contracts.Day(s.ReadDay),
		})
	}
	for _, s := range srcs {
		for _, d := range s.Dates {
			add(s.Kind, s, contracts.Day(d))
			if s.Kind == kindFOMC {
				add(kindMinutes, s, minutesDay(contracts.Day(d)))
			}
		}
	}
	return out, nil
}

// asTyped returns err unchanged when it is a TypedError, else wraps it with kind. A TypedError found inside err
// keeps its own kind.
func asTyped(err error, kind contracts.ErrorKind) contracts.TypedError {
	var te contracts.TypedError
	if errors.As(err, &te) {
		return &wrapped{msg: err.Error(), kind: te.Kind(), err: err}
	}
	return &wrapped{msg: err.Error(), kind: kind, err: err}
}

type wrapped struct {
	msg  string
	kind contracts.ErrorKind
	err  error
}

func (e *wrapped) Error() string             { return e.msg }
func (e *wrapped) Kind() contracts.ErrorKind { return e.kind }
func (e *wrapped) Unwrap() error             { return e.err }
