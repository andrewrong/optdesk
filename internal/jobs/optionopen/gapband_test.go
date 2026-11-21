package optionopen

import (
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// gapExpiries are three expiries inside the 20 to 45 day window of the plan from the session day.
var gapExpiries = []string{"2026-12-04", "2026-12-11", "2026-12-18"}

// gapID is the instrument id of the put of a strike and an expiry (by its index) of stock 1.
func gapID(expiry, strike int) int { return 1000 + expiry*100 + strike }

// gapStore stores the puts of stock 1 for the strikes lo..hi on the first n expiries; the plan holds the puts for
// the strikes planLo..planHi on the second expiry (the plan of a close of 100).
func gapStore(expiries, lo, hi, planLo, planHi int) *pullStores {
	s := &pullStores{refs: map[int64][]openquote.OptionRef{}}
	s.insts = append(s.insts, store.Instrument{ID: 1, Spec: store.InstrumentSpec{Type: "stock", Name: "S1", Currency: "USD"},
		Identifiers: []store.Identifier{conidOf(1)}})
	for e := 0; e < expiries; e++ {
		for k := hi; k >= lo; k-- {
			id := int64(gapID(e, k))
			s.insts = append(s.insts, store.Instrument{ID: id, Identifiers: []store.Identifier{conidOf(id)}})
			s.refs[1] = append(s.refs[1], openquote.OptionRef{UnderlyingID: 1, Currency: "USD", Exchange: "SMART", TradingClass: "S1",
				Multiplier: "100", Expiry: contracts.Day(gapExpiries[e]), Strike: strconv.Itoa(k), Right: "P", InstrumentID: id})
		}
	}
	rank := 0
	for k := planHi; k >= planLo; k-- {
		rank++
		s.items = append(s.items, contracts.OptionOpenItem{SessionDay: types.SerializableDate{Time: sessDay}, Slot: "0945",
			PlanID: "6ba7b811-9dad-11d1-80b4-00c04fd430c8", InstrumentID: gapID(1, k), UnderlyingID: 1, Class: gen.OptionOpenItemClassPuts,
			Rank: rank, PlannedSpot: 100, Sigma: 0.3, SigmaSource: gen.OptionOpenItemSigmaSourcePriorClose})
	}
	return s
}

// askedIDs are the instrument ids asked, from the con_ids of the calls.
func askedIDs(b *fakeBroker) map[int]bool {
	out := map[int]bool{}
	for _, cs := range b.optCalls {
		for _, c := range cs {
			out[int(c.ConID)/10] = true
		}
	}
	return out
}

func TestGapAsksStoredPutsOfTheLiveBand(t *testing.T) {
	t.Parallel()
	s := gapStore(2, 60, 99, 89, 99) // stored puts down to 60 on two expiries; the plan on a close of 100 holds 89 to 99
	b := newFake(90)
	if res := runAll(t, newPull(t, s, b)); res.Err != nil {
		t.Fatal(res.Err)
	}
	asked := askedIDs(b)
	for _, k := range []int{88, 85, 82, 80} { // inside the live band (floor 79.8), outside the plan
		if !asked[gapID(1, k)] {
			t.Errorf("put %d of the live band, stored and not in the plan, was not asked", k)
		}
	}
	if asked[gapID(1, 70)] {
		t.Error("a put below the live band was asked")
	}
	if asked[gapID(0, 60)] {
		t.Error("a stored put on the first expiry far below the band was asked")
	}
}

func TestGapAddedRowIsInsideAndDroppedPlanItemIsOutside(t *testing.T) {
	t.Parallel()
	s := gapStore(2, 60, 99, 89, 99)
	if res := runAll(t, newPull(t, s, newFake(90))); res.Err != nil {
		t.Fatal(res.Err)
	}
	day := sessDay.Format(time.DateOnly)
	added, ok := s.stored[day+"|0945|"+strconv.Itoa(gapID(1, 85))]
	if !ok || added.OutsideBand || added.Status != stOK {
		t.Errorf("added row %+v (stored %v): want ok, not outside the band", added, ok)
	}
	dropped, ok := s.stored[day+"|0945|"+strconv.Itoa(gapID(1, 99))]
	if !ok || !dropped.OutsideBand || dropped.Status != stDeferred {
		t.Errorf("dropped plan item %+v (stored %v): want deferred, outside the band", dropped, ok)
	}
}

func TestGapAsksOfOneStockNeverExceedTheCap(t *testing.T) {
	t.Parallel()
	s := gapStore(3, 40, 99, 89, 99) // three expiries of 60 puts
	for i := range s.items {
		s.items[i].Sigma = 0.8 // a wide band: 25 puts of each expiry, 75 in all, over the cap of 60
	}
	b := newFake(90)
	if res := runAll(t, newPull(t, s, b)); res.Err != nil {
		t.Fatal(res.Err)
	}
	if n := len(askedIDs(b)); n > DefaultPlanParams().CeilPerStock || n < 50 {
		t.Errorf("asked %d contracts of the stock, want at most %d and a full cap", n, DefaultPlanParams().CeilPerStock)
	}
	inPlan := map[int]bool{}
	for _, it := range s.items {
		inPlan[it.InstrumentID] = true
	}
	over := 0
	for _, r := range s.stored {
		if !inPlan[r.InstrumentID] && r.Status == stDeferred {
			over++
			if r.OutsideBand {
				t.Errorf("added contract %d over the cap is outside the band", r.InstrumentID)
			}
		}
	}
	if over == 0 {
		t.Error("no added contract over the cap was written deferred")
	}
}

func TestGapSpotInsideTheStoredBandAsksThePlan(t *testing.T) {
	t.Parallel()
	s := gapStore(3, 60, 110, 90, 99)
	b := newFake(100)
	// The plan of a close of 100 over the stored chain: the asks of a spot of 100 are its items, none added.
	var chain []Contract
	for _, r := range s.refs[1] {
		ct, _ := refContract(r)
		chain = append(chain, ct)
	}
	n := NameInput{UnderlyingID: 1, Symbol: "S1", Spot: 100, PriorCloseSigma: 0.3, Candidates: chain}
	s.items = nil
	for _, it := range BuildPlan(sessDay, DefaultPlanParams(), []NameInput{n}).Items {
		s.items = append(s.items, contracts.OptionOpenItem{SessionDay: types.SerializableDate{Time: sessDay}, Slot: "0945",
			PlanID: "6ba7b811-9dad-11d1-80b4-00c04fd430c8", InstrumentID: int(it.Contract.InstrumentID), UnderlyingID: 1,
			Class: gen.OptionOpenItemClass(it.Class), Rank: it.Rank, Deferred: it.Deferred, PlannedSpot: 100, Sigma: 0.3,
			SigmaSource: gen.OptionOpenItemSigmaSourcePriorClose})
	}
	want := map[int]bool{}
	for _, it := range s.items {
		if !it.Deferred {
			want[it.InstrumentID] = true
		}
	}
	if len(want) == 0 {
		t.Fatal("empty plan")
	}
	if res := runAll(t, newPull(t, s, b)); res.Err != nil {
		t.Fatal(res.Err)
	}
	got := askedIDs(b)
	if len(got) != len(want) {
		t.Errorf("asked %d contracts, want the %d of the plan", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("plan item %d not asked", id)
		}
	}
}
