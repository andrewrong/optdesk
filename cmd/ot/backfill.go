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
)

func init() { register("backfill", backfill) }

func backfill(a *app, args []string) error {
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dayFlag := fs.String("day", "", "day YYYY-MM-DD to backfill (required)")
	job := fs.String("job", "", "job id to backfill (default: one request per runnable job)")
	force := fs.Bool("force", false, "ask for a forced run")
	var ef enqueueFlags
	ef.register(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *dayFlag == "" {
		return fmt.Errorf("--day is required")
	}
	clk := a.deps.newClock()
	day, err := dayOrDefault(clk, *dayFlag)
	if err != nil {
		return err
	}
	if a.profile.Name == "demo" {
		return fmt.Errorf("backfill needs a real profile; the demo profile opens no store")
	}
	var names []string
	if *job != "" {
		names = []string{*job}
	}
	return enqueue(context.Background(), a, "backfill", buildRequests(names, day, clk.Now(), *force, false), ef)
}

// finishBackfill prints the key lines of res, then returns the error of the run, of a refusal, of the key gap
// sync or of the keys still missing, or prints the status line and logs the counts.
func finishBackfill(a *app, day contracts.Day, job, dataset string, res runner.BackfillResult, runErr, syncErr error, texts map[keyID]string) error {
	counts := printKeyLines(a.stdout, day, res.Keys, res.InWindow, texts)
	if runErr != nil {
		var te contracts.TypedError
		if errors.As(runErr, &te) && te.Kind() == contracts.Retryable {
			return errors.New("the runner lease is held, retry in a minute")
		}
		return runFailedError(day, job, dataset, runErr)
	}
	if err := refusalError(res.Refused, day); err != nil {
		return err
	}
	if syncErr != nil {
		return fmt.Errorf("backfill %s: sync of the key gaps failed", day)
	}
	if n := len(res.Missing); n > 0 {
		return fmt.Errorf("backfill %s: %d key(s) are still missing", day, n)
	}
	_, _ = fmt.Fprintln(a.stdout, statusLine(day, counts))
	a.log.Info("backfill "+string(day)+" done", "component", "ot", "status", counts.status(),
		"written", counts.written, "present", counts.present, "no_value", counts.noValue, "failed", counts.failed, "skipped", counts.skipped)
	return nil
}

// runFailedError is the error of a failed backfill; it names the job and the dataset when set and carries
// the redacted text of the runner error.
func runFailedError(day contracts.Day, job, dataset string, err error) error {
	what := fmt.Sprintf("backfill %s", day)
	if job != "" {
		what += " job " + job
	}
	if dataset != "" {
		what += " dataset " + dataset
	}
	return fmt.Errorf("%s failed: %s", what, redactErrText(err.Error()))
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
		return nil, fmt.Errorf("backfill %s job %s: plan failed: %s", day, name, redactErrText(err.Error()))
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
