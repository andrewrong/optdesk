package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

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
	job := fs.String("job", "", "job id to backfill (default: every job); see the known names in the error of an unknown one")
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
	// No advisory fence: the run lease under its own owner keeps a Reconcile and a Backfill apart (Decision 3).
	sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	var res runner.BackfillResult
	var runErr, syncErr error
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
	if runErr != nil {
		var te contracts.TypedError
		if errors.As(runErr, &te) && te.Kind() == contracts.Retryable {
			return errors.New("the runner lease is held, retry in a minute")
		}
		// The runner error may carry the text of a connection error: it is withheld.
		return runFailedError(day, *job, *dataset)
	}
	if err := refusalError(res.Refused, day); err != nil {
		return err
	}
	if syncErr != nil {
		return fmt.Errorf("backfill %s: sync of the key gaps failed", day)
	}
	if err := missingError(a.stdout, res.Missing, day); err != nil {
		return err
	}
	a.log.Info("backfill "+string(day)+" done", "component", "ot", "status", "ok",
		"already_present", len(res.AlreadyPresent), "filled", len(res.Filled), "absent", len(res.Absent), "missing", len(res.Missing),
		"in_window", len(res.InWindow))
	return nil
}

// runFailedError is the withheld error of a failed backfill; it names the job and the dataset when set.
func runFailedError(day contracts.Day, job, dataset string) error {
	what := fmt.Sprintf("backfill %s", day)
	if job != "" {
		what += " job " + job
	}
	if dataset != "" {
		what += " dataset " + dataset
	}
	return fmt.Errorf("%s failed: the error text is withheld because it may hold a DSN or a key", what)
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

// jobDatasets resolves the job named name among jobs to the distinct dataset ids of its plan for day, in plan order.
// A non-empty dataset narrows the result to that one dataset, which must be one of them.
func jobDatasets(ctx context.Context, jobs []contracts.Job, name, dataset string, day contracts.Day) ([]string, error) {
	var found contracts.Job
	known := map[string]bool{}
	for _, j := range jobs {
		known[j.ID()] = true
		if j.ID() == name && found == nil {
			found = j
		}
	}
	if found == nil {
		ids := make([]string, 0, len(known))
		for id := range known {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return nil, fmt.Errorf("unknown job %q; known jobs: %s", name, strings.Join(ids, ", "))
	}
	keys, _, err := found.Plan(ctx, day)
	if err != nil {
		return nil, fmt.Errorf("backfill %s job %s: plan failed: the error text is withheld because it may hold a DSN or a key", day, name)
	}
	var out []string
	seen := map[string]bool{}
	for _, k := range keys {
		if !seen[k.DatasetID] {
			seen[k.DatasetID] = true
			out = append(out, k.DatasetID)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("backfill %s: job %s plans no key on the day", day, name)
	}
	if dataset != "" {
		if !seen[dataset] {
			return nil, fmt.Errorf("backfill %s: dataset %s is not a dataset of job %s", day, dataset, name)
		}
		return []string{dataset}, nil
	}
	return out, nil
}

// missingError prints one line per missing key to out and returns the error that counts them; nil when none is missing.
func missingError(out io.Writer, missing []contracts.ExpectedKey, day contracts.Day) error {
	if len(missing) == 0 {
		return nil
	}
	for _, k := range missing {
		_, _ = fmt.Fprintf(out, "missing day=%s dataset=%s subject=%s\n", day, k.DatasetID, k.SubjectKey)
	}
	return fmt.Errorf("backfill %s: %d key(s) are still missing", day, len(missing))
}
