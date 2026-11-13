package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/strategyresultpg"
)

func init() { register("strategy-compare", strategyCompareCmd) }

// strategyCompareCmd is `ot strategy-compare [--day DAY]`: it reads the short_put rows of DAY from the old table
// (deps.openCandidate) and from strategy.result (deps.openStrategyResults), both with OT_PG_DSN_PAGES (role
// ot_ro_pages), and prints `equal rows=N` or one line per difference; a difference is an error (exit 1). It
// writes nothing. DAY defaults to today in the market zone, as for `ot candidates`.
func strategyCompareCmd(a *app, args []string) error {
	fs := flag.NewFlagSet("strategy-compare", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD (default: today in "+marketZone+")")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return errors.New("strategy-compare needs a real profile; the demo profile opens no store")
	}
	env, err := a.envFile()
	if err != nil {
		return err
	}
	dsn := env[pagesDSNName]
	if dsn == "" {
		return errors.New(pagesDSNName + " is not set")
	}
	if a.deps.openCandidate == nil || a.deps.openStrategyResults == nil {
		return errors.New("strategy-compare is not wired")
	}
	ctx := context.Background()
	day, err := dayOrDefault(a.deps.newClock(), *dayFlag)
	if err != nil {
		return err
	}
	old, err := a.deps.openCandidate(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open candidate store with %s failed", pagesDSNName)
	}
	defer func() { _ = old.Close() }()
	res, err := a.deps.openStrategyResults(ctx, dsn)
	if err != nil {
		return fmt.Errorf("open strategy results with %s failed", pagesDSNName)
	}
	defer func() { _ = res.Close() }()
	lines, err := strategyresultpg.Compare(oldReader(old), res, day)
	if err != nil {
		return errors.New("compare strategy results failed")
	}
	if len(lines) > 0 {
		for _, l := range lines {
			_, _ = fmt.Fprintln(a.stdout, l)
		}
		return fmt.Errorf("strategy-compare %s: %d difference(s)", day, len(lines))
	}
	rows, err := res.Results(ctx, day, "short_put")
	if err != nil {
		return errors.New("read strategy results failed")
	}
	_, _ = fmt.Fprintf(a.stdout, "equal rows=%d\n", len(rows))
	return nil
}

// oldReader hands the candidate store to Compare as the store interface it is asked for, so the package-private
// type of cmd/ot does not cross into the store package.
func oldReader(c candidateStore) store.CandidateReader { return c }
