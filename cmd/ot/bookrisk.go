package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/bookrisk"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
	"github.com/andrewrong/optdesk/internal/strategy/risk/bsm"
	"github.com/andrewrong/optdesk/internal/strategy/risk/fit"
	"github.com/andrewrong/optdesk/internal/strategy/risk/stress"
)

// Design: docs/design/strategy-risk.md#5 ot-wire.

func init() {
	// book_risk follows option_candidates (order 110): it reads the rows of the day that job writes. Its deps are
	// ext deps (depsext.go), so deps and newDeps are not edited.
	extDeps[extBookRisk] = func() any { return bookRiskDeps{newJob: bookrisk.New} }
	registerJob(jobFamily{
		id: jobBookRisk, order: 210,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := a.bookRiskJob(ctx, jc.cfg, *jc.f, jc.extra)
			return j, "", err
		},
	})
}

const (
	jobBookRisk = "book_risk"

	// extBookRisk is the key of bookRiskDeps in deps.ext.
	extBookRisk = "bookrisk"
)

// bookRiskDeps is the ext deps of book_risk: the constructor of the job. A missing entry or a nil constructor
// leaves the job out.
type bookRiskDeps struct {
	newJob func(bookrisk.Stores, clock.Clock) (contracts.Job, error)
}

// bookRiskKeys are the lines book_risk needs, in the order a missing one is named.
func bookRiskKeys(cfg runOnceConfig) (missing string) {
	for _, k := range []struct{ name, val string }{
		{store.DSNEnvCandidate, cfg.candidateDSN},
		{store.DSNEnvOptionLedger, cfg.ledgerDSN},
		{store.DSNEnvAccount, cfg.accountDSN},
	} {
		if k.val == "" {
			return k.name
		}
	}
	return ""
}

// bookRiskJob builds book_risk: the pricer, the stresser and the fitter of internal/strategy/risk (the one place
// that imports them) and the stores, opened as candidates.go opens its own. It is nil, with one logged reason that
// names the line and never a value, for a deps without the constructor or an empty required line; the other jobs
// are unaffected. An open that fails names the key. What it opens goes to extra.
func (a *app) bookRiskJob(ctx context.Context, cfg runOnceConfig, f jobFactory, extra *closerSet) (contracts.Job, error) {
	bd, ok := f.d.ext[extBookRisk].(bookRiskDeps)
	if !ok || bd.newJob == nil || f.d.openStrategyResults == nil || f.d.openOptionLedgerStore == nil ||
		f.d.openAccount == nil || f.d.newOptionLedger == nil || f.d.newOptionStructures == nil {
		return nil, nil
	}
	if key := bookRiskKeys(cfg); key != "" {
		a.log.Warn("job left out", "component", "ot", "job", jobBookRisk, "status", key+" is not set")
		return nil, nil
	}
	book, err := a.loadBook()
	if err != nil {
		return nil, err
	}
	res, err := f.d.openStrategyResults(ctx, cfg.candidateDSN)
	if err != nil {
		return nil, fmt.Errorf("open strategy results with %s failed", store.DSNEnvCandidate)
	}
	extra.addCloser(res)
	led, err := f.d.openOptionLedgerStore(ctx, cfg.ledgerDSN)
	if err != nil {
		return nil, fmt.Errorf("open option_ledger store with %s failed", store.DSNEnvOptionLedger)
	}
	extra.addCloser(led)
	acct, err := f.d.openAccount(ctx, cfg.accountDSN)
	if err != nil {
		return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
	}
	extra.addCloser(acct)
	var marks mark.Reader
	if env, err := a.envFile(); err == nil && f.d.openMark != nil && env[mark.DSNEnvPositionMark] != "" {
		m, err := f.d.openMark(ctx, env[mark.DSNEnvPositionMark])
		if err != nil {
			return nil, fmt.Errorf("open mark store with %s failed", mark.DSNEnvPositionMark)
		}
		extra.addCloser(m)
		marks = m
	}
	stresser := stress.New(bsm.New())
	j, err := bd.newJob(bookrisk.Stores{
		Aliases: bookAliasList(book), Account: acct, Ledger: led, SessionDays: f.compute.SessionDays,
		Ledgers: f.d.newOptionLedger, Structures: f.d.newOptionStructures,
		OptionEod: f.compute.OptionEod, Terms: f.compute.Terms, Marks: marks, Series: f.compute.Series,
		Results: res, Stresser: stresser, Fitter: fit.New(stresser),
	}, f.clk)
	if err != nil {
		return nil, fmt.Errorf("build book_risk job failed: %w", err)
	}
	return j, nil
}
