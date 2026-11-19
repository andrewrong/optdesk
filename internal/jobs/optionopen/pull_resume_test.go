package optionopen

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// storedRow is a row of the table for the instrument, with the status, for the day and slot.
func storedRow(day time.Time, slot string, id, status int) contracts.OptionOpen {
	return contracts.OptionOpen{SessionDay: types.SerializableDate{Time: day}, Slot: slot, InstrumentID: id, Status: status,
		CaptureID: "c" + slot, Currency: "USD"}
}

func asked(b *fakeBroker) int {
	n := 0
	for _, c := range b.optCalls {
		n += len(c)
	}
	return n
}

func TestPullFlushesRowsAfterEachGroup(t *testing.T) {
	t.Parallel()
	s := seed(3, 2)
	b := newFake(100)
	atFail := -1
	b.opt = func(call int, cs []broker.Contract) ([]broker.Quote, error) {
		if call == 2 {
			atFail = s.total()
			return nil, typedErr{contracts.Retryable}
		}
		return okQuotes(b.clk, cs), nil
	}
	j := newPull(t, s, b)
	res := runAll(t, j)
	if atFail != 4 {
		t.Errorf("rows written when the broker failed at the third stock: %d, want the 4 of the first two", atFail)
	}
	if res.Err != nil && res.Err.Kind() != contracts.Retryable {
		t.Errorf("err %v", res.Err)
	}
	if s.total() != 6 || len(s.stored) != 6 {
		t.Fatalf("written %d, stored %d, want one row of each of the 6 items", s.total(), len(s.stored))
	}
	// The same key again writes the rows of the items not final once more: still one row for each item.
	b.opt = nil
	res = runAll(t, j)
	if res.Err != nil || res.Returned != 6 || len(s.stored) != 6 {
		t.Errorf("second run: err %v returned %d stored %d", res.Err, res.Returned, len(s.stored))
	}
	for _, r := range s.stored {
		if r.Status != stOK {
			t.Errorf("row %d status %d after the second run", r.InstrumentID, r.Status)
		}
	}
}

func TestPullResumeAsksOnlyMissingItems(t *testing.T) {
	t.Parallel()
	s := seed(1, 10)
	for id := 1000; id < 1006; id++ {
		_ = s.InsertOptionOpen(context.Background(), []contracts.OptionOpen{storedRow(sessDay, "0945", id, stOK)})
	}
	_ = s.InsertOptionOpen(context.Background(), []contracts.OptionOpen{storedRow(sessDay, "0945", 1006, stNotListed)})
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		for i := range qs {
			qs[i] = broker.Quote{Contract: cs[i], Status: broker.StatusNoData, ReceivedAt: b.clk.Now()}
		}
		return qs, nil
	}
	res := runAll(t, newPull(t, s, b))
	if asked(b) != 3 {
		t.Errorf("asked %d contracts, want the 3 with no final row", asked(b))
	}
	for _, c := range b.optCalls[0] {
		if c.ConID < 10070 {
			t.Errorf("asked a stored item %d", c.ConID)
		}
	}
	if res.Returned != 10 || res.Usable != 6 || res.Err != nil {
		t.Errorf("returned %d usable %d err %v: the 6 stored ok rows count toward the share", res.Returned, res.Usable, res.Err)
	}
}

func TestPullResumeAsksAgainNonFinalStatuses(t *testing.T) {
	t.Parallel()
	s := seed(1, 9)
	for i, st := range []int{stPartial, stNoData, stNoSubscription, stDeadline, stDeferred, stError, stDisconnected} {
		_ = s.InsertOptionOpen(context.Background(), []contracts.OptionOpen{storedRow(sessDay, "0945", 1000+i, st)})
	}
	_ = s.InsertOptionOpen(context.Background(), []contracts.OptionOpen{storedRow(sessDay, "1000", 1007, stOK)})
	_ = s.InsertOptionOpen(context.Background(), []contracts.OptionOpen{storedRow(sessDay.AddDate(0, 0, -1), "0945", 1008, stOK)})
	b := newFake(100)
	if res := runAll(t, newPull(t, s, b)); res.Err != nil {
		t.Fatal(res.Err)
	}
	if asked(b) != 9 {
		t.Errorf("asked %d contracts, want all 9", asked(b))
	}
}

func TestPullFlushErrorIsRetryable(t *testing.T) {
	t.Parallel()
	s := seed(1, 3)
	s.writeErr = errors.New("db down")
	res := runAll(t, newPull(t, s, newFake(100)))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Errorf("err %v, want Retryable", res.Err)
	}
}

func TestPullReadErrorIsRetryable(t *testing.T) {
	t.Parallel()
	s := seed(1, 3)
	s.readErr = errors.New("db down")
	b := newFake(100)
	res := runAll(t, newPull(t, s, b))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(b.optCalls) != 0 {
		t.Errorf("err %v calls %d, want Retryable and nothing asked", res.Err, len(b.optCalls))
	}
}

// writeOnly is a Writer that cannot read.
type writeOnly struct{ s *pullStores }

func (w writeOnly) InsertOptionOpen(ctx context.Context, rows []contracts.OptionOpen) error {
	return w.s.InsertOptionOpen(ctx, rows)
}

func TestNewPullNeedsAReader(t *testing.T) {
	t.Parallel()
	s := seed(1, 1)
	b := newFake(100)
	d := PullDeps{Broker: b, Plans: s, Contracts: s, Writer: writeOnly{s}, Instruments: s, Clock: b.clk}
	if _, err := NewPull(d); err == nil {
		t.Error("a pull that cannot read was built")
	}
	d.Reader = s
	if _, err := NewPull(d); err != nil {
		t.Errorf("a pull with a Reader: %v", err)
	}
}
