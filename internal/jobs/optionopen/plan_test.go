package optionopen

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/broker"
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
	return r.jobWith(t, PlanDeps{}, symbols...)
}

// jobWith builds the plan job with the rig's ports and the optional ports of d.
func (r *planRig) jobWith(t *testing.T, d PlanDeps, symbols ...string) contracts.Job {
	t.Helper()
	d.Stored = Stored{Watch: watchOf(symbols...), Instruments: r.inst, Bars: r.bars, Options: r.opts, Accounts: r.acct, Eod: r.eod}
	d.Marks, d.Plans, d.Clock = r.marks, r.plans, r.clk
	j, err := NewPlan(d)
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

func TestPlanKeyIsOneFixedKeyAndRunPlansEveryStock(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.acct = fakeAccounts{pos: []store.Position{{SecType: "STK", Symbol: "NVDA", ConID: 77, Exchange: "SMART", Currency: "USD", Quantity: "10"}}}
	a := r.seed(t, 100, putStrikes...)
	b := r.seed(t, 101, putStrikes...)
	j := r.job(t, "MSFT", "AAPL")
	keys, hash, err := j.Plan(context.Background(), runDay)
	if err != nil || hash == "" || len(keys) != 1 || keys[0].DatasetID != PlanDatasetID || keys[0].SubjectKey != "us" {
		t.Fatalf("keys %v hash %q err %v", keys, hash, err)
	}
	if res := j.Run(context.Background(), runDay, keys); res.Err != nil {
		t.Fatal(res.Err)
	}
	seen := map[int]bool{}
	for _, it := range r.plans.items {
		seen[it.UnderlyingID] = true
	}
	if !seen[int(a)] || !seen[int(b)] {
		t.Fatalf("items of %v, want both stocks", seen)
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

func TestPlanBudgetEndsAsAReasonRowForTheNameNotReached(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	r.seed(t, 101, putStrikes...)
	rs := &fakeReasons{}
	r.inst.adv = func() { r.clk.Advance(41 * time.Minute) }
	j := r.jobWith(t, PlanDeps{Reasons: rs}, "AAPL", "MSFT")
	res := r.run(t, j)
	if res.Err != nil {
		t.Fatalf("one name stored: no error, got %v", res.Err)
	}
	got := rs.of(runDay, PlanDatasetID, "us")
	if len(got) != 1 || got[0].Scope != "MSFT" || got[0].Code != "fetch_failed" {
		t.Fatalf("reasons %+v", got)
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

func TestEmptyPlanStoresReasonCode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		code string
	}{{"no_con_id", "no_con_id"}, {"no_close", "no_close"}, {"no_chain", "no_chain"}} {
		r := newPlanRig()
		r.seed(t, 100, putStrikes...)
		rs := &fakeReasons{}
		j := r.jobWith(t, PlanDeps{Reasons: rs, History: &fakeHistory{}}, "AAPL")
		pj := j.(*planJob)
		switch tc.name {
		case "no_con_id":
			pj.d.Watch = watchOf("AAPL")
			pj.d.Watch[0].Contract.ConID = 0
		case "no_close":
			pj.d.Bars = fakeBars{}
		case "no_chain":
			pj.d.Options = &fakeOptions{}
		}
		res := r.run(t, j)
		if res.Err == nil || res.Err.Kind() != contracts.Permanent || res.Returned != 0 {
			t.Fatalf("%s: err=%v returned=%d", tc.name, res.Err, res.Returned)
		}
		got := rs.of(runDay, PlanDatasetID, "us")
		if len(got) != 1 || got[0].Code != tc.code || got[0].Scope != "AAPL" || got[0].Dataset != PlanDatasetID || got[0].Subject != "us" {
			t.Fatalf("%s: reasons %+v", tc.name, got)
		}
	}
}

func TestOnlyFetchFailedIsRetryable(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	var logs []string
	rs := &fakeReasons{}
	h := &fakeHistory{err: errors.New("gateway down")}
	j := r.jobWith(t, PlanDeps{Reasons: rs, History: h, Log: func(l string) { logs = append(logs, l) }}, "AAPL")
	j.(*planJob).d.Bars = fakeBars{}
	res := r.run(t, j)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || h.asked != 1 {
		t.Fatalf("err=%v asked=%d", res.Err, h.asked)
	}
	if got := rs.of(runDay, PlanDatasetID, "us"); len(got) != 1 || got[0].Code != "fetch_failed" {
		t.Fatalf("reasons %+v", got)
	}
	if len(logs) != 1 || !strings.HasPrefix(logs[0], "fault stage=option_open_plan day=2026-11-10 key=us reason=") {
		t.Fatalf("logs %q", logs)
	}
	// A good fetch with no bar is no_close, Permanent; a good fetch with a bar plans the name.
	h.err, h.bars = nil, nil
	if res := r.run(t, j); res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("empty fetch: %v", res.Err)
	}
	h.bars = []broker.Bar{{Close: "100"}}
	if res := r.run(t, j); res.Err != nil || res.Returned == 0 {
		t.Fatalf("fetched close: %v", res.Err)
	}
}

func TestListedZeroGroupIsNoValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		rows func(id int64) fakeListRows
		kind contracts.ErrorKind
	}{
		{"zero group", func(id int64) fakeListRows {
			return fakeListRows{groups: map[int64][]openquote.ListRecord{id: {{UnderlyingID: id, ListedSession: runDay, Outcome: "listed", Count: 0}}}}
		}, contracts.NoValue},
		{"no_expiry_in_window", func(id int64) fakeListRows {
			return fakeListRows{runs: []openquote.ListRunRecord{{UnderlyingID: id, Session: runDay, Reason: "no_expiry_in_window"}}}
		}, contracts.NoValue},
		{"stale zero group", func(id int64) fakeListRows {
			return fakeListRows{groups: map[int64][]openquote.ListRecord{id: {{UnderlyingID: id, ListedSession: "2026-11-01", Outcome: "listed"}}}}
		}, contracts.Permanent},
	} {
		r := newPlanRig()
		id, _ := r.inst.ResolveInstrument(context.Background(), store.Identifier{Scheme: "ibkr_conid", Value: "100"}, store.InstrumentSpec{}, nil)
		rs := &fakeReasons{}
		j := r.jobWith(t, PlanDeps{Reasons: rs, ListRows: tc.rows(id)}, "AAPL")
		res := r.run(t, j)
		if res.Err == nil || res.Err.Kind() != tc.kind {
			t.Fatalf("%s: %v", tc.name, res.Err)
		}
		got := rs.of(runDay, PlanDatasetID, "us")
		if tc.kind == contracts.NoValue && len(got) != 0 {
			t.Fatalf("%s: a name with no options writes a reason: %+v", tc.name, got)
		}
		if tc.kind == contracts.Permanent && (len(got) != 1 || got[0].Code != "no_chain") {
			t.Fatalf("%s: %+v", tc.name, got)
		}
	}
}

func TestMixedRunStoresItemsAndReasonsForFailedNames(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	var symbols []string
	for i := 0; i < 44; i++ {
		symbols = append(symbols, fmt.Sprintf("S%02d", i))
		r.seed(t, int64(100+i), putStrikes...)
	}
	symbols = append(symbols, "ZZZ") // conid 144, no stored contract
	rs := &fakeReasons{}
	j := r.jobWith(t, PlanDeps{Reasons: rs}, symbols...)
	res := r.run(t, j)
	if res.Err != nil || res.Returned == 0 {
		t.Fatalf("err=%v returned=%d", res.Err, res.Returned)
	}
	got := rs.of(runDay, PlanDatasetID, "us")
	if len(got) != 1 || got[0].Scope != "ZZZ" || got[0].Code != "no_chain" {
		t.Fatalf("reasons %+v", got)
	}
	// A clean run clears the reason.
	r.seed(t, 144, putStrikes...)
	if res := r.run(t, j); res.Err != nil || len(rs.of(runDay, PlanDatasetID, "us")) != 0 {
		t.Fatalf("err=%v reasons %+v", res.Err, rs.of(runDay, PlanDatasetID, "us"))
	}
}

func TestFetchFailedBecomesPermanentAfterFourRuns(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.seed(t, 100, putStrikes...)
	rs := &fakeReasons{}
	j := r.jobWith(t, PlanDeps{Reasons: rs, History: &fakeHistory{err: errors.New("no answer")}}, "AAPL")
	j.(*planJob).d.Bars = fakeBars{}
	for i := 1; i <= maxFetchFailedRuns; i++ {
		if res := r.run(t, j); res.Err == nil || res.Err.Kind() != contracts.Retryable {
			t.Fatalf("run %d: %v", i, res.Err)
		}
	}
	res := r.run(t, j)
	if res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("run 5: %v", res.Err)
	}
	if got := rs.of(runDay, PlanDatasetID, "us"); len(got) != 1 || got[0].Code != "fetch_failed" {
		t.Fatalf("the reason stays fetch_failed: %+v", got)
	}
	// Another day starts the count again.
	r.clk.Advance(24 * time.Hour)
	keys, _, _ := j.Plan(context.Background(), "2026-11-11")
	if res := j.Run(context.Background(), "2026-11-11", keys); res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("next day: %v", res.Err)
	}
}

func TestPlanWindowsPerCalendar(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	us := r.job(t, "AAPL").(interface {
		Window() (string, time.Duration, time.Duration)
	})
	_, jp := jpPlanRig(t, planManifestJP)
	jpw := jp.(interface {
		Window() (string, time.Duration, time.Duration)
	})
	if c, _, _ := us.Window(); c != "XNYS" {
		t.Errorf("US plan window %s", c)
	}
	if c, _, _ := jpw.Window(); c != "XTKS" {
		t.Errorf("JP plan window %s", c)
	}
	rr := newRig(oneChain())
	usR, _ := NewResolve(rr.deps("AAPL"), ResolveOptions{})
	jpR, _ := NewResolveJP(rr.deps("AAPL"), ResolveOptions{})
	if c, _, _ := usR.(interface {
		Window() (string, time.Duration, time.Duration)
	}).Window(); c != "XNYS" {
		t.Errorf("US resolve window %s", c)
	}
	if c, _, _ := jpR.(interface {
		Window() (string, time.Duration, time.Duration)
	}).Window(); c != "XTKS" {
		t.Errorf("JP resolve window %s", c)
	}
}
