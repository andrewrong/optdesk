package reconcile

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

// Sources and outcomes of a job-run row (docs/design/scheduler-per-job.md, Decision 6).
const (
	sourceScheduler = "scheduler"
	sourceConsole   = "console"

	outcomeOK     = "ok"
	outcomeFailed = "failed"
	outcomeCut    = "cut"
)

// runLedgers holds the JobRunLedger of each reconciler given one by WithJobRunLedger. It is a side table
// because the struct of the reconciler lives in reconcile.go, which this change does not edit. A reconciler
// with no entry records nothing: run-once and the old Reconcile path are such.
var runLedgers sync.Map // *reconciler -> jobrun.JobRunLedger

// WithJobRunLedger sets the ledger in which the spawned runs and the console runs of a reconciler or
// backfiller built by this package are recorded. Any other value is left unchanged. The row records and is
// not a guard: a failed write is logged and never stops a run.
func WithJobRunLedger(r any, l jobrun.JobRunLedger) {
	if rec, ok := r.(*reconciler); ok && l != nil {
		runLedgers.Store(rec, l)
	}
}

// runRow is the open row of one run.
type runRow struct {
	r     *reconciler
	led   jobrun.JobRunLedger
	id    string
	jobID string
	mu    sync.Mutex
	done  int
	total int
}

type runRowKey struct{}

// recordRun runs run as one job run recorded in the ledger of r: it opens the row (Start), passes run the
// reconciler whose key runs report to the row and the context, and closes the row in a deferred call with
// the outcome run returns ("ok", "failed" or "cut"), so every return, a panic included, closes it. With
// keys > 0 the row counts the keys the job's key runner finishes, of keys; with keys == 0 run counts its
// own through noteTotal and noteKey. Without a ledger, or without a job id (a console call over every job), run is called as it is.
func (r *reconciler) recordRun(
	ctx context.Context, jobID, source string, keys int, run func(rr *reconciler, ctx context.Context) string,
) {
	v, ok := runLedgers.Load(r)
	if !ok || jobID == "" {
		run(r, ctx)
		return
	}
	row := &runRow{r: r, led: v.(jobrun.JobRunLedger), id: uuid.NewString(), jobID: jobID, total: keys}
	now := r.clk.Now()
	if err := row.led.Start(ctx, jobrun.JobRun{ID: row.id, JobID: jobID, Source: source, StartedAt: now, HeartbeatAt: now, KeysTotal: keys}); err != nil {
		row.warn("start", err)
		run(r, ctx)
		return
	}
	outcome := outcomeFailed
	defer func() {
		// The run's context may be the cancelled one that cut the run; the close is not.
		if err := row.led.Close(context.WithoutCancel(ctx), row.id, outcome, r.clk.Now()); err != nil {
			row.warn("close", err)
		}
	}()
	rr := r
	ctx = context.WithValue(ctx, runRowKey{}, row)
	if keys > 0 {
		cp := *r
		cp.kr = rowKeyRunner{KeyRunner: r.kr, row: row}
		rr = &cp
	}
	outcome = run(rr, ctx)
}

// outcomeOf names how a run ended: cut when its context ended or a job stopped it, failed on an error or a
// failed key, otherwise ok.
func outcomeOf(ctx context.Context, cut, failed bool) string {
	switch {
	case cut || ctx.Err() != nil:
		return outcomeCut
	case failed:
		return outcomeFailed
	}
	return outcomeOK
}

// noteTotal sets the number of keys of the run that ctx belongs to; it does nothing for an unrecorded run.
func noteTotal(ctx context.Context, n int) {
	if row, ok := ctx.Value(runRowKey{}).(*runRow); ok {
		row.mu.Lock()
		row.total = n
		row.mu.Unlock()
	}
}

// noteKey records one key of the run that ctx belongs to as done; it does nothing for an unrecorded run.
func noteKey(ctx context.Context, k contracts.ExpectedKey) {
	if row, ok := ctx.Value(runRowKey{}).(*runRow); ok {
		row.progress(ctx, k)
	}
}

func (w *runRow) progress(ctx context.Context, k contracts.ExpectedKey) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.done++
	step := k.DatasetID + "/" + k.SubjectKey
	if err := w.led.Progress(context.WithoutCancel(ctx), w.id, w.done, w.total, step, w.r.clk.Now()); err != nil {
		w.warn("progress", err)
	}
}

func (w *runRow) warn(op string, err error) {
	w.r.log.Warn("job run ledger write failed", slog.String("op", op), slog.String("job", w.jobID), slog.String("error", err.Error()))
}

// rowKeyRunner reports each key its job finishes to the row.
type rowKeyRunner struct {
	runner.KeyRunner
	row *runRow
}

func (k rowKeyRunner) RunKey(
	ctx context.Context, job contracts.Job, day contracts.Day, key contracts.ExpectedKey, attempt int, closeAt, deadline time.Time, reason string,
) (store.Run, error) {
	rec, err := k.KeyRunner.RunKey(ctx, job, day, key, attempt, closeAt, deadline, reason)
	// Dependents the run triggers go through the same runner; only the row's own job counts.
	if job.ID() == k.row.jobID {
		k.row.progress(ctx, key)
	}
	return rec, err
}
