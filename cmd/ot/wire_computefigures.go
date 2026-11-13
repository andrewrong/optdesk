package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/techstate"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7; docs/design/valuation.md#valuation-wire-us.

func init() {
	registerJob(jobFamily{
		id: "compute_figures", order: 70,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			if fd, ok := fundamentalsOf(a.deps); ok {
				if err := openFacts(ctx, jc, fd); err != nil {
					return nil, "", err
				}
				jc.f.fd = fd
			}
			j, err := jc.f.computeFigures()
			return j, "", err
		},
	})
}

// computeFigures builds compute_figures. Without the fact store it is the job with the ease figure and no
// valuation step. With it, the valuation constructor has no ease input, so the job is the pair of that job and the
// ease job, run as one (withEase): the plan and the readings of both, and nothing an existing job stored is lost.
func (f jobFactory) computeFigures() (contracts.Job, error) {
	return f.listed(groupFundamentals, f.computeFiguresOf)
}

// computeFiguresOf builds the job over the fundamentals entries of one Plan; only the valuation step reads them.
func (f jobFactory) computeFiguresOf(entries []jobs.WatchEntry) (contracts.Job, error) {
	ease, err := f.d.newComputeFigures(f.compute, f.d.computeFigures, f.d.computeRules, f.d.computeUnits, f.d.computeSeriesOps, f.d.computeEase, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build compute_figures job failed: %w", err)
	}
	if f.facts == nil || f.fd.newFigures == nil {
		return computefigures.WithTech(ease, techDeps()), nil
	}
	fn := f.fd.fn
	val, err := f.fd.newFigures(f.compute, f.d.computeFigures, f.d.computeRules, f.d.computeUnits, f.d.computeSeriesOps,
		computefigures.ValuationDeps{
			Facts: f.facts, Entries: entries, StandardOf: fn.StandardOf, BuildFilings: fn.BuildFilings,
			Band: fn.Band, Questions: fn.Questions, Diagnosis: fn.Diagnosis,
		}, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build compute_figures job failed: %w", err)
	}
	return withEase(computefigures.WithTech(val, techDeps()), ease)
}

// techDeps is the tech step of compute_figures: the trial constants and the classifier. WithTech leaves the job
// as it is when the compute stores have no price-bar or index reader.
func techDeps() computefigures.TechDeps {
	return computefigures.TechDeps{Classify: techstate.Classify, Params: techstate.TrialParams()}
}

// scheduledJob is what the scheduler asks of compute_figures besides contracts.Job.
type scheduledJob interface {
	contracts.Job
	Schedule() (calendarID, anchor, zone string, offset time.Duration)
	RunBound() time.Duration
	RestartSensitive() bool
	Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error)
}

// easeJoined is compute_figures as the valuation job (val) plus the ease job: the plan is the union of the two
// (the keys only the ease job plans are the ease keys), a run hands each key to the job that planned it. The
// schedule and the postcondition are val's; the postcondition reads the readings by key, the same for both.
type easeJoined struct {
	scheduledJob
	ease contracts.Job
}

// withEase joins val and ease; val must have the scheduler methods (computefigures jobs have).
func withEase(val, ease contracts.Job) (contracts.Job, error) {
	sj, ok := val.(scheduledJob)
	if !ok {
		return nil, fmt.Errorf("build compute_figures job failed: %T has no schedule", val)
	}
	return easeJoined{scheduledJob: sj, ease: ease}, nil
}

// easeOnly returns the keys only the ease job plans for day, and the plan hash of the ease job.
func (e easeJoined) easeOnly(ctx context.Context, day contracts.Day, have []contracts.ExpectedKey) (extra []contracts.ExpectedKey, hash string, err error) {
	keys, hash, err := e.ease.Plan(ctx, day)
	if err != nil {
		return nil, "", err
	}
	seen := make(map[string]bool, len(have))
	for _, k := range have {
		seen[k.SubjectKey] = true
	}
	for _, k := range keys {
		if !seen[k.SubjectKey] {
			extra = append(extra, k)
		}
	}
	return extra, hash, nil
}

// Plan is the plan of the valuation job plus the keys of the ease job it lacks; the hash covers both plans.
func (e easeJoined) Plan(ctx context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys, hash, err := e.scheduledJob.Plan(ctx, day)
	if err != nil {
		return nil, "", err
	}
	extra, eHash, err := e.easeOnly(ctx, day, keys)
	if err != nil || len(extra) == 0 {
		return keys, hash, err
	}
	keys = append(slices.Clone(keys), extra...)
	sort.Slice(keys, func(a, b int) bool { return keys[a].SubjectKey < keys[b].SubjectKey })
	sum := sha256.Sum256([]byte(hash + "\n" + eHash))
	return keys, hex.EncodeToString(sum[:]), nil
}

// Run hands the ease keys to the ease job and the rest to the valuation job, and adds the two results.
func (e easeJoined) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	valKeys, _, perr := e.scheduledJob.Plan(ctx, day)
	var extra []contracts.ExpectedKey
	if perr == nil {
		extra, _, perr = e.easeOnly(ctx, day, valKeys)
	}
	if perr != nil {
		return contracts.JobResult{Err: runError{perr}}
	}
	isEase := make(map[string]bool, len(extra))
	for _, k := range extra {
		isEase[k.SubjectKey] = true
	}
	var mine, theirs []contracts.ExpectedKey
	for _, k := range keys {
		if isEase[k.SubjectKey] {
			theirs = append(theirs, k)
		} else {
			mine = append(mine, k)
		}
	}
	var res contracts.JobResult
	if len(mine) > 0 {
		res = e.scheduledJob.Run(ctx, day, mine)
	}
	if len(theirs) > 0 {
		r := e.ease.Run(ctx, day, theirs)
		res.Returned += r.Returned
		res.Usable += r.Usable
		if r.Err != nil && (res.Err == nil || (res.Err.Kind() != contracts.Retryable && r.Err.Kind() == contracts.Retryable)) {
			res.Err = r.Err
		}
	}
	return res
}

// runError is a retryable error of the plan a run needs before it hands out its keys.
type runError struct{ error }

func (runError) Kind() contracts.ErrorKind { return contracts.Retryable }
