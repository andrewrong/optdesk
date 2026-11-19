package shared_test

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/shared"
)

func TestJobLocksTryLock(t *testing.T) {
	t.Parallel()
	locks := shared.NewJobLocks()
	unlock, ok := locks.TryLock("a")
	if !ok {
		t.Fatal("first TryLock refused")
	}
	if _, ok := locks.TryLock("a"); ok {
		t.Fatal("second TryLock of a locked job answered ok")
	}
	other, ok := locks.TryLock("b")
	if !ok {
		t.Fatal("another job refused")
	}
	other()
	unlock()
	unlock() // a second call is harmless
	again, ok := locks.TryLock("a")
	if !ok {
		t.Fatal("released job not lockable again")
	}
	again()
}

func TestHolderReportsSince(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC))
	locks := shared.NewJobLocksAt(clk.Now)
	h, ok := locks.(runner.JobLockHolder)
	if !ok {
		t.Fatal("the lock set does not implement JobLockHolder")
	}
	if _, held := h.Holder("a"); held {
		t.Fatal("a free job reported as locked")
	}
	locked := clk.Now()
	unlock, _ := locks.TryLock("a")
	clk.Advance(time.Minute)
	if _, refused := locks.TryLock("a"); refused {
		t.Fatal("second TryLock answered ok")
	}
	since, held := h.Holder("a")
	if !held || !since.Equal(locked) {
		t.Fatalf("Holder = %v %v, want %v true (a refused TryLock keeps the first instant)", since, held, locked)
	}
	if _, other := h.Holder("b"); other {
		t.Fatal("another job reported as locked")
	}
	unlock()
	if _, held := h.Holder("a"); held {
		t.Fatal("released job reported as locked")
	}
}
