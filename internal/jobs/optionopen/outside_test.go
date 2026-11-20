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

// gappedPlan seeds one stock with n planned puts: the first inside strikes at 99 down, the rest at far strikes the live
// band at a spot of 100 drops. The ids are 1000+k.
func gappedPlan(n, inside int, expiry string) *pullStores {
	// A put the guard of six puts keeps is never outside the band, so the far ones sit beyond the guard.
	s := seed(1, n)
	for i := range s.items {
		s.items[i].Class = "puts"
		s.refs[1][i].Expiry = contracts.Day(expiry)
		if i < inside {
			s.refs[1][i].Strike = strconv.FormatFloat(99-0.4*float64(i), 'f', 1, 64)
		} else {
			s.refs[1][i].Strike = strconv.Itoa(40 - i%20)
		}
	}
	return s
}

func runGapped(t *testing.T, s *pullStores, jp bool, quote func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error)) contracts.JobResult {
	t.Helper()
	b := &fakeBroker{clk: clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC)), spots: []float64{100}}
	b.opt = quote(b)
	deps := PullDeps{Broker: b, Plans: s, Contracts: s, Writer: s, Instruments: s, Clock: b.clk}
	newJob := NewPull
	if jp {
		deps.Plans = jpPlans{s}
		newJob = NewPullJP
	} else {
		b.clk = clockwork.NewFakeClockAt(open0945)
		deps.Clock = b.clk
	}
	j, err := newJob(deps)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), pullDay)
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), pullDay, keys)
}

func allOK(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
	return func(_ int, cs []broker.Contract) ([]broker.Quote, error) { return okQuotes(b.clk, cs), nil }
}

// okFirst answers ok for the first n contracts asked (by id order of the strike list) and no data for the others.
func okFirst(n int) func(*fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
	return func(b *fakeBroker) func(int, []broker.Contract) ([]broker.Quote, error) {
		return func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
			qs := okQuotes(b.clk, cs)
			for i := range qs {
				if cs[i].ConID/10%1000 >= int64(n) {
					qs[i] = broker.Quote{Contract: cs[i], Status: broker.StatusNoData, ReceivedAt: b.clk.Now()}
				}
			}
			return qs, nil
		}
	}
}

func TestUSOutsideBandItemsAreNotInThePlannedShare(t *testing.T) {
	t.Parallel()
	s := gappedPlan(52, 8, "2026-12-11")
	res := runGapped(t, s, false, allOK)
	if res.Err != nil || res.Usable != 8 {
		t.Errorf("err %v usable %d, want no error with 8 ok of the 8 asked", res.Err, res.Usable)
	}
}

func TestJPOutsideBandItemsAreNotInThePlannedShare(t *testing.T) {
	t.Parallel()
	s := gappedPlan(52, 8, "2027-01-10")
	res := runGapped(t, s, true, allOK)
	if res.Err != nil || res.Usable != 8 {
		t.Errorf("err %v usable %d, want no error with 8 ok of the 8 asked", res.Err, res.Usable)
	}
}

func TestAllOutsideBandIsNoValue(t *testing.T) {
	t.Parallel()
	for _, jp := range []bool{false, true} {
		// An expiry beyond the longest DTE of either market: the live band keeps none of the items.
		s := gappedPlan(20, 0, "2027-06-11")
		res := runGapped(t, s, jp, allOK)
		if res.Err == nil || res.Err.Kind() != contracts.NoValue || !strings.Contains(res.Err.Error(), "all outside band") {
			t.Errorf("jp %v: err %v, want NoValue all outside band", jp, res.Err)
		}
	}
}

func TestShareStillBitesOnWhatWasAsked(t *testing.T) {
	t.Parallel()
	for _, jp := range []bool{false, true} {
		expiry := "2026-12-11"
		if jp {
			expiry = "2027-01-10"
		}
		s := gappedPlan(52, 22, expiry)
		res := runGapped(t, s, jp, okFirst(5))
		if res.Err == nil || res.Err.Kind() != contracts.Permanent || !strings.Contains(res.Err.Error(), "outside_band=30") ||
			!strings.Contains(res.Err.Error(), "5 of 22 planned items ok") {
			t.Errorf("jp %v: err %v, want Permanent under share naming outside_band=30", jp, res.Err)
		}
	}
}
