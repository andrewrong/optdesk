package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/candidate"
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

// openOldCandidateStore opens the old candidate store (candidates.option_candidate); a failed open is a nil
// interface, not a typed nil. strategy-compare reads its old side through it, so it does not follow the cutover.
func openOldCandidateStore(ctx context.Context, dsn string) (candidateStore, error) {
	s, err := candidate.Open(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// resultCandidates is the candidate store of the cutover: the result reader over the old one, and the close of both.
type resultCandidates struct {
	store.CandidateReader
	res resultStore
	old candidateStore
}

func (r resultCandidates) Close() error {
	return errors.Join(r.res.Close(), r.old.Close())
}

// openResultCandidateStore is the default of deps.openCandidate (docs/design/strategy-framework.md, 8 cutover):
// short_put of a day with a result run is read from strategy.result, everything else from the old rows. A failed
// open is a nil interface, not a typed nil.
func openResultCandidateStore(ctx context.Context, dsn string) (candidateStore, error) {
	return openResultCandidateWith(openStrategyResults, openOldCandidateStore)(ctx, dsn)
}

// openResultCandidateWith is openResultCandidateStore over the given openers, so a test hands it fake stores.
func openResultCandidateWith(openRes openStrategyResultsFunc, openOld openCandidateFunc) openCandidateFunc {
	return func(ctx context.Context, dsn string) (candidateStore, error) {
		res, err := openRes(ctx, dsn)
		if err != nil {
			return nil, err
		}
		old, err := openOld(ctx, dsn)
		if err != nil {
			_ = res.Close()
			return nil, err
		}
		return resultCandidates{CandidateReader: strategyresultpg.NewCandidateReader(res, oldReader(old)), res: res, old: old}, nil
	}
}
