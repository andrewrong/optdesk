package optionopen

import (
	"context"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// jpTenPlanned plans 10 JP contracts like the 60-day quote test: puts, 60 days after the session day.
func jpTenPlanned(t *testing.T, opt func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error)) (contracts.JobResult, *pullStores) {
	t.Helper()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	s := seed(1, 10)
	for i := range s.items {
		s.items[i].Class = "puts"
		s.refs[1][i].Expiry = "2027-01-10"
		s.refs[1][i].Strike = strconv.Itoa(95 - i)
	}
	b := &fakeBroker{clk: clk, spots: []float64{100}}
	b.opt = opt(b)
	j, err := NewPullJP(PullDeps{Broker: b, Plans: jpPlans{s}, Contracts: s, Writer: s, Instruments: s, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), "2026-11-11", keys), s
}

// jpSixPartial answers a bid only (partial) for the strikes below 90 while partial is set, else ok; the strikes in
// okAlso answer ok even then.
func jpSixPartial(
	cutoff string, partial *bool, okAlso map[string]bool, mu *sync.Mutex,
) func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
	return func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := okQuotes(b.clk, cs)
			mu.Lock()
			defer mu.Unlock()
			for i := range qs {
				if !*partial || cs[i].Strike < cutoff || okAlso[cs[i].Strike] {
					continue
				}
				bid := qs[i].Bid
				qs[i] = broker.Quote{Contract: cs[i], Status: broker.StatusOK, Bid: bid, ReceivedAt: b.clk.Now()}
			}
			return qs, nil
		}
	}
}

func TestJPPartialUnderShareRetries(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	partial := true
	j, keys, s, _ := jpRerun(t, jpSixPartial("90", &partial, nil, &mu))
	res := j.Run(context.Background(), "2026-11-11", keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Usable != 4 || res.Returned != 10 {
		t.Errorf("err %v usable %d returned %d, want Retryable with the 10 rows written", res.Err, res.Usable, res.Returned)
	}
	if s.total() != 10 {
		t.Errorf("written %d rows, want 10", s.total())
	}
	before := map[string]contracts.OptionOpen{}
	maps.Copy(before, s.stored)
	res = j.Run(context.Background(), "2026-11-11", keys)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("second attempt: %v, want Permanent", res.Err)
	}
	msg := res.Err.Error()
	if strings.Contains(msg, "slow broker") || !strings.Contains(msg, "ok=4 partial=6") ||
		!strings.Contains(msg, "planned") || !strings.Contains(msg, "did not change") {
		t.Errorf("error text %q", msg)
	}
	if len(s.stored) != len(before) {
		t.Fatalf("stored %d rows, want %d", len(s.stored), len(before))
	}
	for k, r := range s.stored {
		if !reflect.DeepEqual(r, before[k]) {
			t.Errorf("row %s was rewritten", k)
		}
	}
}

func TestJPPartialChangedStaysRetryable(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	partial := true
	okAlso := map[string]bool{}
	j, keys, _, _ := jpRerun(t, jpSixPartial("88", &partial, okAlso, &mu))
	if res := j.Run(context.Background(), "2026-11-11", keys); res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("first attempt: %v, want Retryable", res.Err)
	}
	mu.Lock()
	okAlso["95"] = true // one more row becomes ok
	mu.Unlock()
	res := j.Run(context.Background(), "2026-11-11", keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Usable != 3 {
		t.Errorf("second attempt: err %v usable %d, want Retryable with 3 ok", res.Err, res.Usable)
	}
}

func TestJPNoDataUnderSharePermanent(t *testing.T) {
	t.Parallel()
	res, _ := jpTenPlanned(t, func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := make([]broker.Quote, len(cs))
			for i, c := range cs {
				qs[i] = broker.Quote{Contract: c, Status: broker.StatusNoData, ReceivedAt: b.clk.Now()}
			}
			return qs, nil
		}
	})
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || !strings.Contains(res.Err.Error(), "0 of 10 planned items ok, under the share 0.50") {
		t.Errorf("err %v, want Permanent 0 of 10 planned items ok", res.Err)
	}
}

// jpRerun builds the JP job over ten planned puts and returns it with its keys, the stores and the broker.
func jpRerun(
	t *testing.T, opt func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error),
) (contracts.Job, []contracts.ExpectedKey, *pullStores, *fakeBroker) {
	t.Helper()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	s := seed(1, 10)
	for i := range s.items {
		s.items[i].Class = "puts"
		s.refs[1][i].Expiry = "2027-01-10"
		s.refs[1][i].Strike = strconv.Itoa(95 - i)
	}
	b := &fakeBroker{clk: clk, spots: []float64{100}}
	b.opt = opt(b)
	j, err := NewPullJP(PullDeps{Broker: b, Plans: jpPlans{s}, Contracts: s, Writer: s, Instruments: s, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	return j, keys, s, b
}

func TestJPRerunAsksOnlyNonFinalItems(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var asked [][]string
	partial := true
	j, keys, s, _ := jpRerun(t, func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := okQuotes(b.clk, cs)
			mu.Lock()
			defer mu.Unlock()
			var round []string
			for i := range qs {
				round = append(round, cs[i].Strike)
				if partial && cs[i].Strike < "90" { // the 4 strikes 89 to 86 answer a bid only: partial
					bid := qs[i].Bid
					qs[i] = broker.Quote{Contract: cs[i], Status: broker.StatusOK, Bid: bid, ReceivedAt: b.clk.Now()}
				}
			}
			asked = append(asked, round)
			return qs, nil
		}
	})
	res := j.Run(context.Background(), "2026-11-11", keys)
	if res.Usable != 6 {
		t.Fatalf("first run: err %v usable %d, want 6 ok", res.Err, res.Usable)
	}
	mu.Lock()
	partial, asked = false, nil
	mu.Unlock()
	res = j.Run(context.Background(), "2026-11-11", keys)
	if res.Err != nil || res.Usable != 10 || res.Returned != 10 {
		t.Errorf("second run: err %v usable %d returned %d, want ok 10 of 10", res.Err, res.Usable, res.Returned)
	}
	var got []string
	for _, r := range asked {
		got = append(got, r...)
	}
	sort.Strings(got)
	if want := []string{"86", "87", "88", "89"}; !slices.Equal(got, want) {
		t.Errorf("second run asked strikes %v, want %v", got, want)
	}
	if len(s.stored) != 10 {
		t.Fatalf("stored %d rows, want 10", len(s.stored))
	}
	for k, r := range s.stored {
		if r.Status != stOK {
			t.Errorf("row %s status %d, want ok", k, r.Status)
		}
	}
}

func TestJPRerunAllFinalAsksNothing(t *testing.T) {
	t.Parallel()
	j, keys, s, b := jpRerun(t, func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) { return okQuotes(b.clk, cs), nil }
	})
	if res := j.Run(context.Background(), "2026-11-11", keys); res.Err != nil || res.Usable != 10 {
		t.Fatalf("first run: err %v usable %d, want ok 10", res.Err, res.Usable)
	}
	writes, calls := len(s.written), len(b.optCalls)
	res := j.Run(context.Background(), "2026-11-11", keys)
	if res.Err != nil || res.Usable != 10 || res.Returned != 10 {
		t.Errorf("rerun: err %v usable %d returned %d, want ok 10 of 10", res.Err, res.Usable, res.Returned)
	}
	if len(s.written) != writes || len(b.optCalls) != calls {
		t.Errorf("rerun wrote %d times and asked %d times more, want none", len(s.written)-writes, len(b.optCalls)-calls)
	}
}
