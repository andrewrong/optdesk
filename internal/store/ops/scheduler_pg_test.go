//go:build pg

package ops

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// newSchedulerDatabase is newDatabase plus migration 0012.
func newSchedulerDatabase(t *testing.T) string {
	t.Helper()
	name := newDatabase(t)
	applyMigration(t, connURL(t, name, ""), "0012_scheduler.sql")
	return name
}

func TestFenceSecondLockRefusedThenFreedOnClose(t *testing.T) {
	t.Parallel()
	name := newSchedulerDatabase(t)
	ctx := context.Background()
	fence, err := OpenFence(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := fence.TryLock(ctx, "build-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fence.TryLock(ctx, "build-2"); err == nil || kind(t, err) != contracts.Retryable {
		t.Fatalf("second TryLock err = %v, want Retryable", err)
	}
	select {
	case <-first.Lost():
		t.Fatal("fence lost while held")
	default:
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	select {
	case <-first.Lost():
	default:
		t.Error("Lost not closed after Close")
	}
	second, err := fence.TryLock(ctx, "build-2")
	if err != nil {
		t.Fatalf("lock after Close: %v", err)
	}
	_ = second.Close()
}

func TestOtRoPagesCannotReadOpsTables(t *testing.T) {
	t.Parallel()
	name := newSchedulerDatabase(t)
	c := connectAs(t, name, "ot_ro_pages")
	for _, tbl := range []string{"job_gap", "scheduler_state", "scheduler_job", "dataset_window", "capture_present"} {
		mustFail(t, c, "SELECT * FROM ops."+tbl)
	}
}

func TestOtRoPagesReadsViews(t *testing.T) {
	t.Parallel()
	name := newSchedulerDatabase(t)
	ctx := context.Background()
	runner, err := OpenSchedulerLedger(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runner.Close() })
	ops, err := Open(ctx, connURL(t, name, "ot_runner"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ops.Close() })
	due := time.Date(2026, 11, 5, 22, 0, 0, 0, time.UTC)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(ops.PutWindow(ctx, store.Window{DatasetID: "obs.series_point", Day: "2026-11-05", OpenAt: due.Add(-time.Hour), CloseAt: due}))
	must(ops.PutWindow(ctx, store.Window{DatasetID: "obs.option_eod", Day: "2026-11-06"}))
	must(ops.MarkPresent(ctx, "2026-11-04", contracts.ExpectedKey{DatasetID: "obs.series_point", SubjectKey: "DGS10"}, store.StatusPresent))
	gap := store.Gap{DatasetID: "obs.series_point", Day: "2026-11-05", Reason: "expired"}
	must(runner.PutGap(ctx, gap))
	must(runner.PutGap(ctx, gap))
	must(runner.MarkGapReported(ctx, gap))

	reader, err := OpenGapReader(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	gaps, err := reader.OpenGaps(ctx)
	must(err)
	if len(gaps) != 1 || gaps[0].Reason != "expired" || gaps[0].FirstSeenAt.IsZero() || gaps[0].ReportedAt.IsZero() || !gaps[0].ClearedAt.IsZero() {
		t.Fatalf("gaps = %+v", gaps)
	}
	exps, err := reader.Expectations(ctx, "2026-11-05", "2026-11-06")
	must(err)
	want := []store.Expectation{
		{DatasetID: "obs.series_point", ExpectedDay: "2026-11-05", PresentDay: "2026-11-04", DueBy: due, Gap: "expired"},
		{DatasetID: "obs.option_eod", ExpectedDay: "2026-11-06", Closed: true},
	}
	if len(exps) != 2 || exps[0] != want[0] || exps[1] != want[1] {
		t.Fatalf("expectations = %+v, want %+v", exps, want)
	}
	must(runner.ClearGap(ctx, gap))
	if gaps, err = reader.OpenGaps(ctx); err != nil || len(gaps) != 0 {
		t.Fatalf("after clear: %+v, %v", gaps, err)
	}

	must(runner.SetState(ctx, "build-1"))
	must(runner.PutJobStates(ctx, []store.JobState{{JobID: "fred", NextStart: due}}))
	st, err := runner.Status(ctx)
	must(err)
	if st.Build != "build-1" || len(st.Jobs) != 1 || len(st.Gaps) != 0 {
		t.Errorf("status = %+v", st)
	}
}

func TestExpectationPresentDayNotAfterExpected(t *testing.T) {
	t.Parallel()
	name := newSchedulerDatabase(t)
	ctx := context.Background()
	c := connectAs(t, name, "ot_runner")
	for _, q := range []string{
		`INSERT INTO ops.dataset_window VALUES ('obs.x','2026-11-02','2026-11-02 14:30Z','2026-11-02 22:00Z'),
		                                       ('obs.x','2026-11-03','2026-11-03 14:30Z','2026-11-03 22:00Z')`,
		`INSERT INTO ops.capture_present (session_day, dataset_id, subject_key, status) VALUES ('2026-11-03','obs.x','SPY','present')`,
	} {
		if _, err := c.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	r, err := OpenGapReader(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.Expectations(ctx, "2026-11-02", "2026-11-02")
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].PresentDay > got[0].ExpectedDay {
		t.Errorf("PresentDay %s after ExpectedDay %s: Freshness reads INVALID", got[0].PresentDay, got[0].ExpectedDay)
	}
}

func TestExpectationPresentDayNeedsCompleteDay(t *testing.T) {
	t.Parallel()
	name := newSchedulerDatabase(t)
	ctx := context.Background()
	c := connectAs(t, name, "ot_runner")
	for _, q := range []string{
		`INSERT INTO ops.dataset_window VALUES ('obs.x','2026-11-03','2026-11-03 14:30Z','2026-11-03 22:00Z')`,
		`INSERT INTO ops.capture_expected_key VALUES ('2026-11-03','h','obs.x','SPY'),('2026-11-03','h','obs.x','QQQ')`,
		`INSERT INTO ops.capture_present VALUES ('2026-11-03','obs.x','SPY','present')`,
	} {
		if _, err := c.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	r, err := OpenGapReader(ctx, connURL(t, name, "ot_ro_pages"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	got, err := r.Expectations(ctx, "2026-11-03", "2026-11-03")
	if err != nil || len(got) != 1 || got[0].PresentDay != "" {
		t.Fatalf("one key of two present: %+v %v", got, err)
	}
	if _, err := c.Exec(ctx, `INSERT INTO ops.capture_present VALUES ('2026-11-03','obs.x','QQQ','no_value')`); err != nil {
		t.Fatal(err)
	}
	got, err = r.Expectations(ctx, "2026-11-03", "2026-11-03")
	if err != nil || len(got) != 1 || got[0].PresentDay != "2026-11-03" {
		t.Fatalf("all keys settled: %+v %v", got, err)
	}
}
