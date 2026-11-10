package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

func init() { register("backfill", backfill) }

func backfill(a *app, args []string) error {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD to backfill (required)")
	dataset := fs.String("dataset", "", "dataset id (default: every key of the day)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dayFlag == "" {
		return fmt.Errorf("--day is required")
	}
	day, err := dayOrDefault(a.deps.newClock(), *dayFlag)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("backfill needs a real profile; the demo profile opens no store")
	}
	cfg, err := loadRunOnceConfig(a)
	if err != nil {
		return err
	}
	var datasets []string
	if *dataset != "" {
		datasets = []string{*dataset}
	}
	ctx := context.Background()
	clk := a.deps.newClock()
	fw := a.deps.newFwClock()
	// No advisory fence: the run lease under its own owner keeps a Reconcile and a Backfill apart (Decision 3).
	sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	var res runner.BackfillResult
	var runErr, syncErr error
	err = withWiring(ctx, a, clk, cfg, nil, func(w wiring) error {
		kr := a.deps.newKeyRunner(w.ledger, sl, fw)
		_, b := a.deps.newReconciler(w.ledger, sl, w.cal, fw, w.runnable(a.log), runner.WindowRule{}, kr)
		res, runErr = b.Backfill(ctx, day, datasets)
		if runErr == nil && res.Refused == "" {
			syncErr = a.deps.newKeyGapSyncer(w.ledger, sl, fw).SyncKeys(ctx, day)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if runErr != nil {
		var te contracts.TypedError
		if errors.As(runErr, &te) && te.Kind() == contracts.Retryable {
			return errors.New("the runner lease is held, retry in a minute")
		}
		// The runner error may carry the text of a connection error: it is withheld.
		what := fmt.Sprintf("backfill %s", day)
		if *dataset != "" {
			what += " dataset " + *dataset
		}
		return fmt.Errorf("%s failed: the error text is withheld because it may hold a DSN or a key", what)
	}
	if err := refusalError(res.Refused, day); err != nil {
		return err
	}
	if syncErr != nil {
		return fmt.Errorf("backfill %s: sync of the key gaps failed", day)
	}
	if len(res.Missing) > 0 {
		return fmt.Errorf("backfill %s: %d key(s) are still missing", day, len(res.Missing))
	}
	a.log.Info("backfill "+string(day)+" done", "component", "ot", "status", "ok",
		"already_present", len(res.AlreadyPresent), "filled", len(res.Filled), "absent", len(res.Absent), "missing", len(res.Missing),
		"in_window", len(res.InWindow))
	return nil
}

// refusalError is the error for a day the backfiller refused; nil when it did not.
func refusalError(refused runner.BackfillRefusal, day contracts.Day) error {
	if refused == "" {
		return nil
	}
	if refused == runner.RefusedNoPlan {
		return fmt.Errorf("no plan was recorded for day %s: the day is not planned yet or was never reconciled", day)
	}
	return fmt.Errorf("backfill %s was refused", day)
}
