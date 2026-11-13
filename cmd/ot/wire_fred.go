package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/fredseries"
	"github.com/andrewrong/optdesk/internal/sources/nyfed"
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

// fred builds the job with the NY Fed source as its second source, on every path. The deps hook
// newFredSeries is not used: wire.go is not edited, so it cannot carry the second source.
func (f jobFactory) fred() (contracts.Job, error) {
	return f.fredWithSecond(nyfed.Config{Clock: f.clk})
}

// fredWithSecond builds the job with the NY Fed source made from cfg as its second source.
func (f jobFactory) fredWithSecond(cfg nyfed.Config) (contracts.Job, error) {
	second, err := nyfed.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("build fred_series second source failed: %w", err)
	}
	j, err := fredseries.NewWithSecondary(f.src, second, f.ser, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build fred_series job failed: %w", err)
	}
	return j, nil
}
