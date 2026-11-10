package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/compute/ivbar"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/ivhistory"
)

// Design: docs/design/compute-stage2.md#8-iv-history-wire (Decision 11).

// jobIvHistory is the id of the iv_history job.
const jobIvHistory = "iv_history"

func init() {
	registerJob(jobFamily{
		id: jobIvHistory, order: 150,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			h := ivHistoryClient(ctx, jc)
			if h == nil {
				if jc.f.d.newHistory != nil && jc.f.compute.Readings != nil {
					a.log.Warn("job left out", "component", "ot", "job", jobIvHistory, "status", reasonBrokerDown)
				}
				return nil, "", nil
			}
			j, err := jc.f.ivHistory(h)
			return j, "", err
		},
	})
}

// ivHistoryClient is the history client the job gets, or nil when the job is left out. It is the client that
// bars_refresh and fx_refresh hold (one dial for the three jobs); when their step did not run it is dialled here,
// and closed through the closer set. With the broker down, or a bar line that left bars_refresh on its stand-in
// client, or without the history constructor or the reading store, there is none.
func ivHistoryClient(ctx context.Context, jc *jobCtx) broker.HistoryClient {
	f := jc.f
	if f.d.newHistory == nil || f.compute.Readings == nil || !jc.brokerUp {
		return nil
	}
	if f.hist != nil {
		if _, stub := f.hist.(unreachableHistory); stub {
			return nil
		}
		return f.hist
	}
	c, err := f.d.newHistory(ctx, jc.cfg.socketPath, broker.Options{})
	if err != nil {
		return nil
	}
	jc.extra.addCloser(c)
	return c
}

// ivHistory builds iv_history over the history client, the reading store opened for compute_figures and the
// watch list; the close of a bar becomes the stored text through the ivbar package.
func (f jobFactory) ivHistory(h broker.HistoryClient) (contracts.Job, error) {
	j, err := ivhistory.New(h, f.compute.Readings, f.compute.Readings, f.watch, ivbar.FromClose, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build iv_history job failed: %w", err)
	}
	return j, nil
}
