package optionopen

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

const pullDay = contracts.Day("2026-11-11")

// pullStores is one fake of the four ports the job reads and writes.
type pullStores struct {
	items    []contracts.OptionOpenItem
	refs     map[int64][]openquote.OptionRef
	insts    []store.Instrument
	written  [][]contracts.OptionOpen
	writeErr error
	planErr  error
}

func (s *pullStores) Plan(_ context.Context, _ contracts.Day, slot string, _ ...int64) ([]contracts.OptionOpenItem, error) {
	if slot != "0945" {
		return nil, nil
	}
	return s.items, s.planErr
}

func (s *pullStores) ListOptions(_ context.Context, u int64) ([]openquote.OptionRef, error) {
	return s.refs[u], nil
}

func (s *pullStores) PutOption(context.Context, openquote.OptionRef) error { return nil }

func (s *pullStores) InsertOptionOpen(_ context.Context, rows []contracts.OptionOpen) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.written = append(s.written, rows)
	return nil
}

func (s *pullStores) Instruments(_ context.Context, ids []int64) ([]store.Instrument, error) {
	var out []store.Instrument
	for _, in := range s.insts {
		for _, id := range ids {
			if in.ID == id {
				out = append(out, in)
			}
		}
	}
	return out, nil
}

func conidOf(id int64) store.Identifier {
	return store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(id*10, 10)}
}

// seed makes stocks 1..n, each with perStock held puts from 95 down, and returns the stores. The stock u is "S<u>", its
// instrument id is u, and its option k has the instrument id u*1000+k.
func seed(stocks, perStock int) *pullStores {
	s := &pullStores{refs: map[int64][]openquote.OptionRef{}}
	for u := 1; u <= stocks; u++ {
		s.insts = append(s.insts, store.Instrument{ID: int64(u), Spec: store.InstrumentSpec{Type: "stock", Name: "S" + strconv.Itoa(u), Currency: "USD"},
			Identifiers: []store.Identifier{conidOf(int64(u))}})
		for k := 0; k < perStock; k++ {
			id := int64(u*1000 + k)
			strike := strconv.Itoa(95 - k)
			right := "P"
			s.insts = append(s.insts, store.Instrument{ID: id, Identifiers: []store.Identifier{conidOf(id)}})
			s.refs[int64(u)] = append(s.refs[int64(u)], openquote.OptionRef{UnderlyingID: int64(u), Currency: "USD", Exchange: "SMART",
				TradingClass: "S" + strconv.Itoa(u), Multiplier: "100", Expiry: "2026-12-11", Strike: strike, Right: right, InstrumentID: id})
			class := gen.OptionOpenItemClassHeld // a held contract is always inside the band
			s.items = append(s.items, contracts.OptionOpenItem{SessionDay: types.SerializableDate{Time: sessDay}, Slot: "0945",
				PlanID: "6ba7b811-9dad-11d1-80b4-00c04fd430c8", InstrumentID: int(id), UnderlyingID: u, Class: class, Rank: k + 1,
				PlannedSpot: 100, Sigma: 0.3, SigmaSource: gen.OptionOpenItemSigmaSourcePriorClose})
		}
	}
	return s
}

func newPull(t *testing.T, s *pullStores, b *fakeBroker) contracts.Job {
	t.Helper()
	j, err := NewPull(PullDeps{Broker: b, Plans: s, Contracts: s, Writer: s, Instruments: s, Clock: b.clk})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func runAll(t *testing.T, j contracts.Job) contracts.JobResult {
	t.Helper()
	keys, _, err := j.Plan(context.Background(), pullDay)
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), pullDay, keys)
}

func TestNewPullNeedsItsPorts(t *testing.T) {
	t.Parallel()
	if _, err := NewPull(PullDeps{}); err == nil {
		t.Error("a job with no ports was built")
	}
}

func TestPullScheduleIsTheOpenPlusFiveMinutes(t *testing.T) {
	t.Parallel()
	s := seed(1, 1)
	j := newPull(t, s, newFake(100))
	sc, ok := j.(interface {
		Schedule() (string, string, string, time.Duration)
		RunBound() time.Duration
	})
	if !ok {
		t.Fatal("the job has no schedule")
	}
	cal, anchor, _, off := sc.Schedule()
	if j.ID() != "option_open" || cal != "XNYS" || anchor != "session_open" || off != 5*time.Minute || sc.RunBound() != 25*time.Minute {
		t.Errorf("id %s slot %s %s %v bound %v", j.ID(), cal, anchor, off, sc.RunBound())
	}
}

func TestPlanCutsKeysAtMostOneHundredTwentyContracts(t *testing.T) {
	t.Parallel()
	s := seed(5, 50) // 250 items: two stocks (100) per key, the fifth alone
	j := newPull(t, s, newFake(100))
	keys, hash, err := j.Plan(context.Background(), pullDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 || hash == "" || keys[0].DatasetID != "obs.option_open" || keys[0].SubjectKey == keys[1].SubjectKey {
		t.Errorf("keys %v hash %q", keys, hash)
	}
	_, hash2, _ := newPull(t, seed(5, 50), newFake(100)).Plan(context.Background(), pullDay)
	if hash != hash2 {
		t.Error("the plan hash of the same plan differs")
	}
	_, hash3, _ := newPull(t, seed(5, 40), newFake(100)).Plan(context.Background(), pullDay)
	if hash == hash3 {
		t.Error("the plan hash does not follow the plan")
	}
}

func TestNoPlanPlansNoKeysAndRunIsNoValue(t *testing.T) {
	t.Parallel()
	s := &pullStores{}
	j := newPull(t, s, newFake(100))
	if keys, hash, err := j.Plan(context.Background(), pullDay); err != nil || len(keys) != 0 || hash != "" {
		t.Fatalf("empty plan: keys %v hash %q err %v", keys, hash, err)
	}
	s.items = seed(2, 3).items
	if keys, _, err := j.Plan(context.Background(), pullDay); err != nil || len(keys) == 0 {
		t.Fatalf("stored plan: keys %v err %v", keys, err)
	}
	s.items = nil
	res := j.Run(context.Background(), pullDay, []contracts.ExpectedKey{{DatasetID: "obs.option_open", SubjectKey: "batch-001"}})
	if res.Err == nil || res.Err.Kind() != contracts.NoValue || res.Err.Error() != "option_open: no plan" {
		t.Errorf("Run: %v", res.Err)
	}
}

func TestRunWritesOneRowPerPlannedItemWithTheContractOfTheStores(t *testing.T) {
	t.Parallel()
	s := seed(2, 8)
	b := newFake(100)
	res := runAll(t, newPull(t, s, b))
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(s.written) != 1 || len(s.written[0]) != 16 || res.Returned != 16 || res.Usable != 16 {
		t.Fatalf("written %d batches, returned %d usable %d", len(s.written), res.Returned, res.Usable)
	}
	seen := map[int]bool{}
	for _, r := range s.written[0] {
		if seen[r.InstrumentID] || r.Slot != "0945" || r.Currency != "USD" || r.Status != stOK {
			t.Errorf("row %d slot %s status %d", r.InstrumentID, r.Slot, r.Status)
		}
		seen[r.InstrumentID] = true
	}
	c := b.optCalls[0][0]
	if c.SecType != "OPT" || c.Symbol != "S1" || c.Expiry != "20261211" || c.Right != "P" || c.Multiplier != "100" ||
		c.TradingClass != "S1" || c.ConID != 10000 || c.Exchange != "SMART" {
		t.Errorf("option contract %+v", c)
	}
}

func TestItemWithoutAStoredContractEndsError(t *testing.T) {
	t.Parallel()
	s := seed(1, 8)
	s.refs[1] = s.refs[1][1:] // the first item has no master.option_contract row
	res := runAll(t, newPull(t, s, newFake(100)))
	rows := s.written[0]
	if len(rows) != 8 {
		t.Fatalf("rows %d, want one per planned item", len(rows))
	}
	var bad []contracts.OptionOpen
	for _, r := range rows {
		if r.Status == stError {
			bad = append(bad, r)
		}
	}
	if len(bad) != 1 || bad[0].InstrumentID != 1000 || code(bad[0]) != "NO_CONTRACT" {
		t.Errorf("error rows %+v", bad)
	}
	if res.Err != nil {
		t.Errorf("7 of 8 ok is above the share: %v", res.Err)
	}
}

func TestPostconditionOkShareOfTheItemsNotDeferred(t *testing.T) {
	t.Parallel()
	s := seed(1, 10)
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		for i := 0; i < 6; i++ { // 4 ok of 10: under 0.5
			qs[i] = broker.Quote{Contract: cs[i], Status: broker.StatusNoData, ReceivedAt: b.clk.Now()}
		}
		return qs, nil
	}
	res := runAll(t, newPull(t, s, b))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || res.Usable != 4 || res.Returned != 10 {
		t.Errorf("err %v usable %d returned %d, want a failed run with the 10 rows written", res.Err, res.Usable, res.Returned)
	}
	if len(s.written) != 1 {
		t.Error("the rows of a run that misses the share are still written")
	}

	// Half of the items deferred by the plan: 5 of the 5 that count are ok.
	s2 := seed(1, 10)
	for i := 5; i < 10; i++ {
		s2.items[i].Deferred = true
	}
	if res := runAll(t, newPull(t, s2, newFake(100))); res.Err != nil {
		t.Errorf("deferred items do not count against the share: %v", res.Err)
	}
}

func TestNoNewKeyAfterTheStopTime(t *testing.T) {
	t.Parallel()
	s := seed(1, 3)
	b := newFake(100)
	b.clk.Advance(15 * time.Minute) // 15:00Z is 10:00 ET
	res := runAll(t, newPull(t, s, b))
	if res.Err == nil || len(s.written) != 0 || len(b.optCalls) != 0 {
		t.Errorf("err %v written %d calls %d; a key started after 10:00 ET", res.Err, len(s.written), len(b.optCalls))
	}
}

func TestStopTimeBetweenKeys(t *testing.T) {
	t.Parallel()
	s := seed(2, 60) // two keys
	b := newFake(100)
	b.step = 20 * time.Minute // the first key ends after 10:00 ET
	res := runAll(t, newPull(t, s, b))
	if len(s.written) != 1 || res.Err == nil {
		t.Errorf("written %d err %v; want the first key written and the second not started", len(s.written), res.Err)
	}
}

func TestWrongDayAndWriteFailure(t *testing.T) {
	t.Parallel()
	s := seed(1, 3)
	j := newPull(t, s, newFake(100))
	keys, _, _ := j.Plan(context.Background(), pullDay)
	if res := j.Run(context.Background(), "2026-11-10", keys); res.Err == nil || res.Err.Kind() != contracts.NoValue {
		t.Errorf("wrong day: %v", res.Err)
	}
	s.writeErr = errors.New("db down")
	if res := j.Run(context.Background(), pullDay, keys); res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Errorf("write failure: %v", res.Err)
	}
	if res := j.Run(context.Background(), pullDay, []contracts.ExpectedKey{{SubjectKey: "nope"}}); res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Errorf("unknown key: %v", res.Err)
	}
}

func TestRunWithOnlyNonLiveQuotesFails(t *testing.T) {
	t.Parallel()
	s := seed(1, 10)
	b := newFake(100)
	b.opt = func(_ int, cs []broker.Contract) ([]broker.Quote, error) {
		qs := okQuotes(b.clk, cs)
		for i := range qs {
			qs[i].MarketDataType = 2
		}
		return qs, nil
	}
	res := runAll(t, newPull(t, s, b))
	if res.Err == nil || res.Err.Kind() != contracts.Permanent || res.Usable != 0 {
		t.Errorf("err %v usable %d, want a Permanent failure with no usable row", res.Err, res.Usable)
	}
}

func TestPlanReturnsTheRetryableErrorOfTheStore(t *testing.T) {
	t.Parallel()
	s := seed(1, 1)
	s.planErr = errors.New("db down")
	_, _, err := newPull(t, s, newFake(100)).Plan(context.Background(), pullDay)
	if err == nil || err.(contracts.TypedError).Kind() != contracts.Retryable {
		t.Fatalf("Plan err %v", err)
	}
}
