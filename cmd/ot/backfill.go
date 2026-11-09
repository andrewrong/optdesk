package main

import (
	"context"
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
	fence, err := holdOffScheduler(ctx, a, cfg, "backfill", "backfill refused: ot scheduler holds the lock or the lock cannot be taken; check ot scheduler status")
	if err != nil {
		return err
	}
	defer func() { _ = fence.Close() }()
	var res runner.BackfillResult
	var runErr, cursorErr error
	var cursor contracts.Day
	err = withWiring(ctx, a, clk, cfg, nil, func(w wiring) error {
		b := a.deps.newBackfiller(w.ledger, w.cal, clk, w.runnable(a.log), runner.WindowRule{})
		res, runErr = b.Backfill(ctx, day, datasets)
		if runErr == nil && res.Refused == runner.RefusedAheadOfCursor {
			var cur store.Cursor
			cur, cursorErr = w.ledger.Cursor(ctx)
			cursor = cur.Day
		}
		return nil
	})
	if err != nil {
		return err
	}
	if runErr != nil {
		// The runner error may carry the text of a connection error: it is withheld.
		what := fmt.Sprintf("backfill %s", day)
		if *dataset != "" {
			what += " dataset " + *dataset
		}
		return fmt.Errorf("%s failed: the error text is withheld because it may hold a DSN or a key", what)
	}
	if cursorErr != nil {
		return fmt.Errorf("read reconcile cursor failed")
	}
	if err := refusalError(res.Refused, day, cursor); err != nil {
		return err
	}
	if len(res.Missing) > 0 {
		return fmt.Errorf("backfill %s: %d key(s) are still missing", day, len(res.Missing))
	}
	a.log.Info("backfill "+string(day)+" done", "component", "ot", "status", "ok",
		"already_present", len(res.AlreadyPresent), "filled", len(res.Filled), "absent", len(res.Absent), "missing", len(res.Missing))
	return nil
}

// refusalError is the error for a day the backfiller refused; nil when it did not.
func refusalError(refused runner.BackfillRefusal, day, cursor contracts.Day) error {
	switch refused {
	case "":
		return nil
	case runner.RefusedAheadOfCursor:
		shown := string(cursor)
		if shown == "" {
			shown = "(none)"
		}
		return fmt.Errorf("day %s is ahead of the cursor %s: use run-once", day, shown)
	case runner.RefusedNoPlan:
		return fmt.Errorf("no plan was recorded for day %s: nothing to backfill", day)
	}
	return fmt.Errorf("backfill %s was refused", day)
}
