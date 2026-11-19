package optionopen

import (
	"context"
	"strconv"
	"strings"
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

func TestJPPartialUnderShareRetries(t *testing.T) {
	t.Parallel()
	res, s := jpTenPlanned(t, func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := okQuotes(b.clk, cs)
			for i := range qs {
				if cs[i].Strike < "90" { // the 6 strikes 95 to 90 answer a bid and no ask or IV: partial
					continue
				}
				bid := qs[i].Bid
				qs[i] = broker.Quote{Contract: cs[i], Status: broker.StatusOK, Bid: bid, ReceivedAt: b.clk.Now()}
			}
			return qs, nil
		}
	})
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || res.Usable != 4 || res.Returned != 10 {
		t.Errorf("err %v usable %d returned %d, want Retryable with the 10 rows written", res.Err, res.Usable, res.Returned)
	}
	if s.total() != 10 {
		t.Errorf("written %d rows, want 10", s.total())
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
