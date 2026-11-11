package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/underlyingresolve"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// Design: docs/design/underlyings.md#9b ot-listed-jobs-wiring.

// jobUnderlyingResolve is the id of the underlying_resolve job: it settles the pending names of the stored list.
const jobUnderlyingResolve = "underlying_resolve"

// listStartTimeout bounds the read of the list that builds a job at start or in a probe, which have no context.
const listStartTimeout = 30 * time.Second

// startListed builds a listedJob with the read of the list that happens at start, under its own timeout: the builders
// of the families take no context. It is a variable so that the read stays outside the call chain of the builders.
var startListed = func(build listedBuild, group string, reader underlying.Reader,
	held func(context.Context) (map[string]bool, error), clk clock.Clock) (*listedJob, error) {
	ctx, cancel := context.WithTimeout(context.Background(), listStartTimeout)
	defer cancel()
	return newListedJob(ctx, build, group, reader, held, clk)
}

// heldAliases are the account aliases whose positions make a name held (the aliases optionopen reads).
// nameKey is the name_key of a row of the list: market, a colon and the upper-case symbol.
func nameKey(market, symbol string) string { return market + ":" + strings.ToUpper(symbol) }

var heldAliases = []string{"A", "B"}

func init() {
	registerJob(jobFamily{
		id: jobUnderlyingResolve, order: 200,
		// A job whose line is empty is left out whatever the broker does, so the broker reason is for a set line only.
		startOff: func(cfg runOnceConfig, brokerUp bool) string {
			if cfg.resolveDSN == "" {
				return ""
			}
			return brokerDownOff(cfg, brokerUp)
		},
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			if jc.cfg.resolveDSN == "" {
				a.log.Warn("underlying resolve job not wired", "component", "ot", "job", jobUnderlyingResolve, "status", underlying.DSNEnvResolve+" is not set")
				return nil, "", nil
			}
			if a.deps.openUnderlying == nil {
				return nil, "", nil
			}
			st, err := a.deps.openUnderlying(ctx, jc.cfg.resolveDSN)
			if err != nil {
				return nil, "", fmt.Errorf("open underlying store with %s failed", underlying.DSNEnvResolve)
			}
			jc.extra.addCloser(st)
			j, err := underlyingresolve.New(jc.brk, st, jc.f.clk)
			if err != nil {
				return nil, "", fmt.Errorf("build underlying_resolve job failed: %w", err)
			}
			return j, "", nil
		},
	})
}

// openJobList opens the reader of the stored list with the runner line (the view grant of ot_runner) and gives f the
// reader and the held reader. A deps without the opener leaves f.list nil, which listed refuses. What it opens goes to
// extra. No error holds a DSN.
func openJobList(ctx context.Context, a *app, cfg runOnceConfig, f *jobFactory, extra *closerSet) error {
	if a.deps.openUnderlying == nil {
		return nil
	}
	st, err := a.deps.openUnderlying(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open underlying list with %s failed", store.DSNEnvRunner)
	}
	extra.addCloser(st)
	f.list = st
	f.held = heldKeys(f.d, cfg, func() time.Time { return f.clk.Now() }, extra)
	return nil
}

// heldKeys reads the names held in the newest positions of heldAliases (a stock with a quantity other than zero,
// searched back heldLookbackDays from today), keyed as the rows of the list are: market, a colon and the symbol. A
// position carries no market here, so each held symbol is keyed under both markets. Without the account line nobody is
// held. The account store opens on the first read and closes through extra.
func heldKeys(d deps, cfg runOnceConfig, now func() time.Time, extra *closerSet) func(context.Context) (map[string]bool, error) {
	if cfg.accountDSN == "" || d.openAccount == nil {
		return func(context.Context) (map[string]bool, error) { return map[string]bool{}, nil }
	}
	rd := &lazyAccountReader{lazyStore[store.AccountStore]{open: func(ctx context.Context) (store.AccountStore, error) {
		s, err := d.openAccount(ctx, cfg.accountDSN)
		if err != nil {
			return nil, fmt.Errorf("open account store with %s failed", store.DSNEnvAccount)
		}
		return s, nil
	}}}
	extra.addCloser(rd)
	return func(ctx context.Context) (map[string]bool, error) {
		syms, err := heldSymbols(ctx, siteReaders{account: rd, aliases: heldAliases}, now())
		if err != nil {
			return nil, err
		}
		held := map[string]bool{}
		for sym := range syms {
			held[nameKey("US", sym)], held[nameKey("JP", sym)] = true, true
		}
		return held, nil
	}
}

// listed builds the job of one family as a listedJob: the entries of group come from the stored list at each Plan.
// The list is read once now to build the job the schedule and the optional methods are answered from.
func (f jobFactory) listed(group string, build listedBuild) (contracts.Job, error) {
	if f.list == nil || f.held == nil {
		return nil, errors.New("no underlying list reader")
	}
	// The read of the start builds the inner job the schedule is answered from, which no name changes: it asks nobody's
	// positions, so building the families opens no account store; every Plan after it reads the held names.
	var started atomic.Bool
	held := func(ctx context.Context) (map[string]bool, error) {
		if started.CompareAndSwap(false, true) {
			return map[string]bool{}, nil
		}
		return f.held(ctx)
	}
	j, err := startListed(build, group, f.list, held, f.clk)
	if err != nil {
		return nil, err
	}
	// RunKey marks a key by the postcondition when the job has one and by the rows it returned when it has not, so a
	// decorator may offer the method only when the inner job has it.
	if _, ok := j.current().(runner.Postcondition); ok {
		return listedPost{j}, nil
	}
	return j, nil
}

// listedPost is a listedJob over inner jobs that have a postcondition: it forwards it to the newest inner job.
type listedPost struct{ *listedJob }

func (p listedPost) Postcondition(ctx context.Context, day, datasetID, subjectKey string) (bool, error) {
	pc, ok := p.current().(runner.Postcondition)
	if !ok {
		return false, nil
	}
	return pc.Postcondition(ctx, day, datasetID, subjectKey)
}

// listedOf is the listedJob inside a job built by listed.
func listedOf(j contracts.Job) (*listedJob, bool) {
	switch v := j.(type) {
	case *listedJob:
		return v, true
	case listedPost:
		return v.listedJob, true
	}
	return nil, false
}
