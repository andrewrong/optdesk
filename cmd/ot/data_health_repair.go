package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
	"github.com/andrewrong/optdesk/internal/store/coverage"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// Design: docs/design/data-health.md#8 (Decisions 4, 5 and 8).

// textUnknownDataset is the fixed refusal of a repair of a dataset no job writes.
const textUnknownDataset = "the dataset is not known"

// repairNonePath is the answer of a repair that queued nothing.
const repairNonePath = "/" + siteDataHealthRoute + "?repair=none"

var errRepairUnavailable = errors.New("repair: coverage reader or store not available")

// postRepair serves action=repair: the dataset is the only field read. The coverage reader is opened with the
// pages line per post and closed after it.
func postRepair(ctx context.Context, a *app, st jobrequest.Poster, clk clock.Clock, w http.ResponseWriter, r *http.Request) {
	dataset := r.PostForm.Get("dataset")
	d, ok := a.deps.ext[extDataHealth].(dataHealthDeps)
	env, err := a.envFile()
	if !ok || d.openCoverage == nil || err != nil || env[pagesDSNName] == "" {
		storeFailed(a, w, "data health repair: no coverage reader")
		return
	}
	cov, err := d.openCoverage(ctx, env[pagesDSNName])
	if err != nil {
		storeFailed(a, w, "data health repair: cannot open the coverage")
		return
	}
	defer func() { _ = cov.Close() }()
	first, found, err := repairGaps(ctx, cov, st, dataset, clk)
	switch {
	case !found:
		http.Error(w, textUnknownDataset, http.StatusUnprocessableEntity)
	case err != nil:
		storeFailed(a, w, "data health repair: failed")
	case first == 0:
		http.Redirect(w, r, repairNonePath, http.StatusSeeOther)
	default:
		redirectToRequest(w, r, first)
	}
}

// repairJobOf is the first job of the page's table that lists the dataset; nil when none does.
func repairJobOf(dataset string) *data_health_page.Job {
	jobs := dataHealthJobs()
	for i := range jobs {
		for _, d := range jobs[i].Datasets {
			if d == dataset {
				return &jobs[i]
			}
		}
	}
	return nil
}

// repairGaps queues one plain request (Force false) for each day data_health_page.GapDays gives for the dataset,
// oldest first, and returns the id of the first request (0 when nothing was queued). found is false for a dataset
// no job writes. An open request of the same job and day is kept, not an error.
func repairGaps(ctx context.Context, cov coverage.Reader, st jobrequest.Poster, dataset string, clk clock.Clock) (first int64, found bool, err error) {
	job := repairJobOf(dataset)
	if job == nil {
		return 0, false, nil
	}
	now := clk.Now().UTC()
	today, err := dayOrDefault(clk, "")
	if err != nil {
		return 0, true, err
	}
	cells, err := repairCells(ctx, cov, job, dataset, now, today)
	if err != nil {
		return 0, true, err
	}
	for _, day := range data_health_page.GapDays(cells) {
		res, err := st.Enqueue(ctx, jobrequest.Request{JobID: job.ID, Day: string(day), Force: false})
		if err != nil {
			return first, true, err
		}
		switch res.Outcome {
		case jobrequest.EnqueueCreated, jobrequest.EnqueueExisting:
			if first == 0 {
				first = res.Row.ID
			}
		case jobrequest.EnqueueWithdrawing:
			// The open request of this day is going away; the owner presses again when it shows withdrawn.
		case jobrequest.EnqueueBusy:
			return first, true, errRepairUnavailable
		default:
			return first, true, errRepairUnavailable
		}
	}
	return first, true, nil
}

// repairCells reads the sessions and cells as the page does and applies the verdicts with a present form.
func repairCells(ctx context.Context, cov coverage.Reader, job *data_health_page.Job, dataset string, now time.Time,
	today contracts.Day) ([]data_health_page.Cell, error) {
	cal := job.Calendar
	if cal == "" {
		cal = calUS
	}
	t, err := time.Parse(time.DateOnly, string(today))
	if err != nil {
		return nil, err
	}
	lookback := contracts.Day(t.AddDate(0, 0, -coverage.LookbackDays).Format(time.DateOnly))
	sessions, err := cov.Sessions(ctx, cal, lookback, today)
	if err != nil {
		return nil, err
	}
	days := data_health_page.WindowDays(sessions, today)
	if len(days) == 0 {
		return nil, nil
	}
	rows, err := cov.Cells(ctx, days[0], today)
	if err != nil {
		return nil, err
	}
	cells := data_health_page.DatasetCells(rows, dataset, days)
	data_health_page.Apply(cells, job, &data_health_page.Form{}, now, today)
	return cells, nil
}
