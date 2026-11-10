package main

import (
	"context"
	"fmt"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
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
		build: func(_ context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.optionEod(jc.brk)
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
			j, err := jc.f.optionEodJP(jc.brk)
			return j, "", err
		},
	})
}

func (f jobFactory) optionEod(b broker.Broker) (contracts.Job, error) {
	return buildOptionEod(f.d, b, f.inst, f.eod, f.watch, f.clk)
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

// optionEodJP builds option_eod_jp over the same stores and the same watch list as option_eod.
func (f jobFactory) optionEodJP(b broker.Broker) (contracts.Job, error) {
	return buildOptionEodJP(f.d, b, f.inst, f.eod, f.watch, f.clk)
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
