package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/andrewrong/optdesk/internal/compute/buyprice"
	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/jobs"
	jobsbuyprice "github.com/andrewrong/optdesk/internal/jobs/buyprice"
	"github.com/andrewrong/optdesk/internal/store"
)

// Design: docs/design/buy-price-source.md#7; contract: docs/contracts/ot/buyprice.md.

// openJobValuation opens the valuation reader with the pages line (ot_runner has no grant on the valuation views) and
// gives f the reader and the resolver. A deps without the opener, or an empty line, leaves f.valuation nil: the option
// families then price with hand prices only. What it opens goes to extra. No error holds a DSN.
func openJobValuation(a *app, cfg runOnceConfig, f *jobFactory, extra *closerSet) error {
	// The calculator goes through its interface so that the two compute packages stay unaware of each other here.
	var calc valuelinesapi.Calculator = valuelines.New()
	f.buy = buyprice.New(calc)
	if a.deps.openValuation == nil || cfg.pagesDSN == "" {
		return nil
	}
	rd, _, c, err := a.deps.openValuation(cfg.pagesDSN)
	if err != nil {
		return fmt.Errorf("open valuation reader with %s failed", store.DSNEnvPages)
	}
	if c != nil {
		extra.addCloser(c)
	}
	f.valuation = rd
	return nil
}

// pricesFor is the prices step of a group: nil for every group but groupOptions. Its first call (the build at start)
// reads no valuation and logs one line; every later call reads the valuation once.
func (f jobFactory) pricesFor(group string) listedPrices {
	if group != groupOptions {
		return nil
	}
	var started atomic.Bool
	return func(ctx context.Context, entries []jobs.WatchEntry) ([]jobs.WatchEntry, error) {
		rd := f.valuation
		if started.CompareAndSwap(false, true) {
			msg := "buyprice: start builds with hand prices, each Plan reads the valuation"
			if rd == nil {
				msg = "buyprice: no valuation reader, hand prices only"
			}
			if f.log != nil {
				f.log.Info(msg, "component", "ot")
			}
			rd = nil
		}
		return jobsbuyprice.Apply(ctx, rd, f.buy, entries)
	}
}
