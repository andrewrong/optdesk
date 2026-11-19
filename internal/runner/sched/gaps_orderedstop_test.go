package sched

import (
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestKeyKindIgnoresOrderedStop(t *testing.T) {
	t.Parallel()
	g := &gapSyncer{clk: clockwork.NewFakeClockAt(time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC))}
	var runs []store.Run
	for _, id := range []string{"1", "2", "3"} {
		runs = append(runs, store.Run{ID: id, Status: store.RunAbandoned, ReasonCode: "ordered_stop"})
	}
	if got := g.keyKind(runs, "", true); got != "" {
		t.Fatalf("keyKind = %q, want none", got)
	}
	var plain []store.Run
	for _, id := range []string{"1", "2", "3"} {
		plain = append(plain, store.Run{ID: id, Status: store.RunAbandoned, ReasonCode: "abandoned_stop"})
	}
	if got := g.keyKind(plain, "", true); got != gapCrashedRepeated {
		t.Fatalf("keyKind = %q, want %q", got, gapCrashedRepeated)
	}
}
