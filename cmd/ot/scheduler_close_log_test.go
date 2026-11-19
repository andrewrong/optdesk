package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/profile"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// threeClosedLedger is an opened ledger whose CloseRunning closes three runs.
type threeClosedLedger struct{ store.OpsLedger }

func (threeClosedLedger) CloseRunning(context.Context, time.Time, string) (int, error) { return 3, nil }

func TestCloseRunningAtStartLogsRows(t *testing.T) {
	t.Parallel()
	p, err := profile.Load(func(string) string { return "" })
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &app{profile: p, log: slog.New(profile.NewLogHandler(p, slog.NewTextHandler(&buf, nil)))}
	a.deps.openSchedulerLedger = func(context.Context, string) (store.SchedulerLedger, error) { return &fakeSchedLedger{}, nil }
	if err := closeRunningAtStart(t.Context(), a, clockwork.NewFakeClock(), threeClosedLedger{}, "dsn"); err != nil {
		t.Fatal(err)
	}
	line := buf.String()
	for _, want := range []string{"rows=3", "status="} {
		if !strings.Contains(line, want) {
			t.Fatalf("log line %q lacks %q", line, want)
		}
	}
}
