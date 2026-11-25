package optionopen

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestPullWithoutSpotAsksThePlannedItems(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.under = func(int) ([]broker.Quote, error) { return []broker.Quote{{Status: broker.StatusNoData}}, nil }
	asked, cut := target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put)
	cut.Item.Deferred = true
	r := newCore(b).Pull(context.Background(), []Target{asked, cut})
	if len(b.optCalls) != 1 || len(b.optCalls[0]) != 1 {
		t.Fatalf("option calls %v, want one call of one contract", b.optCalls)
	}
	a := rowOf(t, r, 11)
	if a.Status != stOK || a.UnderlyingPrice != nil || a.UnderlyingEventTs != nil || a.Skew || a.OutsideBand {
		t.Errorf("asked row %+v, want status ok, no underlying price, no skew, not outside the band", a)
	}
	d := rowOf(t, r, 12)
	if d.Status != stDeferred || code(d) != codePlanCut || d.UnderlyingPrice != nil {
		t.Errorf("plan-deferred row status %d code %q, want deferred PLAN_CUT", d.Status, code(d))
	}
}

func TestPullWithoutSpotKeepsTheDisconnect(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	b.under = func(int) ([]broker.Quote, error) { return nil, typedErr{contracts.Retryable} }
	r := newCore(b).Pull(context.Background(), []Target{target(1, 11, ClassAroundSpot, 95, Put), target(1, 12, ClassPuts, 90, Put)})
	if len(b.optCalls) != 0 {
		t.Errorf("option calls %d, want none", len(b.optCalls))
	}
	if len(r.Rows) != 2 {
		t.Fatalf("rows %d, want 2", len(r.Rows))
	}
	for _, row := range r.Rows {
		if row.Status != stDisconnected || code(row) != "UNAVAILABLE" {
			t.Errorf("status %d code %q, want disconnected UNAVAILABLE", row.Status, code(row))
		}
	}
}
