package main

import (
	"context"
	"errors"

	"github.com/andrewrong/optdesk/internal/strategy/replay"
	"github.com/andrewrong/optdesk/internal/strategy/replayapi"
)

// Design: docs/design/strategy-replay-page.md#3.

// replayResult is the report of one run with the header the output names besides it.
type replayResult struct {
	Report replayapi.Report
	Header replayHeader
}

// replayRun is the one function behind `ot strategy-replay` and the strategy replay page: the checks of the options
// that are not about flags, the variants, the as-of day, the four store lines, the lots and the engine. It reads
// with ctx and writes nothing. The demo profile and a missing line are an error.
func replayRun(ctx context.Context, a *app, o replayOpts) (replayResult, error) {
	if o.lots != lotsShortPuts && o.lots != lotsShortLots {
		return replayResult{}, errors.New("lots is not " + lotsShortPuts + " or " + lotsShortLots)
	}
	if o.minLots < 0 {
		return replayResult{}, errors.New("--min-lots is negative")
	}
	if a.profile.Name == "demo" {
		return replayResult{}, errors.New("strategy-replay needs a real profile; the demo profile opens no store")
	}
	hold, early, err := replayVariants(o)
	if err != nil {
		return replayResult{}, err
	}
	asOf, err := replayAsOf(a, o.asOf)
	if err != nil {
		return replayResult{}, err
	}
	env, err := a.replayEnv()
	if err != nil {
		return replayResult{}, err
	}
	lots, mkt, sessions, err := a.replayLots(ctx, env, o.lots, o.alias, asOf)
	if err != nil {
		return replayResult{}, err
	}
	rep := replay.New().Replay(lots, mkt, early, sessions, asOf, o.minLots)
	names := []replayName{{Name: hold.Name, Hash: hold.Strategy.Params().Hash}}
	for _, v := range early {
		names = append(names, replayName{Name: v.Name, Hash: v.Strategy.Params().Hash})
	}
	return replayResult{Report: rep, Header: replayHeader{Lots: o.lots, AsOf: asOf, Variants: names, MinLots: o.minLots}}, nil
}
