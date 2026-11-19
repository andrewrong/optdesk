package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute/recon"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/reconbars"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
	"github.com/andrewrong/optdesk/internal/store/reconpg"
)

// Design: docs/design/base-data-page.md#23.

// jobReconBars is the id of the recon_bars job.
const jobReconBars = "recon_bars"

// reasonNoReconDSN is why recon_bars is held while the line of the recon writer is empty.
const reasonNoReconDSN = reconstore.DSNEnvRecon + " is not set (run setup pg)"

func init() {
	registerJob(jobFamily{
		id: jobReconBars, order: 230,
		startOff: func(cfg runOnceConfig, _ bool) string {
			if cfg.reconDSN == "" {
				return reasonNoReconDSN
			}
			return ""
		},
		build: func(ctx context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			if jc.cfg.reconDSN == "" {
				// The reason is the startOff of the family; the scheduler reports the job as disabled with it.
				return nil, "", nil
			}
			j, err := jc.f.reconBars(ctx, jc, reconpg.OpenWriter, reconbars.New)
			if err != nil {
				return nil, "", err
			}
			return j, jc.cfg.barsOff(), nil
		},
	})
}

// reconBars builds recon_bars over the bar source of its manifest, the instrument store, the bar store, the writer
// opened with the recon line and the bars names of the stored list; recon.Compare is the compare function. open and
// mk are the writer opener and the constructor of the job.
func (f jobFactory) reconBars(
	ctx context.Context, jc *jobCtx, open reconstore.OpenWriterFunc, mk reconbars.NewReconBarsFunc,
) (contracts.Job, error) {
	m, err := reconbars.DefaultManifest()
	if err != nil {
		return nil, fmt.Errorf("build recon_bars job failed: %w", err)
	}
	src, err := f.barSource(m.SourceID)
	if err != nil {
		return nil, fmt.Errorf("build recon_bars job failed: %w", err)
	}
	if f.bars == nil {
		return nil, fmt.Errorf("build recon_bars job failed: no bar store")
	}
	w, err := open(ctx, jc.cfg.reconDSN)
	if err != nil {
		return nil, fmt.Errorf("open recon writer with %s failed", reconstore.DSNEnvRecon)
	}
	jc.extra.addCloser(w)
	return f.listed(groupBars, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		j, err := mk(src, f.inst, f.bars, w, recon.Compare, entries, f.clk)
		if err != nil {
			return nil, fmt.Errorf("build recon_bars job failed: %w", err)
		}
		return j, nil
	})
}
