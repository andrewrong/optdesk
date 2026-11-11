package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

func init() {
	registerJob(jobFamily{
		id: jobDividendCapture, order: 40,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := a.dividendJob(ctx, jc.cfg, jc.f, jc.brokerUp, jc.extra)
			return j, "", err
		},
	})
}

// jobDividendCapture is the id of the dividend_capture job.
const jobDividendCapture = "dividend_capture"

// dividendCapture builds dividend_capture over the reference client, the instrument store opened for option_eod,
// the dividend store and the dividends names of the stored list.
func (f jobFactory) dividendCapture(ref broker.ReferenceClient) (contracts.Job, error) {
	return f.listed(groupDividends, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		j, err := f.d.newDividendCapture(ref, f.inst, f.div, f.div, entries, f.clk)
		if err != nil {
			return nil, fmt.Errorf("build dividend_capture job failed: %w", err)
		}
		return j, nil
	})
}

// dividendJob builds dividend_capture beside option_eod with the same broker state: with the broker down at start
// (the option jobs hold the stand-in) or without the events line the job is left out, with one named line, and the
// other jobs run as before. It is nil for a deps without the constructors. The dividend store and the reference
// client are closed by extra when withWiring returns. No error holds a DSN or a path.
func (a *app) dividendJob(ctx context.Context, cfg runOnceConfig, f *jobFactory, brokerUp bool, extra *closerSet) (contracts.Job, error) {
	d := f.d
	if d.newDividendCapture == nil || d.openDividend == nil || d.newReference == nil {
		return nil, nil
	}
	reason := ""
	switch {
	case !brokerUp:
		reason = reasonBrokerDown
	case cfg.eventsDSN == "":
		reason = reasonNoEventsDSN
	}
	if reason != "" {
		a.log.Warn("job left out", "component", "ot", "job", jobDividendCapture, "status", reason)
		return nil, nil
	}
	s, err := d.openDividend(ctx, cfg.eventsDSN)
	if err != nil {
		return nil, fmt.Errorf("open dividend store with %s failed", store.DSNEnvEvents)
	}
	extra.addCloser(s)
	f.div = s
	ref, err := d.newReference(ctx, cfg.socketPath, broker.Options{})
	if err != nil {
		a.log.Warn("job left out", "component", "ot", "job", jobDividendCapture, "status", reasonBrokerDown)
		return nil, nil
	}
	extra.addCloser(ref)
	return f.dividendCapture(ref)
}
