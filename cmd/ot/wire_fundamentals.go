package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute/valuation"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/jobs/fundamentals"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/sec"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/fundamental"
)

// Design: docs/design/valuation.md#valuation-wire-us (Decisions 9 and 13).

// jobFundamentals is the id of the fundamentals job.
const jobFundamentals = "fundamentals"

// extFundamentals is the key of fundamentalsDeps in deps.ext.
const extFundamentals = "fundamentals"

// fundamentalsDeps are the constructors of the fundamentals job, of the valuation step of compute_figures and of
// the companies page: the SEC source, the job, the fact store, the figures job with a valuation step, and the
// compute functions. They live in deps.ext so that deps gains no field.
type fundamentalsDeps struct {
	newSEC     sources.NewSECFunc
	newJob     fundamentals.NewFundamentalsFunc
	openFacts  store.OpenFactStoreFunc
	newFigures computefigures.NewComputeFiguresValuationFunc
	fn         valuationFuncs
}

func init() {
	extDeps[extFundamentals] = func() any {
		return fundamentalsDeps{
			newSEC: sec.New, newJob: fundamentals.New, openFacts: fundamental.Open, newFigures: computefigures.NewWithValuation,
			fn: valuationFuncs{
				StandardOf: valuation.StandardOf, BuildFilings: valuation.BuildFilings, Band: valuation.Band,
				ImpliedGrowth: valuation.ImpliedGrowth, ReturnAtPrice: valuation.ReturnAtPrice,
				Questions: valuation.Questions, Diagnosis: valuation.Diagnosis,
			},
		}
	}
	registerJob(jobFamily{
		id: jobFundamentals, order: 160,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			fd, ok := fundamentalsOf(a.deps)
			if !ok || fd.newJob == nil || jc.cfg.factDSN == "" || jc.cfg.secAgent == "" {
				if ok && fd.newJob != nil {
					a.log.Warn("job left out", "component", "ot", "job", jobFundamentals, "status", fundamentalsOff(jc.cfg))
				}
				return nil, "", nil
			}
			if err := openFacts(ctx, jc, fd); err != nil || jc.f.facts == nil {
				return nil, "", err
			}
			src, err := newSEC(fd.newSEC, jc.cfg.secAgent, jc.f.clk)
			if err != nil || src == nil {
				return nil, "", err
			}
			j, err := fd.newJob(src, jc.f.facts, jc.f.watch, jc.f.clk)
			if err != nil {
				return nil, "", fmt.Errorf("build fundamentals job failed: %w", err)
			}
			return j, "", nil
		},
	})
}

// fundamentalsOf is the ext deps of the fundamentals job; false when deps hold none (a test deps).
func fundamentalsOf(d deps) (fundamentalsDeps, bool) {
	fd, ok := d.ext[extFundamentals].(fundamentalsDeps)
	return fd, ok
}

// fundamentalsOff is the reason the job is left out for the lines of cfg.
func fundamentalsOff(cfg runOnceConfig) string {
	if cfg.factDSN == "" {
		return reasonNoFactDSN
	}
	return reasonNoSECUserAgent
}

// openFacts opens the fact store once per run with the fundamental line and keeps it in the factory; the closer
// set closes it. Without the line or the opener it leaves facts nil.
func openFacts(ctx context.Context, jc *jobCtx, fd fundamentalsDeps) error {
	if jc.f.facts != nil || jc.cfg.factDSN == "" || fd.openFacts == nil {
		return nil
	}
	s, err := fd.openFacts(ctx, jc.cfg.factDSN)
	if err != nil {
		return fmt.Errorf("open fact store with %s failed", store.DSNEnvFundamental)
	}
	jc.extra.addCloser(s)
	jc.f.facts = s
	return nil
}
