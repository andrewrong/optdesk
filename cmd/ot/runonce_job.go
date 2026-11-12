package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
)

// jobTableIDs returns the ids of the 24 jobs of jobTable, sorted.
func jobTableIDs() []string {
	ids := make([]string, 0, len(jobTable))
	for _, r := range jobTable {
		ids = append(ids, r.ID)
	}
	sort.Strings(ids)
	return ids
}

// unknownJobError is the answer for a --job name that has no row in jobTable.
func unknownJobError(name string) error {
	return fmt.Errorf("run-once: unknown job %s; known jobs: %s", name, strings.Join(jobTableIDs(), ", "))
}

// runOnceJob runs one job of the scheduler for real on day: the same wiring as the scheduler and ot backfill, the run
// lease of BackfillWith, the guards and the stores of the job. force runs a key that is already present again.
func runOnceJob(ctx context.Context, a *app, clk clock.Clock, cfg runOnceConfig, day contracts.Day, id string, force bool) error {
	fw := a.deps.newFwClock()
	sl, err := a.deps.openSchedulerLedger(ctx, cfg.runnerDSN)
	if err != nil {
		return fmt.Errorf("open scheduler ledger with %s failed", store.DSNEnvRunner)
	}
	defer func() { _ = sl.Close() }()
	return withWiring(ctx, a, clk, cfg, nil, func(w wiring) error {
		return runWiredJob(ctx, a, w, sl, fw, day, id, force)
	})
}

// runWiredJob is runOnceJob after the wiring is built: it checks that the job is runnable, backfills its keys, prints
// one line per key and the summary, and syncs the key gaps after a run.
func runWiredJob(ctx context.Context, a *app, w wiring, sl store.SchedulerLedger, fw clockwork.Clock,
	day contracts.Day, id string, force bool) error {
	js := w.runnable(a.log)
	if !hasJob(js, id) {
		reason := w.off[id]
		if reason == "" && isBarsJob(w, id) {
			reason = w.barsOff
		}
		if reason != "" {
			return fmt.Errorf("run-once: job %s is left out: %s", id, reason)
		}
		return fmt.Errorf("run-once: job %s is not wired", id)
	}
	_, b := a.deps.newReconciler(w.ledger, sl, w.cal, fw, js, runner.WindowRule{}, a.deps.newKeyRunner(w.ledger, sl, fw))
	fb, ok := b.(runner.ForceBackfiller)
	if !ok {
		return fmt.Errorf("run-once: job %s cannot run: the runner has no forced backfill", id)
	}
	datasets, err := jobDatasets(ctx, js, id, "", day)
	if err != nil {
		return err
	}
	res, runErr := fb.BackfillWith(ctx, day, datasets, runner.BackfillOptions{Force: force, Job: id})
	if runErr != nil {
		var te contracts.TypedError
		if errors.As(runErr, &te) && te.Kind() == contracts.Retryable {
			return errors.New("run-once refused: the runner lease is held by another run; retry in a minute")
		}
		// The runner error may carry the text of a connection error: it is withheld.
		return runFailedError(day, id, "")
	}
	status, ran := "ok", false
	for _, k := range res.Keys {
		if k.ErrorCode != "" {
			status = "failed"
		}
		ran = ran || k.Group == runner.GroupRan
		_, _ = fmt.Fprintf(a.stdout, "run-once job=%s day=%s dataset=%s subject=%s group=%s status=%s reason=%s error=%s\n",
			id, day, k.Key.DatasetID, k.Key.SubjectKey, k.Group, k.Status, k.ReasonCode, k.ErrorCode)
	}
	if res.Refused != "" {
		status = "refused"
	}
	_, _ = fmt.Fprintf(a.stdout, "run-once %s job=%s status=%s keys=%d\n", day, id, status, len(res.Keys))
	if err := refusalError(res.Refused, day); err != nil {
		return err
	}
	if ran {
		if err := a.deps.newKeyGapSyncer(w.ledger, sl, fw).SyncKeys(ctx, day); err != nil {
			return fmt.Errorf("run-once %s: sync of the key gaps failed", day)
		}
	}
	if status == "failed" {
		return fmt.Errorf("run-once %s job %s: a key ended in an error", day, id)
	}
	return nil
}

func hasJob(js []contracts.Job, id string) bool {
	for _, j := range js {
		if j.ID() == id {
			return true
		}
	}
	return false
}

func isBarsJob(w wiring, id string) bool { return hasJob(w.bars, id) }

// runOnceErr gives an error of the run-once of one job as it stands when it already says run-once, else with the day.
func runOnceErr(err error, day contracts.Day) error {
	if strings.HasPrefix(err.Error(), "run-once") {
		return err
	}
	return fmt.Errorf("run-once %s: %w", day, err)
}

// checkJobCount checks how many --job names a run-once takes and where --force is allowed.
func checkJobCount(n int, dryRun, force bool) error {
	if n > 1 && !dryRun {
		return fmt.Errorf("run-once takes exactly one --job")
	}
	if force && (n == 0 || dryRun) {
		return fmt.Errorf("--force needs --job and no --dry-run")
	}
	return nil
}
