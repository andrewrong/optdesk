package shared_test

import (
	"testing"

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
