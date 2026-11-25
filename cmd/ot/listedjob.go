package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// listedError is the typed error of listedJob.
type listedError struct {
	kind contracts.ErrorKind
	msg  string
	err  error
}

func (e *listedError) Error() string {
	if e.err != nil {
		return e.msg + ": " + e.err.Error()
	}
	return e.msg
}
func (e *listedError) Kind() contracts.ErrorKind { return e.kind }
func (e *listedError) Unwrap() error             { return e.err }

// listedBuild builds the inner job of one job family from the entries of its group.
type listedBuild func([]jobs.WatchEntry) (contracts.Job, error)

// listedPrices is the step that sets the resolved buy price and its source on the entries of an option family
// (docs/design/buy-price-source.md Decision 4); a listedJob of any other group has none.
type listedPrices func(context.Context, []jobs.WatchEntry) ([]jobs.WatchEntry, error)

// listedKeepDays is the number of most recently planned days whose snapshots a listedJob keeps.
const listedKeepDays = 7

// planSnap is one Plan of a day: its key set and the inner job that planned it.
type planSnap struct {
	keys  map[contracts.ExpectedKey]bool
	inner contracts.Job
}

// listedJob is the decorator of docs/design/underlyings.md Decision 9: it rebuilds the inner job from the
// current list at each Plan, and a Run keeps the inner job of the Plan it came from.
type listedJob struct {
	build  listedBuild
	group  string
	reader underlying.Reader
	held   func(context.Context) (map[string]bool, error) // names held, by row Key; read once per Plan
	clock  clock.Clock
	prices listedPrices // nil for every group but groupOptions

	mu     sync.Mutex
	newest contracts.Job                // answers ID and the optional methods
	snaps  map[contracts.Day][]planSnap // per day, oldest first; at most listedKeepDays days
	order  []contracts.Day              // the days of snaps, the day planned longest ago first
}

// newListedJob builds the inner job once from the list at start, so that the schedule and the optional methods
// are answered before the first Plan. A read error at start is returned.
func newListedJob(ctx context.Context, build listedBuild, group string, reader underlying.Reader,
	held func(context.Context) (map[string]bool, error), clk clock.Clock, prices listedPrices) (*listedJob, error) {
	l := &listedJob{build: build, group: group, reader: reader, held: held, clock: clk, prices: prices}
	entries, _, err := l.entries(ctx)
	if err != nil {
		return nil, err
	}
	inner, err := l.buildInner(entries)
	if err != nil {
		return nil, err
	}
	l.newest = inner
	return l, nil
}

// entries reads the list and the held names once and returns the entries of the group, and whether the store
// holds an active name.
func (l *listedJob) entries(ctx context.Context) ([]jobs.WatchEntry, bool, error) {
	rows, err := l.reader.Current(ctx)
	if err != nil {
		return nil, false, &listedError{kind: contracts.Retryable, msg: "read underlying list failed", err: err}
	}
	held, err := l.held(ctx)
	if err != nil {
		return nil, false, &listedError{kind: contracts.Retryable, msg: "read held names failed", err: err}
	}
	var out []jobs.WatchEntry
	anyActive := false
	for _, r := range rows {
		if r.Status == underlying.StatusActive {
			anyActive = true
		}
		if slices.Contains(followedBy(r, held[r.Key], held["option:"+r.Key]), l.group) {
			out = append(out, watchEntry(r))
		}
	}
	if l.prices != nil {
		if out, err = l.prices(ctx, out); err != nil {
			return nil, false, &listedError{kind: contracts.Retryable, msg: "read valuation failed", err: err}
		}
	}
	return out, anyActive, nil
}

func (l *listedJob) buildInner(entries []jobs.WatchEntry) (contracts.Job, error) {
	at := l.clock.Now().UTC().Format(time.RFC3339)
	inner, err := l.build(entries)
	if err != nil {
		return nil, &listedError{kind: contracts.Permanent, msg: fmt.Sprintf("build %s job at %s failed", l.group, at), err: err}
	}
	return inner, nil
}

// ID is the id of the newest inner job.
func (l *listedJob) ID() string { return l.current().ID() }

func (l *listedJob) current() contracts.Job {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.newest
}

// Plan reads the list now, builds a new inner job from it and plans with it. An empty store is a permanent
// error; a read error is retryable and plans nothing.
func (l *listedJob) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	entries, anyActive, err := l.entries(ctx)
	if err != nil {
		return nil, "", err
	}
	if !anyActive {
		return nil, "", &listedError{kind: contracts.Permanent, msg: "underlying list is empty"}
	}
	inner, err := l.buildInner(entries)
	if err != nil {
		return nil, "", err
	}
	keys, hash, err := inner.Plan(ctx, day)
	if err != nil {
		return nil, "", err
	}
	set := make(map[contracts.ExpectedKey]bool, len(keys))
	for _, k := range keys {
		set[k] = true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.snaps == nil {
		l.snaps = map[contracts.Day][]planSnap{}
	}
	l.snaps[day] = append(l.snaps[day], planSnap{keys: set, inner: inner})
	if i := slices.Index(l.order, day); i >= 0 {
		l.order = slices.Delete(l.order, i, i+1)
	}
	l.order = append(l.order, day) // most recently planned last
	if len(l.snaps) > listedKeepDays {
		// the day planned longest ago goes first; the day just planned is last in order and stays
		for _, d := range l.order[:len(l.order)-listedKeepDays] {
			delete(l.snaps, d)
		}
		l.order = slices.Clone(l.order[len(l.order)-listedKeepDays:])
	}
	l.newest = inner
	return keys, hash, nil
}

// Run uses the newest snapshot of day whose key set holds all keys.
func (l *listedJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	l.mu.Lock()
	var inner contracts.Job
	snaps := l.snaps[day]
	for i := len(snaps) - 1; i >= 0 && inner == nil; i-- {
		if snaps[i].holds(keys) {
			inner = snaps[i].inner
		}
	}
	l.mu.Unlock()
	if inner == nil {
		var terr contracts.TypedError
		if inner, terr = l.replan(ctx, day, keys); terr != nil {
			return contracts.JobResult{Err: terr}
		}
	}
	return inner.Run(ctx, day, keys)
}

// replan builds an inner job from the list now and returns it when its plan for day holds all keys; the
// snapshots are not touched. A plan that lacks a key is a Permanent "no plan for these keys".
func (l *listedJob) replan(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) (contracts.Job, contracts.TypedError) {
	noPlan := &listedError{kind: contracts.Permanent, msg: "no plan for these keys"}
	entries, _, err := l.entries(ctx)
	if err != nil {
		return nil, asTyped(err)
	}
	inner, err := l.buildInner(entries)
	if err != nil {
		return nil, asTyped(err)
	}
	planned, _, err := inner.Plan(ctx, day)
	if err != nil {
		return nil, noPlan
	}
	set := make(map[contracts.ExpectedKey]bool, len(planned))
	for _, k := range planned {
		set[k] = true
	}
	if !(planSnap{keys: set}).holds(keys) {
		return nil, noPlan
	}
	return inner, nil
}

func (s planSnap) holds(keys []contracts.ExpectedKey) bool {
	for _, k := range keys {
		if !s.keys[k] {
			return false
		}
	}
	return true
}

// Schedule is the schedule of the newest inner job.
func (l *listedJob) Schedule() (calendarID, anchor, zone string, offset time.Duration) {
	if s, ok := l.current().(interface {
		Schedule() (string, string, string, time.Duration)
	}); ok {
		return s.Schedule()
	}
	return "", "", "", 0
}

// RunBound is the bound of the newest inner job.
func (l *listedJob) RunBound() time.Duration {
	if s, ok := l.current().(interface{ RunBound() time.Duration }); ok {
		return s.RunBound()
	}
	return 0
}

// RestartSensitive is the flag of the newest inner job.
func (l *listedJob) RestartSensitive() bool {
	if s, ok := l.current().(interface{ RestartSensitive() bool }); ok {
		return s.RestartSensitive()
	}
	return false
}

// asTyped returns the TypedError err is or wraps, or nil when it is none.
func asTyped(err error) contracts.TypedError {
	te, _ := errors.AsType[contracts.TypedError](err)
	return te
}
