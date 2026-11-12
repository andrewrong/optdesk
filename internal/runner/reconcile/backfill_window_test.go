package reconcile

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// heldLedger is a fake ledger whose run lease is held by holder: Acquire under another
// owner is a Retryable error.
type heldLedger struct {
	*spyLedger
	holder string
}

func (h *heldLedger) Acquire(ctx context.Context, o string, ttl time.Duration) (store.Lease, error) {
	if !strings.HasPrefix(o, h.holder) {
		return store.Lease{}, kindErr{kind: contracts.Retryable}
	}
	return h.spyLedger.Acquire(ctx, o, ttl)
}

func leaseRetryable(err error) bool {
	te, ok := err.(contracts.TypedError)
	return ok && te.Kind() == contracts.Retryable
}

func TestBackfillAheadOfCursor(t *testing.T) {
	t.Parallel()
	for _, cursor := range []contracts.Day{"", "2026-11-03"} {
		l := bfSetup(cursor, keyA)
		job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
		res, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), bfDay, nil)
		if err != nil || res.Refused != "" || len(res.Filled) != 1 || len(job.calls) != 1 {
			t.Fatalf("cursor %q: res %+v err %v", cursor, res, err)
		}
	}
}

func TestBackfillRunsInWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clk := clockwork.NewFakeClockAt(utc("2026-11-04T20:00:00Z"))
	for _, mark := range []store.PresentStatus{"", store.StatusNoValue} {
		l := bfSetup(bfDay, keyA)
		if mark != "" {
			l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: mark}
		}
		if err := l.PutWindow(ctx, store.Window{DatasetID: "ds", Day: bfDay, CloseAt: utc("2026-11-04T21:00:00Z")}); err != nil {
			t.Fatal(err)
		}
		job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
		res, err := newBackfiller(l, clk, job).Backfill(ctx, bfDay, nil)
		if err != nil || len(res.InWindow) != 0 || len(job.calls) != 1 {
			t.Fatalf("mark %q: res %+v err %v calls %d", mark, res, err, len(job.calls))
		}
		if len(res.Filled)+len(res.Absent) != 1 {
			t.Fatalf("mark %q: res %+v", mark, res)
		}
	}
}

func TestBackfillExpiredKey(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	l := bfSetup("2026-11-03", keyA)
	l.present[bfDay] = map[contracts.ExpectedKey]store.PresentStatus{keyA: store.StatusNoValue}
	l.runs[bfDay] = []store.Run{{ID: "2026-11-04|ds|a|1", Day: bfDay, Key: keyA, Status: store.RunDone, ReasonCode: "expired"}}
	if err := l.PutWindow(ctx, store.Window{DatasetID: "ds", Day: bfDay, CloseAt: utc("2026-11-04T21:00:00Z")}); err != nil {
		t.Fatal(err)
	}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	res, err := newBackfiller(l, afterClose(), job).Backfill(ctx, bfDay, nil)
	if err != nil || len(res.Filled) != 1 || len(job.calls) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if l.present[bfDay][keyA] != store.StatusPresent {
		t.Fatalf("status %q", l.present[bfDay][keyA])
	}
}

func TestBackfillOwnLease(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// A lease held by the runner excludes a backfill.
	h := &heldLedger{spyLedger: bfSetup(bfDay, keyA), holder: "runner"}
	job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	if _, err := newBackfiller(h, afterClose(), job).Backfill(ctx, bfDay, nil); !leaseRetryable(err) || len(job.calls) != 0 {
		t.Fatalf("backfill under runner lease: err %v calls %d", err, len(job.calls))
	}

	// A lease held by a backfill excludes Reconcile.
	h = &heldLedger{spyLedger: bfSetup("2026-11-03"), holder: "backfill:"}
	job = &fakeJob{keys: []contracts.ExpectedKey{keyA}}
	rec := NewWithRule(h, oneDay(), afterClose(), []contracts.Job{job}, runner.WindowRule{CalendarID: "XNYS"})
	if err := rec.Reconcile(ctx, bfDay); !leaseRetryable(err) || len(job.calls) != 0 {
		t.Fatalf("reconcile under backfill lease: err %v calls %d", err, len(job.calls))
	}

	// The backfill renews its lease under its own owner.
	clk := clockwork.NewFakeClockAt(utc("2026-11-06T00:00:00Z"))
	led := newRkLedger()
	led.cursor = store.Cursor{Day: "2026-11-03"}
	led.expected[bfDay], led.hash[bfDay] = []contracts.ExpectedKey{keyA}, "plan-1"
	sched := &rkSched{renewed: make(chan store.Lease, 8)}
	bj := newBlockJob(contracts.JobResult{Returned: 1, Usable: 1})
	_, bf := NewReconciler(led, sched, oneDay(), clk, []contracts.Job{bj}, runner.WindowRule{CalendarID: "XNYS"}, nil)
	done := make(chan error, 1)
	go func() { _, err := bf.Backfill(ctx, bfDay, nil); done <- err }()
	<-bj.started
	if err := clk.BlockUntilContext(ctx, 3); err != nil {
		t.Fatal(err)
	}
	clk.Advance(31 * time.Second)
	l := <-sched.renewed
	if !strings.HasPrefix(l.Owner, "backfill:") || len(l.Owner) != len("backfill:")+8 {
		t.Fatalf("renewed lease of %q", l.Owner)
	}
	close(bj.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestBackfillCursorUnmoved(t *testing.T) {
	t.Parallel()
	for _, cursor := range []contracts.Day{"", "2026-11-03", bfDay, "2026-11-05"} {
		l := bfSetup(cursor, keyA)
		job := &fakeJob{keys: []contracts.ExpectedKey{keyA}}
		if _, err := newBackfiller(l, afterClose(), job).Backfill(context.Background(), bfDay, nil); err != nil {
			t.Fatal(err)
		}
		if l.cursor != (store.Cursor{Day: cursor}) || l.cursors.Load() != 0 {
			t.Fatalf("cursor %q became %q", cursor, l.cursor.Day)
		}
	}
}
