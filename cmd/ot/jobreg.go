package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 to 4.

// jobFamily is one job of ot (or two that share a build, in one file) as the registry holds it.
type jobFamily struct {
	// id is the job id; order places the family in the wired list (steps of ten, unique, see the design).
	id    string
	order int
	// startOff is the reason the family is disabled before any store is open (what startDisabled reports);
	// nil means never.
	startOff func(cfg runOnceConfig, brokerUp bool) string
	// build returns (job, "", nil) for a wired job, (nil, "", nil) for an absent family (the family logs its own
	// "job left out" line), and (job, reason, nil) for a job that is held: kept for the scheduler, left out of
	// run-once and backfill with one line carrying the reason.
	build func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error)
}

// jobCtx is what the families share while wireJobs builds them in order. It names no family.
type jobCtx struct {
	cfg runOnceConfig
	// f is the one factory of the run; families that set its fields (stores they open) do it here, wireJobs
	// copies it into wiring once after the loop.
	f *jobFactory
	// brk is the dialled broker (the stand-in when brokerUp is false).
	brk      broker.Broker
	brokerUp bool
	cal      clock.Calendar
	extra    *closerSet
}

var jobFamilies = map[string]jobFamily{}

// registerJob adds f; it panics on a second family with the same id or the same order, at start.
func registerJob(f jobFamily) {
	if f.id == "" || f.build == nil {
		panic("registerJob: a family needs an id and a build")
	}
	for _, g := range jobFamilies {
		if g.id == f.id {
			panic(fmt.Sprintf("registerJob: duplicate job id %q", f.id))
		}
		if g.order == f.order {
			panic(fmt.Sprintf("registerJob: job %q and %q share order %d", f.id, g.id, f.order))
		}
	}
	jobFamilies[f.id] = f
}

// sortedJobFamilies returns the registered families by order.
func sortedJobFamilies() []jobFamily {
	out := make([]jobFamily, 0, len(jobFamilies))
	for _, f := range jobFamilies {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].order < out[j].order })
	return out
}
