package sched

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestKeyKindIgnoresConsoleStop(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC))
	g := &gapSyncer{clk: clk}
	retryable := store.Run{ID: "1", Status: store.RunDone, ErrorCode: string(contracts.Retryable)}
	stopped := func(id string, st store.RunStatus) store.Run {
		return store.Run{ID: id, Status: st, ReasonCode: "console_stop"}
	}
	for _, tc := range []struct {
		name string
		runs []store.Run
		want string
	}{
		{"newest console_stop does not hide a retryable run", []store.Run{retryable, stopped("2", store.RunAbandoned)}, gapExpired},
		{"a partial console_stop run is no newest run", []store.Run{retryable, stopped("2", store.RunPartial)}, gapExpired},
		{"three console_stop rows are no crash strike", []store.Run{
			stopped("1", store.RunAbandoned), stopped("2", store.RunAbandoned), stopped("3", store.RunAbandoned),
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := g.keyKind(tc.runs, "", true); got != tc.want {
				t.Fatalf("keyKind = %q, want %q", got, tc.want)
			}
		})
	}
}
