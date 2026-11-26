package main

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

func TestConsoleRerunSyncsKeyGaps(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		res    runner.BackfillResult
		err    error
		state  jobrequest.State
		reason string
		sync   int
	}{
		{"rerun done", jcLines(runner.GroupRerun), nil, jobrequest.StateDone, "completed", 1},
		{"rerun failed", jcLines(runner.GroupRerun), errors.New("other"), jobrequest.StateFailed, "error", 1},
		{"ran done", jcLines(runner.GroupRan), nil, jobrequest.StateDone, "completed", 1},
		{"ran failed", jcLines(runner.GroupRan), errors.New("other"), jobrequest.StateFailed, "error", 1},
		{"not due refused", jcLines(runner.GroupNotDue), nil, jobrequest.StateRefused, "not_due", 0},
		{"no key refused", runner.BackfillResult{Refused: runner.RefusedNoKey}, nil, jobrequest.StateRefused, "no_key", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r := jcOne(t, "fred_series")
			r.bf.res, r.bf.err = c.res, c.err
			r.turn(jcNever)
			r.wantEnd(c.state, c.reason)
			if n := r.log.count("sync:" + jcDay); n != c.sync {
				t.Fatalf("%d SyncKeys calls, want %d", n, c.sync)
			}
		})
	}
	t.Run("interrupted", func(t *testing.T) {
		t.Parallel()
		r := jcOne(t, "fred_series")
		ctx, cancel := context.WithCancel(t.Context())
		r.bf.fn = func(context.Context) (runner.BackfillResult, error) {
			cancel()
			return jcLines(runner.GroupRerun), context.Canceled
		}
		_, _ = r.host.ConsoleTurn(ctx, jcNever)
		if n := r.log.count("sync"); n != 0 {
			t.Fatalf("%d SyncKeys calls after interrupt", n)
		}
	})
}
