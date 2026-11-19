package optioneod

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func strikePairs(n int) []broker.Contract {
	out := make([]broker.Contract, n)
	for i := range out {
		out[i] = broker.Contract{Symbol: "TEST", Strike: strconv.Itoa(i + 1), Right: "C", Multiplier: "100"}
	}
	return out
}

func strikeOf(c broker.Contract) int { n, _ := strconv.Atoi(c.Strike); return n }

func noBudget() contracts.TypedError { return nil }

func TestResolvePairsInFlight(t *testing.T) {
	t.Parallel()
	var cur, peak atomic.Int32
	entered := make(chan struct{}, 40)
	release := make(chan struct{})
	resolve := func(_ context.Context, c broker.Contract) (broker.Resolved, error) {
		n := cur.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		cur.Add(-1)
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	done := make(chan []broker.Contract)
	go func() {
		got, terr := resolvePairs(context.Background(), resolve, strikePairs(40), noBudget)
		if terr != nil {
			t.Error(terr)
		}
		done <- got
	}()
	for i := 0; i < resolveInFlight; i++ { // the pool fills to its width
		<-entered
	}
	go func() {
		for range entered {
		}
	}()
	close(release)
	got := <-done
	close(entered)
	if p := peak.Load(); p < 2 || p > resolveInFlight {
		t.Fatalf("peak in flight %d, want 2..%d", p, resolveInFlight)
	}
	if len(got) != 40 {
		t.Fatalf("listed %d, want 40", len(got))
	}
}

func TestResolvePairsOrder(t *testing.T) {
	t.Parallel()
	pairs := strikePairs(5)
	// pair k answers only after every pair above it has: reverse order
	gates := make([]chan struct{}, len(pairs))
	for i := range gates {
		gates[i] = make(chan struct{})
	}
	resolve := func(_ context.Context, c broker.Contract) (broker.Resolved, error) {
		k := strikeOf(c) - 1
		if k < len(pairs)-1 {
			<-gates[k+1]
		}
		defer close(gates[k])
		if k == 2 {
			return broker.Resolved{}, nil // not listed
		}
		c.ConID = int64(k + 100)
		c.Multiplier = ""
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	got, terr := resolvePairs(context.Background(), resolve, pairs, noBudget)
	if terr != nil {
		t.Fatal(terr)
	}
	want := []int{1, 2, 4, 5}
	if len(got) != len(want) {
		t.Fatalf("got %d pairs, want %d", len(got), len(want))
	}
	for i, c := range got {
		if strikeOf(c) != want[i] || c.Multiplier != "100" {
			t.Fatalf("pair %d: strike %s multiplier %q", i, c.Strike, c.Multiplier)
		}
	}
}

func TestResolvePairsFirstErrorInOrder(t *testing.T) {
	t.Parallel()
	err3, err9 := errors.New("pair 3"), errors.New("pair 9")
	var mu sync.Mutex
	asked := map[int]bool{}
	started := make(chan struct{}, 40)
	release := make(chan struct{})
	fourth := make(chan struct{})
	resolve := func(_ context.Context, c broker.Contract) (broker.Resolved, error) {
		k := strikeOf(c) - 1
		mu.Lock()
		asked[k] = true
		mu.Unlock()
		started <- struct{}{}
		if k == 4 {
			close(fourth)
		}
		switch k {
		case 3:
			<-fourth // pair 4 is in flight when pair 3 fails
			return broker.Resolved{}, err3
		case 9:
			return broker.Resolved{}, err9
		}
		<-release // the other asks stay in flight until the test lets them go
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	done := make(chan contracts.TypedError)
	go func() {
		_, terr := resolvePairs(context.Background(), resolve, strikePairs(40), noBudget)
		done <- terr
	}()
	for i := 0; i < resolveInFlight; i++ { // pairs 0..4 are the five in flight
		<-started
	}
	for i := 0; i < 1000; i++ { // let the failure of pair 3 be recorded before the others end
		runtime.Gosched()
	}
	close(release)
	terr := <-done
	if terr == nil || !errors.Is(terr, err3) {
		t.Fatalf("error %v, want that of pair 3", terr)
	}
	mu.Lock()
	defer mu.Unlock()
	// nothing after the five first pairs is asked once pair 3 failed
	for k := 5; k < 40; k++ {
		if asked[k] {
			t.Fatalf("pair %d asked after pair 3 failed", k)
		}
	}
}

func TestResolvePairsBudget(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2026-11-06 11:00")
	rs := &runState{j: r.job, start: r.clk.Now()}
	r.clk.Advance(time.Duration(r.job.m.BudgetMinutes)*time.Minute + time.Second)
	_, terr := resolvePairs(context.Background(), r.b.ResolveContract, strikePairs(10), rs.checkBudget)
	if terr == nil || terr.Kind() != contracts.Retryable {
		t.Fatalf("error %v, want Retryable", terr)
	}
	if r.b.resolves != 0 {
		t.Fatalf("asked %d, want 0", r.b.resolves)
	}
}
