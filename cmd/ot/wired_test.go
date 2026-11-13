package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// runWired is the wired run of one day for the tests: it refuses the demo profile, loads the config, opens the
// stores, wires the jobs and reconciles the day through the deps (--day only).
func runWired(a *app, args []string) error {
	fs := flag.NewFlagSet("run-once", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("run-once needs a real profile; the demo profile opens no store")
	}
	cfg, err := loadRunOnceConfig(a)
	if err != nil {
		return err
	}
	ctx, clk := context.Background(), a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	check := func(ledger store.OpsLedger) error {
		cur, err := ledger.Cursor(ctx)
		if err != nil {
			return errors.New("read reconcile cursor failed")
		}
		if cur.Day != "" && day <= cur.Day {
			return fmt.Errorf("day %s is at or before the cursor %s: nothing to reconcile; use ot backfill --day %s", day, cur.Day, day)
		}
		return nil
	}
	fw := a.deps.newFwClock()
	sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("run-once %s: open scheduler ledger with %s failed", day, store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	err = withWiring(ctx, a, clk, cfg, check, func(w wiring) error {
		js := w.runnable(a.log)
		rec, _ := a.deps.newReconciler(w.ledger, sl, w.cal, fw, js, runner.WindowRule{}, a.deps.newKeyRunner(w.ledger, sl, fw))
		if err := rec.Reconcile(ctx, day); err != nil {
			return err
		}
		return printOutcomes(ctx, a.stdout, w.ledger, js, day, w.off[jobEarningsRefresh])
	})
	if err != nil {
		return fmt.Errorf("run-once %s: %w", day, err)
	}
	return nil
}

// backfillWired backfills one day through the deps (--day, --job, --dataset).
func backfillWired(a *app, args []string) error {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD to backfill (required)")
	dataset := fs.String("dataset", "", "dataset id (default: every key of the day)")
	job := fs.String("job", "", "job id to backfill (default: every job)")
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
	var filterErr error
	ctx := context.Background()
	clk := a.deps.newClock()
	fw := a.deps.newFwClock()
	sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	var res runner.BackfillResult
	var runErr, syncErr error
	report, restore := installKeyReport()
	defer restore()
	err = withWiring(ctx, a, clk, cfg, nil, func(w wiring) error {
		if *job != "" {
			if datasets, filterErr = jobDatasets(ctx, w.runnable(a.log), *job, *dataset, day); filterErr != nil {
				return nil
			}
		}
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
	if filterErr != nil {
		return filterErr
	}
	return finishBackfill(a, day, *job, *dataset, res, runErr, syncErr, report.recorded())
}
