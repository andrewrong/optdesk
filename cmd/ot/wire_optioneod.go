package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioneod"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/symbol"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

func brokerDownOff(_ runOnceConfig, brokerUp bool) string {
	if !brokerUp {
		return reasonBrokerDown
	}
	return ""
}

func init() {
	registerJob(jobFamily{
		id: jobOptionEod, order: 20, startOff: brokerDownOff,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.optionEodWith(jc.brk, eodStoredReader(ctx, a, jc))
			return j, "", err
		},
	})
	// option_eod_jp is left out, with one named line, when the XTKS calendar answers an error for today (Decision 13).
	registerJob(jobFamily{
		id: jobOptionEodJP, order: 30, startOff: brokerDownOff,
		build: func(ctx context.Context, a *app, jc *jobCtx) (contracts.Job, string, error) {
			if reason := xtksReason(ctx, jc.cal, jc.f.clk); reason != "" {
				a.log.Warn(reason, "component", "ot", "job", jobOptionEodJP)
				return nil, "", nil
			}
			// A JP name of the list that is not resolved yet carries no con_id: ask the broker for it, as bars_refresh does.
			f := *jc.f
			f.entryFix = func(es []jobs.WatchEntry) []jobs.WatchEntry {
				return resolveJPConIDs(ctx, jc.brk, es, a.log, jobOptionEodJP)
			}
			j, err := f.optionEodJP(jc.brk)
			return j, "", err
		},
	})
}

// resolveJPConIDs returns a copy of watch in which every JP stock entry without a con_id has the con_id the broker
// answers to one lookup. An entry whose lookup fails, is not listed or answers con_id 0 stays as it is, with one Warn
// line naming job (the job that ran it); the job then answers no_value (or an error) for that stock only. Every other entry and the input are left untouched.
func resolveJPConIDs(ctx context.Context, b broker.Broker, watch []jobs.WatchEntry, log *slog.Logger, job string) []jobs.WatchEntry {
	out := make([]jobs.WatchEntry, len(watch))
	copy(out, watch)
	for i, e := range out {
		if e.Market != "JP" || e.Contract.SecType != "STK" || e.Contract.ConID != 0 {
			continue
		}
		got, err := b.ResolveContract(ctx, e.Contract)
		reason := ""
		switch {
		case err != nil:
			reason = err.Error()
		case !got.Listed:
			reason = "not listed"
		case got.Contract.ConID == 0:
			reason = "answer has no con_id"
		}
		if reason != "" {
			log.Warn(job+": con_id of watch stock not resolved", "component", "ot", "job", job,
				"symbol", e.Contract.Symbol, "reason", reason)
			continue
		}
		out[i].Contract.ConID = got.Contract.ConID
	}
	return out
}

// optionEod builds option_eod with no stored-contract reader: preflight and the dry run use it.
func (f jobFactory) optionEod(b broker.Broker) (contracts.Job, error) {
	return f.optionEodWith(b, nil)
}

// optionEodWith builds option_eod; a non-nil rd gives the job the contracts the evening resolve stored, applied inside
// the callback because listed rebuilds the job for each run.
func (f jobFactory) optionEodWith(b broker.Broker, rd optioneod.ContractReader) (contracts.Job, error) {
	return f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		j, err := buildOptionEod(f.d, b, f.inst, f.eod, entries, f.clk)
		j, err = withOptionEodReasons(j, err, f.reasons)
		if err == nil {
			j, err = f.withCalendar(j)
		}
		if err != nil || rd == nil {
			return j, err
		}
		return optioneod.WithContracts(j, rd, f.inst)
	})
}

// eodStoredReader opens the option open store for option_eod (closed through jc.extra) and returns it as the reader of
// stored contracts. It returns nil, with one Warn line naming the job and the reason (never the DSN), when there is no
// DSN, no opener, no instrument store or the store does not open: the job then runs as before.
func eodStoredReader(ctx context.Context, a *app, jc *jobCtx) optioneod.ContractReader {
	why := ""
	switch {
	case jc.cfg.openDSN == "":
		why = reasonNoOpenDSN
	case jc.f.d.openOptionOpen == nil || jc.f.inst == nil:
		why = reasonOpenStoreFail
	}
	if why == "" {
		s, err := jc.f.d.openOptionOpen(ctx, jc.cfg.openDSN)
		if err == nil && s != nil {
			jc.extra.addCloser(s)
			return s
		}
		why = reasonOpenStoreFail
	}
	a.log.Warn("option_eod reads no stored contracts", "component", "ot", "job", jobOptionEod, "status", why)
	return nil
}

// buildOptionEod builds option_eod over the watch list: the one constructor path of run-once, the scheduler and
// the dry run.
func buildOptionEod(d deps, b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter,
	watch []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	build := d.newOptionEodWatch
	if build == nil {
		// A deps literal of a test that fakes only newOptionEod; newDeps always sets newOptionEodWatch.
		build = func(b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter,
			_ []jobs.WatchEntry, c clock.Clock, t symbol.Translator) (contracts.Job, error) {
			return d.newOptionEod(b, r, w, c, t)
		}
	}
	j, err := build(b, r, w, watch, clk, d.symbols)
	if err != nil {
		return nil, fmt.Errorf("build option_eod job failed: %w", err)
	}
	return j, nil
}

// optionEodJP builds option_eod_jp over the same stores and the same options names as option_eod; entryFix, when
// set, is applied to the names of each Plan.
func (f jobFactory) optionEodJP(b broker.Broker) (contracts.Job, error) {
	return f.listed(groupOptions, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		j, err := buildOptionEodJP(f.d, b, f.inst, f.eod, f.fixedEntries(entries), f.clk)
		j, err = withOptionEodReasons(j, err, f.reasons)
		if err != nil {
			return j, err
		}
		return f.withCalendar(j)
	})
}

// withCalendar gives j the session calendar of the factory, which a close-anchored slot needs to tell an early run; a
// nil calendar (dry run, preflight) leaves j as it is.
func (f jobFactory) withCalendar(j contracts.Job) (contracts.Job, error) {
	if f.cal == nil {
		return j, nil
	}
	return optioneod.WithCalendar(j, f.cal)
}

// fixedEntries is es after entryFix when it is set, es itself otherwise.
func (f jobFactory) fixedEntries(es []jobs.WatchEntry) []jobs.WatchEntry {
	if f.entryFix != nil {
		return f.entryFix(es)
	}
	return es
}

// buildOptionEodJP builds option_eod_jp over the watch list (the JP stock entries add keys): the one constructor
// path of run-once, the scheduler and the dry run.
func buildOptionEodJP(d deps, b broker.Broker, r store.InstrumentResolver, w store.OptionEodWriter,
	watch []jobs.WatchEntry, clk clock.Clock) (contracts.Job, error) {
	var j contracts.Job
	var err error
	if d.newOptionEodJPWatch != nil {
		j, err = d.newOptionEodJPWatch(b, r, w, watch, clk)
	} else {
		// A deps literal of a test that fakes only newOptionEodJP; newDeps always sets newOptionEodJPWatch.
		j, err = d.newOptionEodJP(b, r, w, clk)
	}
	if err != nil {
		return nil, fmt.Errorf("build option_eod_jp job failed: %w", err)
	}
	return j, nil
}
