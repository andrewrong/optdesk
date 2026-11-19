package sched

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner/shared"
)

// causeRec blocks in Reconcile until its context ends, then reports the cause of the end.
type causeRec struct {
	ev    events
	cause chan error
}

func (r *causeRec) Reconcile(ctx context.Context, day contracts.Day) error {
	r.ev.emit("reconcile:" + string(day))
	<-ctx.Done()
	r.cause <- context.Cause(ctx)
	return nil
}

func TestDrainCancelsWorkerWithOrderedStop(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		trigger func(h *harness, cancel context.CancelFunc)
		want    error // the Run result
		ordered bool
	}{
		{"interrupt", func(_ *harness, cancel context.CancelFunc) { cancel() }, nil, true},
		{"executable changed", func(h *harness, _ context.CancelFunc) { h.step(10 * time.Minute) }, ErrExecutableChanged, true},
		{"fence lost", func(h *harness, _ context.CancelFunc) { close(h.fence.lost) }, ErrFenceLost, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			rec := &causeRec{ev: h.ev, cause: make(chan error, 1)}
			h.rec = rec
			var calls atomic.Int32
			h.stuck = true // the worker stays busy in Reconcile
			if tc.want == ErrExecutableChanged {
				h.exe = func() (string, error) {
					if calls.Add(1) > 1 {
						return "new", nil
					}
					return "old", nil
				}
			}
			cancel, done := h.run()
			h.until("reconcile:2025-11-26")
			tc.trigger(h, cancel)
			var got error
			for finished := false; !finished; {
				select {
				case got = <-done:
					finished = true
				default:
					h.clk.Advance(time.Second)
					runtime.Gosched()
				}
			}
			if !errors.Is(got, tc.want) || (tc.want == nil && got != nil) {
				t.Fatalf("Run = %v, want %v", got, tc.want)
			}
			cause := <-rec.cause
			if gotOrdered := errors.Is(cause, shared.ErrOrderedStop); gotOrdered != tc.ordered {
				t.Fatalf("cause = %v, ordered stop %v, want %v", cause, gotOrdered, tc.ordered)
			}
		})
	}
}
