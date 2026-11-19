package optionopenpg

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

func TestOptionOpenSlotNoIdsNoCall(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	for _, ids := range [][]int64{nil, {}} {
		got, err := s.OptionOpenSlot(context.Background(), "2026-11-11", "0945", ids)
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("got %v, %v", got, err)
		}
	}
	if f.inTx != 0 || len(f.sqls) != 0 {
		t.Fatalf("calls: %d transactions, %v", f.inTx, f.sqls)
	}
}

func TestOptionOpenSlotRefusesRefreshSlot(t *testing.T) {
	t.Parallel()
	s, f := newStore()
	_, err := s.OptionOpenSlot(context.Background(), "2026-11-11", openquote.SlotRefresh, []int64{1})
	if kind(err) != contracts.Permanent {
		t.Fatalf("err = %v, kind %q", err, kind(err))
	}
	if len(f.sqls) != 0 {
		t.Fatalf("calls: %v", f.sqls)
	}
}
