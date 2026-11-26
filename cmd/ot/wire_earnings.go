package main

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/sources/yfinance"
)

// Design: docs/design/ot-wire-split.md#Decisions 2 and 7.

// earnings_refresh is held when a line is missing: wired for the scheduler, left out of run-once and backfill.
func init() {
	registerJob(jobFamily{
		id: jobEarningsRefresh, order: 90,
		startOff: func(cfg runOnceConfig, _ bool) string { return cfg.earningsOff() },
		build: func(_ context.Context, _ *app, jc *jobCtx) (contracts.Job, string, error) {
			j, err := jc.f.earningsRefresh()
			return j, jc.cfg.earningsOff(), err
		},
	})
}

// earningsOff is the reason earnings_refresh is left out, or "" when it runs. It names a line, never a value.
func (c runOnceConfig) earningsOff() string {
	switch {
	case c.eventsDSN == "":
		return reasonNoEventsDSN
	case c.alphaKey == "":
		return reasonNoAlphaKey
	}
	return ""
}

// earningsRefresh builds earnings_refresh over the events store, the Alpha Vantage source and the fundamentals names
// of the stored list at each Plan; the J-Quants source is built anew over the JP symbols of those names, so a paused
// JP name is not asked and a new one is. A name the primary source gives no date is asked of yfinance, which runs the
// refearn command through yfinance.Exec, as the bars source does (earnings-fallback.md Decision 8).
func (f jobFactory) earningsRefresh() (contracts.Job, error) {
	return f.earningsRefreshVia(yfinance.Exec)
}

// earningsRefreshVia is earningsRefresh with run as the command runner of the yfinance fallback.
func (f jobFactory) earningsRefreshVia(run yfinance.CommandFunc) (contracts.Job, error) {
	mk := func(market string, symbols []string) (sources.EarningsSource, error) {
		return yfinance.NewEarnings(market, run, sources.EarningsConfig{Symbols: symbols})
	}
	return f.listed(groupFundamentals, func(entries []jobs.WatchEntry) (contracts.Job, error) {
		earn := maps.Clone(f.earn)
		if f.jp != nil {
			jq, err := f.jp(jpSymbols(entries))
			if err != nil {
				return nil, err
			}
			earn["JP"] = jq
		}
		j, err := f.d.newEarningsRefresh(earn, f.events, f.events, entries, f.clk, f.d.symbols)
		if err != nil {
			return nil, fmt.Errorf("build earnings_refresh job failed: %w", err)
		}
		return earningsrefresh.WithFallback(j, mk), nil
	})
}

// noEventsStore stands in for the events store when OT_PG_DSN_EVENTS is empty: the job is left out, so it is
// never called.
type noEventsStore struct{}

func (noEventsStore) InsertEarningsDate(context.Context, contracts.EarningsDate) error {
	return errors.New(reasonNoEventsDSN)
}

func (noEventsStore) EarningsDates(context.Context, string, contracts.Day, contracts.Day) ([]contracts.EarningsDate, error) {
	return nil, errors.New(reasonNoEventsDSN)
}

func (noEventsStore) Close() error { return nil }
