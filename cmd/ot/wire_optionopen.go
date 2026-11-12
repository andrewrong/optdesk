package main

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optionopen"
	"github.com/andrewrong/optdesk/internal/store/openquote"
	"github.com/andrewrong/optdesk/internal/store/optionopenpg"
)

// Design: docs/design/option-open-pull.md#Decisions (the wiring of the four US jobs; Decision 22 (d) and
// docs/design/scheduler.md Decision 8 for a left-out job).

// The ids of the four US option open jobs, the line of the first-day resolve and the reasons they are left out.
const (
	jobOptionOpenResolve    = "option_open_resolve"
	jobOptionOpenResolveGap = "option_open_resolve_gap"
	jobOptionOpenPlan       = "option_open_plan"
	jobOptionOpen           = "option_open"

	// The ids of the three Japan jobs (Decision 21): resolve, plan and pull over XTKS.
	jobOptionOpenResolveJP = "option_open_resolve_jp"
	jobOptionOpenPlanJP    = "option_open_plan_jp"
	jobOptionOpenJP        = "option_open_jp"

	// envOpenResolveMaxNew is read from the process environment: a positive value is passed as MaxNew to the
	// resolve jobs (unset or 0 keeps max_new of the manifest), so the window runs it once with 1500.
	envOpenResolveMaxNew = "OT_OPEN_RESOLVE_MAX_NEW"

	reasonNoOpenDSN     = openquote.DSNEnvOptionOpen + " is not set"
	reasonOpenStoreFail = "the option open store could not be opened"
)

func init() {
	// Orders 120, 130, 140 and 190: resolve, plan, pull, gap pass. A job that cannot be wired is held with its
	// reason (the scheduler lists it as disabled, run-once leaves it out); a dead socket is the startOff.
	registerJob(jobFamily{
		id: jobOptionOpenResolve, order: 120, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenResolve(ctx, a, jc, jobOptionOpenResolve, optionopen.NewResolve, true, broker.Options{ResolveInFlight: 1})
		},
	})
	registerJob(jobFamily{
		id: jobOptionOpenPlan, order: 130, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenPlan(ctx, a, jc, jobOptionOpenPlan, optionopen.NewPlan)
		},
	})
	registerJob(jobFamily{
		id: jobOptionOpen, order: 140, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenPull(ctx, a, jc, jobOptionOpen, optionopen.NewPull)
		},
	})
	registerJob(jobFamily{
		id: jobOptionOpenResolveGap, order: 190, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenResolve(ctx, a, jc, jobOptionOpenResolveGap, optionopen.NewGap, false, broker.Options{ResolveInFlight: 1})
		},
	})
	// Orders 165, 170 and 180: the Japan resolve, plan and pull, with the rules of the US families.
	registerJob(jobFamily{
		id: jobOptionOpenResolveJP, order: 165, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenResolve(ctx, a, jc, jobOptionOpenResolveJP, optionopen.NewResolveJP, true, broker.Options{ResolveInFlight: 1})
		},
	})
	registerJob(jobFamily{
		id: jobOptionOpenPlanJP, order: 170, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenPlan(ctx, a, jc, jobOptionOpenPlanJP, optionopen.NewPlanJP)
		},
	})
	registerJob(jobFamily{
		id: jobOptionOpenJP, order: 180, startOff: ifOpenDSN(brokerDownOff),
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			return buildOpenPull(ctx, a, jc, jobOptionOpenJP, optionopen.NewPullJP)
		},
	})
}

// buildOpenPlan builds a plan job (US or JP) over the stored inputs and the option open store.
func buildOpenPlan(ctx context.Context, a *app, jc *jobCtx, id string,
	newJob func(optionopen.PlanDeps) (contracts.Job, error)) (contracts.Job, string, error) {
	stores, reason, ok := openOptionOpen(ctx, a, jc, id, true)
	if !ok {
		return nil, "", nil
	}
	f := openEntryFix(ctx, a, jc.f, id, jc.brk)
	j, err := f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		return newJob(optionopen.PlanDeps{
			Stored: openStored(jc, stores, f.fixedEntries(entries)), Marks: stores, Plans: stores, Clock: jc.f.clk, Func: optionopen.BuildPlan,
		})
	})
	return held(a, id, j, reason, err)
}

// buildOpenPull builds a pull job (US or JP) over the shared broker client.
func buildOpenPull(ctx context.Context, a *app, jc *jobCtx, id string,
	newJob func(optionopen.PullDeps) (contracts.Job, error)) (contracts.Job, string, error) {
	stores, reason, ok := openOptionOpen(ctx, a, jc, id, false)
	if !ok {
		return nil, "", nil
	}
	j, err := newJob(optionopen.PullDeps{
		Broker: jc.brk, Plans: stores, Contracts: stores, Writer: stores,
		Instruments: jc.f.inst, Clock: jc.f.clk, Log: jc.f.log,
	})
	return held(a, id, j, reason, err)
}

// buildOpenResolve builds the evening resolve (withPull) or the gap pass over a client of its own, dialled with
// opts (one resolve in flight), and the first-day limit of OT_OPEN_RESOLVE_MAX_NEW.
func buildOpenResolve(ctx context.Context, a *app, jc *jobCtx, id string,
	newJob func(optionopen.ResolveDeps, optionopen.ResolveOptions) (contracts.Job, error), withPull bool, opts broker.Options) (contracts.Job, string, error) {
	stores, reason, ok := openOptionOpen(ctx, a, jc, id, true)
	if !ok {
		return nil, "", nil
	}
	brk := jc.brk
	if jc.brokerUp {
		c, err := a.deps.dialBroker(ctx, jc.cfg.socketPath, opts)
		if err != nil {
			reason = firstReason(reason, reasonBrokerDown)
		} else {
			jc.extra.add(c)
			brk = c
		}
	}
	f := openEntryFix(ctx, a, jc.f, id, brk)
	j, err := f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		d := optionopen.ResolveDeps{
			Stored: openStored(jc, stores, f.fixedEntries(entries)), Broker: brk, Marks: stores, Clock: jc.f.clk,
			Log: func(line string) { a.log.Info(line, "component", "ot", "job", id) },
		}
		if withPull {
			d.Pull = stores
		}
		return newJob(d, optionopen.ResolveOptions{MaxNew: openResolveMaxNew(a)})
	})
	return held(a, id, j, reason, err)
}

// openEntryFix is a copy of f for the job id. For the Japan resolve and plan, whose watch entries may carry no
// con_id, the copy asks the broker b for it (resolveJPConIDs) on every Plan, as option_eod_jp does; any other id,
// or a nil broker, leaves entryFix as f has it.
func openEntryFix(ctx context.Context, a *app, f *jobFactory, id string, b broker.Broker) jobFactory {
	c := *f
	if (id == jobOptionOpenResolveJP || id == jobOptionOpenPlanJP) && b != nil {
		c.entryFix = func(es []jobs.WatchEntry) []jobs.WatchEntry { return resolveJPConIDs(ctx, b, es, a.log, id) }
	}
	return c
}

// openResolveMaxNew is the value of OT_OPEN_RESOLVE_MAX_NEW; unset, 0, negative or not a number is 0 (the manifest).
func openResolveMaxNew(a *app) int {
	n, err := strconv.Atoi(a.getenv(envOpenResolveMaxNew))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// held logs the one startup line of a job that is held and returns it with its reason, so that the scheduler
// lists it as disabled and run-once leaves it out. A job that cannot be built is a start error.
func held(a *app, id string, j contracts.Job, reason string, err error) (contracts.Job, string, error) {
	if err != nil {
		return nil, "", err
	}
	if reason != "" {
		a.log.Warn("option open job held", "component", "ot", "job", id, "status", reason)
	}
	return j, reason, nil
}

func firstReason(rs ...string) string {
	for _, r := range rs {
		if r != "" {
			return r
		}
	}
	return ""
}

// openStored is the stored inputs of the plan and the resolve jobs: the options names of one Plan, the held ports and the
// instrument store of the factory. Eod is the reader of the compute stores (the pages role, which may read
// rd.option_eod_r1); the writer role of the eod store has no right on schema rd.
func openStored(jc *jobCtx, stores openquote.Stores, entries []jobs.WatchEntry) optionopen.Stored {
	s := optionopen.Stored{Watch: entries, Instruments: jc.f.inst, Options: stores, Accounts: jc.f.accounts}
	if jc.f.bars != nil {
		s.Bars = jc.f.bars
	}
	if jc.f.compute.OptionEod != nil {
		s.Eod = jc.f.compute.OptionEod
	}
	return s
}

// openOptionOpen checks the inputs of the family and opens the store of optionopenpg with the DSN of the env file
// (jc.cfg.openDSN), closed through jc.extra. A missing line, a missing instrument store (or, with needBars, a missing
// bar store) leaves the family out: ok is false after one startup line naming the job and the line. Held (the
// reason with the job) cannot carry it: wiring.runnable hashes every job once one is held, and a job whose type is
// not hashable would panic there; the scheduler's disabled list therefore lacks these ids until that is fixed.
// A store that cannot be opened is held with its reason.
func openOptionOpen(ctx context.Context, a *app, jc *jobCtx, id string, needBars bool) (stores openquote.Stores, reason string, ok bool) {
	dsn := jc.cfg.openDSN
	left := func(why string) (openquote.Stores, string, bool) {
		a.log.Warn("option open job not wired", "component", "ot", "job", id, "status", why)
		return nil, "", false
	}
	switch {
	case dsn == "":
		return left(reasonNoOpenDSN)
	case jc.f.inst == nil:
		return left(reasonBuildJob)
	case needBars && jc.f.bars == nil:
		return left(reasonNoBarDSN)
	}
	s, err := optionopenpg.Open(ctx, dsn)
	if err != nil {
		return noOpenStore{err: errors.New(reasonOpenStoreFail)}, reasonOpenStoreFail, true
	}
	jc.extra.addCloser(s)
	return s, "", true
}

// noOpenStore stands in for the option open store of a held job: every call answers its error.
type noOpenStore struct{ err error }

func (s noOpenStore) InsertOptionOpen(context.Context, []contracts.OptionOpen) error { return s.err }
func (s noOpenStore) OptionOpen(context.Context, contracts.Day) ([]contracts.OptionOpen, error) {
	return nil, s.err
}
func (s noOpenStore) PutPlan(context.Context, []contracts.OptionOpenItem) error { return s.err }
func (s noOpenStore) Plan(context.Context, contracts.Day, string, ...int64) ([]contracts.OptionOpenItem, error) {
	return nil, s.err
}
func (s noOpenStore) AddNotListed(context.Context, []contracts.OptionNotListed) error { return s.err }
func (s noOpenStore) NotListed(context.Context, time.Time) ([]contracts.OptionNotListed, error) {
	return nil, s.err
}
func (s noOpenStore) RemoveNotListed(context.Context, openquote.NotListedKey) (bool, error) {
	return false, s.err
}
func (s noOpenStore) RemoveNotListedByUnderlying(context.Context, int64) (int, error) {
	return 0, s.err
}
func (s noOpenStore) RemoveNotListedExpired(context.Context, contracts.Day) (int, error) {
	return 0, s.err
}
func (s noOpenStore) PutOption(context.Context, openquote.OptionRef) error { return s.err }
func (s noOpenStore) ListOptions(context.Context, int64) ([]openquote.OptionRef, error) {
	return nil, s.err
}
func (s noOpenStore) Close() error { return nil }

var _ openquote.Stores = noOpenStore{}

// ifOpenDSN is the startOff of the four families: off (by off) only when the DSN line is set. A family without
// the line is left out and has no job, so its id must not be listed as disabled (the tests that count the
// disabled ids rely on it). The line is read from the env file, as cfg.openDSN, like every other job's.
func ifOpenDSN(off func(runOnceConfig, bool) string) func(runOnceConfig, bool) string {
	return func(cfg runOnceConfig, brokerUp bool) string {
		if cfg.openDSN == "" {
			return ""
		}
		return off(cfg, brokerUp)
	}
}
