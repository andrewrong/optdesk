package optioneod

import (
	"context"
	"sync"
	"testing"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// aliasRecorder is a fakeAccount that records the aliases whose positions were read.
type aliasRecorder struct {
	fakeAccount
	mu   sync.Mutex
	read map[string]bool
}

func (r *aliasRecorder) Positions(ctx context.Context, alias, day string) ([]store.Position, error) {
	r.mu.Lock()
	if r.read == nil {
		r.read = map[string]bool{}
	}
	r.read[alias] = true
	r.mu.Unlock()
	return r.fakeAccount.Positions(ctx, alias, day)
}

func runHeldAliases(t *testing.T, acct store.AccountReader, aliases []string) int {
	t.Helper()
	b := heldBroker()
	clk := clockwork.NewFakeClockAt(etTime(t, "2026-11-06 17:00"))
	watch := []jobs.WatchEntry{watchEntry("US", "TEST", "STK", "100", 1000001)}
	ctor := WithHeldAliases(NewWithHeldSymbols, aliases)
	j, err := ctor(b, &fakeResolver{}, &fakeWriter{}, watch, acct, clk, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), testDay, []contracts.ExpectedKey{testKey}); res.Err != nil {
		t.Fatal(res.Err)
	}
	var got []broker.Contract
	for _, s := range b.snaps {
		if s.kind == broker.KindOptionEod {
			got = append(got, s.cs...)
		}
	}
	return count(got, "20261116", "100", "C")
}

func TestWithHeldAliasesReadsTheGivenAlias(t *testing.T) {
	t.Parallel()
	acct := &aliasRecorder{fakeAccount: fakeAccount{positions: map[string][]store.Position{
		"mine 2026-11-06": {shortCall("2026-11-16", "100")},
	}}}
	if n := runHeldAliases(t, acct, []string{"mine"}); n != 1 {
		t.Fatalf("held call of alias mine asked %d times, want 1", n)
	}
	if !acct.read["mine"] || len(acct.read) != 1 {
		t.Fatalf("aliases read = %v, want only mine", acct.read)
	}
}

func TestWithHeldAliasesNoneReadsNothing(t *testing.T) {
	t.Parallel()
	acct := &aliasRecorder{fakeAccount: fakeAccount{positions: map[string][]store.Position{
		"mine 2026-11-06": {shortCall("2026-11-16", "100")},
	}}}
	if n := runHeldAliases(t, acct, nil); n != 0 {
		t.Fatalf("held call asked %d times with no alias, want 0", n)
	}
	if len(acct.read) != 0 {
		t.Fatalf("aliases read = %v, want none", acct.read)
	}
}
