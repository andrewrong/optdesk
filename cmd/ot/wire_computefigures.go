package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

func init() {
	registerJob(jobFamily{
		id: "compute_figures", order: 70,
		build: func(_ context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.computeFigures()
			return j, "", err
		},
	})
}

func (f jobFactory) computeFigures() (contracts.Job, error) {
	j, err := f.d.newComputeFigures(f.compute, f.d.computeFigures, f.d.computeRules, f.d.computeUnits, f.d.computeSeriesOps, f.d.computeEase, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build compute_figures job failed: %w", err)
	}
	return j, nil
}
