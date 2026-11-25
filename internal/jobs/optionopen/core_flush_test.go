package optionopen

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestPullWritesRowsInHandOnCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newFake(100)
	b.opt = func(call int, cs []broker.Contract) ([]broker.Quote, error) {
		if call == 2 { // the second chunk of the second group is cut
			cancel()
			return nil, ctx.Err()
		}
		qs := okQuotes(b.clk, cs)
		for i := range qs {
			qs[i].UndPrice = f(101)
		}
		return qs, nil
	}
	c := newCore(b)
	c.Chunk = 2
	var targets []Target
	for i := range 2 { // group 1: one chunk
		targets = append(targets, target(1, 10+i, ClassHeld, float64(95-i), "P"))
	}
	for i := range 4 { // group 2 (another expiry): two chunks
		t := target(1, 20+i, ClassHeld, float64(95-i), "P")
		t.Expiry = testExpiry.AddDate(0, 0, 7)
		targets = append(targets, t)
	}
	var flushed [][]contracts.OptionOpen
	live := true
	var lastDeadline time.Time
	c.Flush = func(fctx context.Context, rows []contracts.OptionOpen) error {
		live = live && fctx.Err() == nil
		lastDeadline, _ = fctx.Deadline()
		flushed = append(flushed, rows)
		return nil
	}
	res := c.Pull(ctx, targets)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	checkCutRows(t, flushed, res)
	if !live || lastDeadline.IsZero() {
		t.Errorf("a Flush ran with a cancelled context (live %v) or the last had no limit", live)
	}
	last := flushed[len(flushed)-1]
	for _, r := range last {
		if r.Skew || r.UnderlyingPrice == nil || *r.UnderlyingPrice != 100 {
			t.Errorf("row %d of the cut group: skew %v price %v", r.InstrumentID, r.Skew, r.UnderlyingPrice)
		}
	}
}

func TestCutGroupRowsCarryTheStockPrice(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := newFake(339)
	b.opt = func(call int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		for i := range qs {
			qs[i].UndPrice = f(358.32)
		}
		if call == 1 { // the second group is answered, then the context is cancelled
			cancel()
		}
		return qs, nil
	}
	c := newCore(b)
	first := target(1, 10, ClassHeld, 95, "P")
	second := target(1, 20, ClassHeld, 95, "P")
	second.Expiry = testExpiry.AddDate(0, 0, 7)
	res := c.Pull(ctx, []Target{first, second})
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	r := rowOf(t, res, 20)
	if r.Skew || r.UnderlyingPrice == nil || *r.UnderlyingPrice != 339 {
		t.Errorf("cut group row: skew %v price %v, want the stock price 339 and not 358.32", r.Skew, r.UnderlyingPrice)
	}
}

func TestPullWithoutFlushWritesNothingItself(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	res := newCore(b).Pull(context.Background(), []Target{target(1, 10, ClassHeld, 95, "P")})
	if len(res.Rows) != 1 || res.Err != nil {
		t.Errorf("rows %d err %v", len(res.Rows), res.Err)
	}
}

func TestPullFlushErrorStopsTheRun(t *testing.T) {
	t.Parallel()
	b := newFake(100)
	c := newCore(b)
	c.Flush = func(context.Context, []contracts.OptionOpen) error { return errors.New("down") }
	res := c.Pull(context.Background(), []Target{target(1, 10, ClassHeld, 95, "P"), target(2, 20, ClassHeld, 95, "P")})
	if res.Err == nil || len(b.optCalls) != 1 {
		t.Errorf("err %v calls %d: the second stock must not be asked", res.Err, len(b.optCalls))
	}
}

// checkCutRows asserts the rows written: group 1 and the answered chunk of group 2, none with an error code.
func checkCutRows(t *testing.T, flushed [][]contracts.OptionOpen, res Result) {
	t.Helper()
	total := 0
	ids := map[int]bool{}
	for _, rows := range flushed {
		total += len(rows)
		for _, r := range rows {
			ids[r.InstrumentID] = true
			if r.ErrorCode != nil {
				t.Errorf("row %d has the error code %s", r.InstrumentID, *r.ErrorCode)
			}
		}
	}
	if total != 4 || len(res.Rows) != 4 || !ids[10] || !ids[11] || !ids[20] || !ids[21] || ids[22] || ids[23] {
		t.Errorf("written %d rows %v: want group 1 and the answered chunk of group 2 only", total, ids)
	}
}
