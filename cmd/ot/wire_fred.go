package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

func init() {
	registerJob(jobFamily{
		id: jobFredSeries, order: 10,
		startOff: func(cfg runOnceConfig, _ bool) string {
			if cfg.fredKey == "" {
				return reasonNoFredKey
			}
			return ""
		},
		build: func(_ context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.fred()
			return j, "", err
		},
	})
}

func (f jobFactory) fred() (contracts.Job, error) {
	j, err := f.d.newFredSeries(f.src, f.ser, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build fred_series job failed: %w", err)
	}
	return j, nil
}
