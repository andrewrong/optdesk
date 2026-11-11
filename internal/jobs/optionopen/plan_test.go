package optionopen

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

type planRig struct {
	clk   *fakeClock
	opts  *fakeOptions
	marks *fakeMarks
	inst  *fakeInstruments
	plans *fakePlans
	acct  fakeAccounts
	eod   fakeEod
	bars  fakeBars
}

type fakeEod struct{ rows []contracts.OptionEod }

func (f fakeEod) OptionEod(context.Context, int64, contracts.Day, contracts.Day) ([]contracts.OptionEod, error) {
	return f.rows, nil
}

func newPlanRig() *planRig {
	return &planRig{
		clk:   &fakeClock{t: time.Date(2026, 11, 10, 13, 45, 0, 0, time.UTC)},
		opts:  &fakeOptions{},
		marks: &fakeMarks{},
		inst:  &fakeInstruments{},
		plans: &fakePlans{},
		bars:  fakeBars{closes: map[string]string{"2026-11-09": "100"}},
	}
}

// seed stores puts of the stock (watch conid 100+i is instrument i+1 once seeded) 30 days out at the strikes.
func (r *planRig) seed(t *testing.T, conID int64, strikes ...string) int64 {
	t.Helper()
	id, _ := r.inst.ResolveInstrument(context.Background(),
		store.Identifier{Scheme: "ibkr_conid", Value: strconv.FormatInt(conID, 10)}, store.InstrumentSpec{}, nil)
	for i, s := range strikes {
		_ = r.opts.PutOption(context.Background(), openquote.OptionRef{UnderlyingID: id, Currency: "USD", Exchange: "SMART", TradingClass: "X",
			Multiplier: "100", Expiry: expiry30Day, Strike: s, Right: "P", InstrumentID: id*10000 + int64(i)})
	}
	return id
}

func (r *planRig) job(t *testing.T, symbols ...string) contracts.Job {
	t.Helper()
	j, err := NewPlan(PlanDeps{
		Stored: Stored{Watch: watchOf(symbols...), Instruments: r.inst, Bars: r.bars, Options: r.opts, Accounts: r.acct, Eod: r.eod},
		Marks:  r.marks, Plans: r.plans, Clock: r.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (r *planRig) run(t *testing.T, j contracts.Job) contracts.JobResult {
	t.Helper()
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), runDay, keys)
}

var putStrikes = []string{"88.0", "90.0", "92.0", "94.0", "96.0", "98.0", "99.0", "100.0", "101.0"}

func TestPlanWritesTheItemsOfStoredContractsOnly(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	res := r.run(t, r.job(t, "AAPL"))
	if res.Err != nil || len(r.plans.items) == 0 {
		t.Fatalf("err=%v items=%d", res.Err, len(r.plans.items))
	}
	for _, it := range r.plans.items {
		if it.Slot != "0945" || it.PlannedSpot != 100 || it.SessionDay.Format(time.DateOnly) != "2026-11-10" || it.PlanID == "" ||
			it.SigmaSource != "default" || it.Sigma != DefaultSigma || !it.PlannedAt.Equal(r.clk.Now()) {
			t.Fatalf("item: %+v", it)
		}
	}
	if res.Returned != len(r.plans.items) {
		t.Fatalf("returned %d of %d", res.Returned, len(r.plans.items))
	}
}

func TestPlanLeavesOutAMarkedContract(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	id := r.seed(t, 100, putStrikes...)
	r.marks.marks = []contracts.OptionNotListed{{UnderlyingID: int(id), Expiry: types.SerializableDate{Time: mustDay(expiry30Day)}, Strike: "99.0",
		Right: "P", TradingClass: "X", RetryAt: r.clk.Now().Add(time.Hour)}}
	r.run(t, r.job(t, "AAPL"))
	for _, it := range r.plans.items {
		if it.InstrumentID == int(id*10000+6) {
			t.Fatal("the marked strike is in the plan")
		}
	}
	if len(r.plans.items) == 0 {
		t.Fatal("the other strikes are planned")
	}
}

func TestPlanOfAStockWithNoStoredChainIsNoValue(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	j := r.job(t, "AAPL", "MSFT")
	res := r.run(t, j)
	if res.Err == nil || res.Err.Kind() != contracts.NoValue || res.Err.Error() != "option_open: no resolved chain of MSFT" {
		t.Fatalf("err=%v", res.Err)
	}
	if len(r.plans.items) == 0 {
		t.Fatal("the stock that has a chain is still planned")
	}
}

func TestPlanKeysAreOneStockEachAndIncludeHeldStocks(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.acct = fakeAccounts{pos: []store.Position{{SecType: "STK", Symbol: "NVDA", ConID: 77, Exchange: "SMART", Currency: "USD", Quantity: "10"}}}
	keys, hash, err := r.job(t, "MSFT", "AAPL").Plan(context.Background(), runDay)
	if err != nil || hash == "" {
		t.Fatal(err)
	}
	var got []string
	for _, k := range keys {
		if k.DatasetID != PlanDatasetID {
			t.Fatalf("dataset %s", k.DatasetID)
		}
		got = append(got, k.SubjectKey)
	}
	if len(got) != 3 || got[0] != "AAPL" || got[1] != "MSFT" || got[2] != "NVDA" {
		t.Fatalf("keys %v", got)
	}
}

func TestPlanIsTheSameOnASecondRun(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	j := r.job(t, "AAPL")
	r.run(t, j)
	first := append([]contracts.OptionOpenItem(nil), r.plans.items...)
	r.plans.items = nil
	r.run(t, j)
	if len(first) != len(r.plans.items) {
		t.Fatalf("%d then %d items", len(first), len(r.plans.items))
	}
	for i := range first {
		if first[i] != r.plans.items[i] {
			t.Fatalf("item %d differs: %+v %+v", i, first[i], r.plans.items[i])
		}
	}
}

func TestPlanWritesOnlyTheItemsOfItsKeys(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	a := r.seed(t, 100, putStrikes...)
	r.seed(t, 101, putStrikes...)
	j := r.job(t, "AAPL", "MSFT")
	res := j.Run(context.Background(), runDay, []contracts.ExpectedKey{{DatasetID: PlanDatasetID, SubjectKey: "AAPL"}})
	if res.Err != nil || len(r.plans.items) == 0 {
		t.Fatalf("err=%v", res.Err)
	}
	for _, it := range r.plans.items {
		if int64(it.UnderlyingID) != a {
			t.Fatalf("item of another stock: %+v", it)
		}
	}
}

func TestPlanBudgetEndsAsAGapAndNotAThinnerPlan(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	r.seed(t, 101, putStrikes...)
	r.inst.adv = func() { r.clk.Advance(41 * time.Minute) }
	res := r.run(t, r.job(t, "AAPL", "MSFT"))
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("a stock not finished in the budget is a retryable gap, got %v", res.Err)
	}
}

func TestPlanUsesSigmaOfThePriorCloseWhenOptionEodHasIt(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	id := r.seed(t, 100, putStrikes...)
	iv := 0.42
	r.eod = fakeEod{rows: []contracts.OptionEod{{InstrumentID: int(id*10000 + 7), IvDecimal: &iv,
		SessionDay: types.SerializableDate{Time: mustDay("2026-11-09")}}}}
	r.run(t, r.job(t, "AAPL"))
	if got := r.plans.items[0]; got.Sigma != 0.42 || got.SigmaSource != "prior_close" {
		t.Fatalf("sigma %v %s", got.Sigma, got.SigmaSource)
	}
}

func TestPlanHeldContractIsAlwaysPlannedAsHeld(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	id := r.seed(t, 100, putStrikes...)
	// A far put the band does not hold, held.
	_ = r.opts.PutOption(context.Background(), openquote.OptionRef{UnderlyingID: id, Currency: "USD", Exchange: "SMART", TradingClass: "X",
		Multiplier: "100", Expiry: expiry30Day, Strike: "60.0", Right: "P", InstrumentID: 999})
	r.acct = fakeAccounts{pos: []store.Position{{SecType: "OPT", Symbol: "AAPL", Expiry: "2026-12-10", Strike: "60.0", Right: "put", Quantity: "-1"}}}
	r.run(t, r.job(t, "AAPL"))
	for _, it := range r.plans.items {
		if it.InstrumentID == 999 && it.Class == "held" && !it.Deferred {
			return
		}
	}
	t.Fatal("the held contract is not in the plan as held")
}

func TestPlanSlotIsFortyFiveMinutesBeforeTheOpen(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	j := r.job(t, "AAPL")
	cal, anchor, _, off := j.(interface {
		Schedule() (string, string, string, time.Duration)
	}).Schedule()
	if j.ID() != "option_open_plan" || cal != "XNYS" || anchor != "session_open" || off != -45*time.Minute {
		t.Fatalf("%s %s %s %v", j.ID(), cal, anchor, off)
	}
}
