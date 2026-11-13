package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
	"github.com/andrewrong/optdesk/internal/store/strategyresultpg"
	"github.com/andrewrong/optdesk/internal/strategy"
	"github.com/andrewrong/optdesk/internal/strategy/shortput"
)

// The management defaults of the short put (docs/design/strategy-framework.md, Decision 9): hold to expiry; the
// early-close numbers are what the replay of step 2 compares.
const (
	shortPutCloseMode = shortput.CloseExpiry
	shortPutEarlyKeep = "0.50"
	shortPutEarlyDTE  = "21"
)

// resultStore is the results store as the job writes it and the command reads it: the store and its close.
type resultStore = strategyresult.Store

// openStrategyResultsFunc opens strategy.result with one DSN line (internal/store/strategyresultpg.Open).
type openStrategyResultsFunc func(ctx context.Context, dsn string) (resultStore, error)

// openStrategyResults opens the results store; a failed open is a nil interface, not a typed nil.
func openStrategyResults(ctx context.Context, dsn string) (resultStore, error) {
	s, err := strategyresultpg.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// shadowStores is what option_candidates gets for the shadow of the strategies; both nil is the job of today.
type shadowStores struct {
	strategy strategy.Strategy
	results  resultStore
}

// buildStrategies is the one place the list of strategies is built: today the short put, over the put rule, the
// [params] of the manifest of option_candidates and the management defaults. An error of optioncandidates.Params
// is returned as it is; the limits are never copied here.
func buildStrategies(rule compute.OptionCandidates) ([]strategy.Strategy, error) {
	p, err := optioncandidates.Params()
	if err != nil {
		return nil, err
	}
	sp, err := shortput.New(rule, p, shortput.Values{CloseMode: shortPutCloseMode, EarlyKeep: shortPutEarlyKeep, EarlyDTE: shortPutEarlyDTE})
	if err != nil {
		return nil, err
	}
	return []strategy.Strategy{sp}, nil
}

// shadowOf opens the results store with the candidate line and builds the short put for the shadow. The open
// error names the key and never a value. A deps without the opener gives no shadow.
func (f jobFactory) shadowOf(ctx context.Context, dsn string, extra *closerSet) (shadowStores, error) {
	if f.d.openStrategyResults == nil || f.d.newCandidateRule == nil {
		return shadowStores{}, nil
	}
	res, err := f.d.openStrategyResults(ctx, dsn)
	if err != nil {
		return shadowStores{}, fmt.Errorf("open strategy results with %s failed", store.DSNEnvCandidate)
	}
	extra.addCloser(res)
	list, err := buildStrategies(f.d.newCandidateRule())
	if err != nil {
		return shadowStores{}, fmt.Errorf("build strategies failed: %w", err)
	}
	if len(list) != 1 {
		return shadowStores{}, errors.New("build strategies failed: the job takes one strategy")
	}
	return shadowStores{strategy: list[0], results: res}, nil
}
