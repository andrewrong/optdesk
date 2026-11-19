//go:build pg

package jobrunpg

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

var t0 = time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)

// closedRun writes one run of job that took d and ends with outcome; finish is when it ended.
func closedRun(t *testing.T, l jobrun.JobRunLedger, id, job, outcome string, finish time.Time, d time.Duration) {
	t.Helper()
	ctx := context.Background()
	if err := l.Start(ctx, jobrun.JobRun{ID: id, JobID: job, Source: "scheduler", StartedAt: finish.Add(-d)}); err != nil {
		t.Fatalf("start %s: %v", id, err)
	}
	if err := l.Close(ctx, id, outcome, finish); err != nil {
		t.Fatalf("close %s: %v", id, err)
	}
}

// TestCloseOpenClosesCut (new): a row left open is closed "cut" at start-up and then leaves the open view.
func TestCloseOpenClosesCut(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openAs(t, newDatabase(t), "ot_runner")
	for _, id := range []string{"a", "b"} {
		if err := st.Start(ctx, jobrun.JobRun{ID: id, JobID: "j-" + id, Source: "console", StartedAt: t0}); err != nil {
			t.Fatal(err)
		}
	}
	closedRun(t, st, "c", "j-c", "ok", t0.Add(time.Minute), time.Minute)
	if err := st.Progress(ctx, "a", 2, 5, "k2", t0.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got, err := st.Running(ctx)
	if err != nil || len(got) != 2 || got[0].ID != "a" || got[0].KeysDone != 2 || got[0].KeysTotal != 5 || got[0].Step != "k2" {
		t.Fatalf("Running = %+v, %v", got, err)
	}
	n, err := st.CloseOpen(ctx, t0.Add(time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("CloseOpen = %d, %v; want 2", n, err)
	}
	if got, err := st.Running(ctx); err != nil || len(got) != 0 {
		t.Fatalf("Running after CloseOpen = %+v, %v", got, err)
	}
	if n, err := st.CloseOpen(ctx, t0.Add(2*time.Hour)); err != nil || n != 0 {
		t.Fatalf("second CloseOpen = %d, %v; want 0", n, err)
	}
}

// TestStartTwiceForOneJobSucceeds (new): the row is no guard; a second open run of one job id is accepted and both show.
func TestStartTwiceForOneJobSucceeds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openAs(t, newDatabase(t), "ot_runner")
	for _, id := range []string{"r1", "r2"} {
		if err := st.Start(ctx, jobrun.JobRun{ID: id, JobID: "same", Source: "scheduler", StartedAt: t0}); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	got, err := st.Running(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("Running = %+v, %v; want 2", got, err)
	}
	if err := st.MarkSlowNotified(ctx, "r2", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ = st.Running(ctx)
	if !got[0].SlowNotifiedAt.IsZero() || !got[1].SlowNotifiedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("slow_notified_at = %v, %v", got[0].SlowNotifiedAt, got[1].SlowNotifiedAt)
	}
}

// TestUsualMedianIgnoresCutAndFailed (new): the median is of the last 20 ok runs; cut and failed runs are no sample.
func TestUsualMedianIgnoresCutAndFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := openAs(t, newDatabase(t), "ot_runner")
	// Five old ok runs of 100 minutes fall outside the last 20; 20 newer ok runs take 1..20 minutes.
	for i := 0; i < 5; i++ {
		closedRun(t, st, "old"+string(rune('a'+i)), "job", "ok", t0.Add(time.Duration(i)*time.Hour), 100*time.Minute)
	}
	for i := 1; i <= 20; i++ {
		closedRun(t, st, "ok"+string(rune('a'+i)), "job", "ok", t0.Add(24*time.Hour+time.Duration(i)*time.Hour), time.Duration(i)*time.Minute)
	}
	closedRun(t, st, "bad", "job", "failed", t0.Add(100*time.Hour), 500*time.Minute)
	closedRun(t, st, "cut1", "job", "cut", t0.Add(101*time.Hour), 500*time.Minute)
	closedRun(t, st, "solo", "other", "ok", t0.Add(time.Hour), 4*time.Minute)
	closedRun(t, st, "nothing", "never", "failed", t0.Add(time.Hour), time.Minute)

	got, err := st.Usuals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []jobrun.UsualStat{
		{JobID: "job", Median: 10*time.Minute + 30*time.Second, Samples: 20},
		{JobID: "other", Median: 4 * time.Minute, Samples: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("Usuals = %+v; want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Usuals[%d] = %+v; want %+v", i, got[i], want[i])
		}
	}
}

// TestRolesGrants (new): the runner role writes and reads; the pages role reads the two views and nothing else.
func TestRolesGrants(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	name := newDatabase(t)
	runner := openAs(t, name, "ot_runner")
	pages := openAs(t, name, "ot_ro_pages")
	if err := runner.Start(ctx, jobrun.JobRun{ID: "r", JobID: "j", StartedAt: t0}); err != nil {
		t.Fatal(err)
	}
	closedRun(t, runner, "ok1", "j", "ok", t0.Add(time.Hour), 3*time.Minute)
	if got, err := pages.Running(ctx); err != nil || len(got) != 1 || got[0].ID != "r" {
		t.Fatalf("pages Running = %+v, %v", got, err)
	}
	if got, err := pages.Usuals(ctx); err != nil || len(got) != 1 || got[0].Median != 3*time.Minute {
		t.Fatalf("pages Usuals = %+v, %v", got, err)
	}
	if err := pages.Start(ctx, jobrun.JobRun{ID: "x", JobID: "j", StartedAt: t0}); err == nil {
		t.Fatal("pages role inserted a row")
	}
	if err := pages.Close(ctx, "r", "cut", t0); err == nil {
		t.Fatal("pages role updated a row")
	}
	if _, err := pages.pool.Exec(ctx, `SELECT id FROM ops.job_run`); err == nil {
		t.Fatal("pages role read the table")
	}
	if _, err := runner.pool.Exec(ctx, `DELETE FROM ops.job_run`); err == nil {
		t.Fatal("runner role deleted rows")
	}
}
