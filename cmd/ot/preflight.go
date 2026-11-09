package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
)

// The job ids of the jobs ot wires by name, and the reasons a job is left out (Decision 8 of
// docs/design/scheduler.md). A reason holds no key and no path.
const (
	jobFredSeries      = "fred_series"
	jobOptionEod       = "option_eod"
	jobQuoteSnapshot   = "quote_snapshot"
	jobEarningsRefresh = "earnings_refresh"

	reasonNoFredKey  = "FRED_API_KEY is not set"
	reasonBrokerDown = "broker is not reachable"
	reasonBuildJob   = "the job could not be built"

	reasonNoEventsDSN = "OT_PG_DSN_EVENTS is not set"
	reasonNoAlphaKey  = "ALPHA_VANTAGE_API_KEY is not set"
)

// noKeySource stands in for the FRED source when FRED_API_KEY is empty: fred_series is left out, so it
// never runs, and the job exists only so that the ticker knows its schedule.
type noKeySource struct{}

func (noKeySource) Fetch(context.Context, sources.FetchPlan) ([]contracts.SeriesPoint, error) {
	return nil, errors.New("FRED_API_KEY is not set")
}

// startDisabled is the check of Decision 8 at process start: the left-out job ids with their reasons.
func startDisabled(cfg runOnceConfig, brokerUp bool) map[string]string {
	out := map[string]string{}
	if cfg.fredKey == "" {
		out[jobFredSeries] = reasonNoFredKey
	}
	if r := cfg.earningsOff(); r != "" {
		out[jobEarningsRefresh] = r
	}
	if !brokerUp {
		out[jobOptionEod] = reasonBrokerDown
		out[jobQuoteSnapshot] = reasonBrokerDown
	}
	return out
}

// prober re-runs the check of one left-out job. A passing check returns the job built anew and an empty
// reason; a failing one returns the reason. fred_series reads the key loaded at start, so with an empty
// key it fails again until the process restarts. The two broker jobs share the one fresh dial of a pass.
type prober struct {
	w      wiring
	hasKey bool

	mu  sync.Mutex
	brk broker.Broker // the broker of the last passing dial, nil before one
}

func (p *prober) probe(ctx context.Context, id string) (contracts.Job, string) {
	switch id {
	case jobFredSeries:
		if !p.hasKey {
			return nil, reasonNoFredKey
		}
		return p.built(p.w.factory.fred())
	case jobEarningsRefresh:
		// the lines are read once at start, so a missing one keeps the job out until the process restarts
		if p.w.earningsOff != "" {
			return nil, p.w.earningsOff
		}
		return p.built(p.w.factory.earningsRefresh())
	case jobOptionEod, jobQuoteSnapshot:
		b, err := p.broker(ctx)
		if err != nil {
			return nil, reasonBrokerDown
		}
		if id == jobOptionEod {
			return p.built(p.w.factory.optionEod(b))
		}
		return p.built(p.w.factory.quoteSnapshot(b))
	}
	return nil, "unknown job"
}

func (p *prober) built(j contracts.Job, err error) (contracts.Job, string) {
	if err != nil {
		return nil, reasonBuildJob
	}
	return j, ""
}

func (p *prober) broker(ctx context.Context) (broker.Broker, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.brk != nil {
		return p.brk, nil
	}
	b, err := p.w.dial(ctx)
	if err != nil {
		return nil, err
	}
	p.brk = b
	return b, nil
}

// swapReconciler is the Reconciler and Prober the ticker holds. Reconcile runs the current reconciler,
// which was built from the enabled jobs; a passing Probe builds the job anew, builds a new reconciler from
// the enabled jobs plus that job and swaps it under the mutex, so no restart is needed.
type swapReconciler struct {
	mu      sync.Mutex
	cur     runner.Reconciler
	enabled []contracts.Job
	build   func([]contracts.Job) runner.Reconciler
	probe   func(ctx context.Context, id string) (contracts.Job, string)
	log     *slog.Logger
}

var (
	_ runner.Reconciler = (*swapReconciler)(nil)
	_ runner.Prober     = (*swapReconciler)(nil)
)

func newSwapReconciler(
	enabled []contracts.Job, build func([]contracts.Job) runner.Reconciler,
	probe func(context.Context, string) (contracts.Job, string), log *slog.Logger,
) *swapReconciler {
	return &swapReconciler{cur: build(enabled), enabled: enabled, build: build, probe: probe, log: log}
}

func (s *swapReconciler) Reconcile(ctx context.Context, day contracts.Day) error {
	s.mu.Lock()
	cur := s.cur
	s.mu.Unlock()
	return cur.Reconcile(ctx, day)
}

// Probe returns "" when the job id is back and now part of the reconciler, else the reason it stays out.
func (s *swapReconciler) Probe(ctx context.Context, id string) string {
	job, reason := s.probe(ctx, id)
	if reason != "" {
		return reason
	}
	s.mu.Lock()
	s.enabled = append(append([]contracts.Job(nil), s.enabled...), job)
	s.cur = s.build(s.enabled)
	s.mu.Unlock()
	s.log.Info("job back in the schedule", "component", "ot", "job", id)
	return ""
}
