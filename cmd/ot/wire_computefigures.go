package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
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

// computeFigures builds compute_figures. With the fact store open it is the job with the valuation step, whose
// Entries are the watch list and whose StandardOf is the one the page uses; that constructor has no ease figure.
// Without the store it is the job with the ease figure and no valuation step.
func (f jobFactory) computeFigures() (contracts.Job, error) {
	var j contracts.Job
	var err error
	if f.facts != nil && f.fd.newFigures != nil {
		fn := f.fd.fn
		j, err = f.fd.newFigures(f.compute, f.d.computeFigures, f.d.computeRules, f.d.computeUnits, f.d.computeSeriesOps,
			computefigures.ValuationDeps{
				Facts: f.facts, Entries: f.watch, StandardOf: fn.StandardOf, BuildFilings: fn.BuildFilings,
				Band: fn.Band, Questions: fn.Questions, Diagnosis: fn.Diagnosis,
			}, f.clk)
	} else {
		j, err = f.d.newComputeFigures(f.compute, f.d.computeFigures, f.d.computeRules, f.d.computeUnits, f.d.computeSeriesOps, f.d.computeEase, f.clk)
	}
	if err != nil {
		return nil, fmt.Errorf("build compute_figures job failed: %w", err)
	}
	return j, nil
}
