package optionopen

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

func jpWatch(codes ...string) []jobs.WatchEntry {
	var out []jobs.WatchEntry
	for i, c := range codes {
		out = append(out, jobs.WatchEntry{Market: "JP", Contract: broker.Contract{SecType: "STK", Symbol: c, Exchange: "TSEJ",
			Currency: "JPY", ConID: int64(100 + i)}})
	}
	return out
}

func jpPlanRig(t *testing.T, manifest []byte, codes ...string) (*planRig, contracts.Job) {
	t.Helper()
	r := newPlanRig()
	r.clk.t = time.Date(2026, 11, 9, 23, 15, 0, 0, time.UTC) // 08:15 JST of 2026-11-10
	for i := range codes {
		r.seed(t, int64(100+i), putStrikes...)
	}
	j, err := newPlanJP(manifest, PlanDeps{
		Stored: Stored{Watch: append(jpWatch(codes...), watchOf("AAPL")...), Instruments: r.inst, Bars: r.bars, Options: r.opts,
			Accounts: r.acct, Eod: r.eod},
		Marks: r.marks, Plans: r.plans, Clock: r.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	return r, j
}

func TestPlanJPSchedulesAtTheOpenMinus45MinutesOfXTKS(t *testing.T) {
	t.Parallel()
	_, j := jpPlanRig(t, planManifestJP)
	sc := j.(interface {
		Schedule() (string, string, string, time.Duration)
		Window() (string, time.Duration, time.Duration)
	})
	cal, anchor, _, off := sc.Schedule()
	wcal, _, _ := sc.Window()
	if j.ID() != "option_open_plan_jp" || cal != "XTKS" || anchor != "session_open" || off != -45*time.Minute || wcal != "XTKS" {
		t.Errorf("id %s slot %s %s %v window %s", j.ID(), cal, anchor, off, wcal)
	}
}

func TestPlanJPPlansTheJPNamesOnlyWithDTE14To90UnderSlot0905(t *testing.T) {
	t.Parallel()
	r, j := jpPlanRig(t, planManifestJP, "7203")
	// A put 70 days out: inside the JP range, outside the US one.
	_ = r.opts.PutOption(context.Background(), openquote.OptionRef{UnderlyingID: 1, Currency: "JPY", Exchange: "OSE.JPN", TradingClass: "7203",
		Multiplier: "100", Expiry: "2027-01-19", Strike: "100.0", Right: "P", InstrumentID: 77})
	res := r.run(t, j)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	far := false
	for _, it := range r.plans.items {
		if it.Slot != "0905" || it.UnderlyingID != 1 || it.SessionDay.Format(time.DateOnly) != "2026-11-10" {
			t.Fatalf("item %+v", it)
		}
		far = far || it.InstrumentID == 77
	}
	if !far || len(r.plans.items) == 0 {
		t.Errorf("the put 70 days out is not planned: %d items", len(r.plans.items))
	}
}

func TestPlanJPNoOptionNameGetsNoOptionContract(t *testing.T) {
	t.Parallel()
	custom := []byte(strings.Replace(string(planManifestJP), "no_option = []", `no_option = ["7203"]`, 1))
	r, j := jpPlanRig(t, custom, "7203", "6758")
	if res := r.run(t, j); res.Err != nil {
		t.Fatal(res.Err)
	}
	if len(r.plans.items) == 0 {
		t.Fatal("the other name is planned")
	}
	for _, it := range r.plans.items {
		if it.UnderlyingID == 1 {
			t.Fatalf("a no_option name has the item %+v", it)
		}
	}
}

func TestJPPlanFuncKeepsANoOptionNameAsUnderlyingOnly(t *testing.T) {
	t.Parallel()
	rules, err := parseJPRules([]byte(strings.Replace(string(planManifestJP), "no_option = []", `no_option = ["7203"]`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	cand := []Contract{{InstrumentID: 5, Expiry: day.AddDate(0, 0, 30), Strike: 99, Right: Put}}
	plan := rules.planFunc(BuildPlan)(day, JPPlanParams(), []NameInput{
		{UnderlyingID: 1, Symbol: "7203", Spot: 100, Candidates: cand, Held: cand},
		{UnderlyingID: 2, Symbol: "6758", Spot: 100, Candidates: cand},
	})
	if len(plan.Names) != 2 {
		t.Fatalf("names %+v", plan.Names)
	}
	for _, it := range plan.Items {
		if it.Symbol == "7203" {
			t.Errorf("option item of a no_option name: %+v", it)
		}
	}
	if len(plan.Items) == 0 || plan.Items[0].Rank != 1 {
		t.Errorf("items %+v", plan.Items)
	}
}

func TestJPPlanFuncCapsAnIndexAt80AndAStockAt60(t *testing.T) {
	t.Parallel()
	rules, err := parseJPRules(planManifestJP)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	var cand []Contract
	for k := 0; k < 120; k++ {
		cand = append(cand, Contract{InstrumentID: int64(1000 + k), Expiry: day.AddDate(0, 0, 30), Strike: float64(1 + k), Right: Put})
	}
	p := JPPlanParams()
	p.MaxPerExpiry, p.CeilAroundSpot, p.CeilPuts, p.CeilCalls, p.StoreK, p.PutK, p.FarOTM = 200, 1000, 1000, 1000, 100, 100, 1
	plan := rules.planFunc(BuildPlan)(day, p, []NameInput{
		{UnderlyingID: 1, Symbol: "7203", Spot: 100, Candidates: cand},
		{UnderlyingID: 2, Symbol: "N225", Spot: 100, Candidates: cand},
	})
	live := map[string]int{}
	for _, it := range plan.Items {
		if !it.Deferred {
			live[it.Symbol]++
		}
	}
	if live["7203"] != 60 || live["N225"] != 80 {
		t.Errorf("live items per name: %v", live)
	}
}

func TestJPBrokerKeepsTheChainOfOSEOfTheStockCode(t *testing.T) {
	t.Parallel()
	rules, err := parseJPRules(planManifestJP)
	if err != nil {
		t.Fatal(err)
	}
	fb := &fakeResolver{chains: []broker.Chain{
		{Exchange: "SMART", TradingClass: "7203", Multiplier: "100"},
		{Exchange: "OSE.JPN", TradingClass: "7203", Multiplier: "10"},
		{Exchange: "OSE.JPN", TradingClass: "7203", Multiplier: "100"},
		{Exchange: "OSE.JPN", TradingClass: "225", Multiplier: "100"},
	}}
	b := jpBroker{fb, rules}
	got, _ := b.ListChains(context.Background(), broker.Contract{Symbol: "7203"})
	if len(got) != 1 || got[0].Exchange != "OSE.JPN" || got[0].Multiplier != "100" || got[0].TradingClass != "7203" {
		t.Errorf("stock chains %+v", got)
	}
	got, _ = b.ListChains(context.Background(), broker.Contract{Symbol: "N225"})
	if len(got) != 1 || got[0].TradingClass != "225" {
		t.Errorf("index chains %+v", got)
	}
}

func TestJPAccountsShowOnlyJPYNamesWithOptions(t *testing.T) {
	t.Parallel()
	rules, _ := parseJPRules([]byte(strings.Replace(string(planManifestJP), "no_option = []", `no_option = ["7203"]`, 1)))
	acc := jpAccounts{fakeAccounts{pos: []store.Position{
		{Alias: "A", Symbol: "AAPL", Currency: "USD"}, {Alias: "A", Symbol: "7203", Currency: "JPY"}, {Alias: "A", Symbol: "6758", Currency: "JPY"},
	}}, rules}
	rows, err := acc.Positions(context.Background(), "A", "2026-11-10")
	if err != nil || len(rows) != 1 || rows[0].Symbol != "6758" {
		t.Errorf("rows %+v err %v", rows, err)
	}
}

func TestNewResolveJPSlotAndPorts(t *testing.T) {
	t.Parallel()
	if _, err := NewResolveJP(ResolveDeps{}, ResolveOptions{}); err == nil {
		t.Error("a job with no ports was built")
	}
	r := newPlanRig()
	j, err := NewResolveJP(ResolveDeps{
		Stored: Stored{Instruments: r.inst, Bars: r.bars, Options: r.opts}, Broker: &fakeResolver{}, Marks: r.marks, Clock: r.clk,
	}, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cal, anchor, _, off := j.(interface {
		Schedule() (string, string, string, time.Duration)
	}).Schedule()
	if j.ID() != "option_open_resolve_jp" || cal != "XTKS" || anchor != "session_close" || off != 60*time.Minute {
		t.Errorf("id %s slot %s %s %v", j.ID(), cal, anchor, off)
	}
}

// jpPlans serves the plan under the slot 0905.
type jpPlans struct{ *pullStores }

func (p jpPlans) Plan(ctx context.Context, d contracts.Day, _ string, ids ...int64) ([]contracts.OptionOpenItem, error) {
	return p.pullStores.Plan(ctx, d, "0945", ids...)
}

func newPullJP(t *testing.T, s *pullStores, clk clockwork.Clock) contracts.Job {
	t.Helper()
	j, err := NewPullJP(PullDeps{Broker: &fakeBroker{clk: clk.(*clockwork.FakeClock), spots: []float64{100}}, Plans: jpPlans{s}, Contracts: s, Writer: s,
		Instruments: s, Clock: clk})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestPullJPScheduleIsTheOpenPlusFiveMinutes(t *testing.T) {
	t.Parallel()
	j := newPullJP(t, seed(1, 1), clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC)))
	sc := j.(interface {
		Schedule() (string, string, string, time.Duration)
		RunBound() time.Duration
	})
	cal, anchor, _, off := sc.Schedule()
	if j.ID() != "option_open_jp" || cal != "XTKS" || anchor != "session_open" || off != 5*time.Minute || sc.RunBound() != 20*time.Minute {
		t.Errorf("id %s slot %s %s %v bound %v", j.ID(), cal, anchor, off, sc.RunBound())
	}
}

func TestPullJPRunsOnTheTokyoDateNotTheNewYorkDate(t *testing.T) {
	t.Parallel()
	// 00:05Z on 2026-11-11 is 09:05 JST of the 11th and 19:05 EST of the 10th.
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	s := seed(1, 2)
	j := newPullJP(t, s, clk)
	keys, _, err := j.Plan(context.Background(), "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), "2026-11-11", keys); res.Err != nil || res.Returned != 2 {
		t.Fatalf("Tokyo date: err=%v returned %d", res.Err, res.Returned)
	}
	if len(s.written) != 1 || s.written[0][0].Slot != "0905" {
		t.Errorf("written %+v", s.written)
	}
	if res := j.Run(context.Background(), "2026-11-10", keys); res.Err == nil || res.Err.Kind() != contracts.NoValue {
		t.Errorf("the New York date ran: %v", res.Err)
	}
}

func TestJPLunchBreakIsTheManifestValue(t *testing.T) {
	t.Parallel()
	if got := JPLunchBreak(); got != [2]string{"1130", "1230"} {
		t.Errorf("lunch break %v", got)
	}
}

func TestPullJPQuotesAPlannedContractOf60DaysToExpiry(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	s := seed(1, 1)
	s.items[0].Class = "puts"
	s.refs[1][0].Expiry = "2027-01-10" // 60 days after the session day
	s.refs[1][0].Strike = "95"
	j := newPullJP(t, s, clk)
	keys, _, err := j.Plan(context.Background(), "2026-11-11")
	if err != nil {
		t.Fatal(err)
	}
	if res := j.Run(context.Background(), "2026-11-11", keys); res.Err != nil {
		t.Fatal(res.Err)
	}
	row := s.written[0][0]
	if row.Status != stOK || row.OutsideBand {
		t.Fatalf("got status %d outside_band %v, want ok inside the JP band 14-90", row.Status, row.OutsideBand)
	}
}

func TestPlanJPPlansTheIndexOfTheWatchList(t *testing.T) {
	t.Parallel()
	r := newPlanRig()
	r.clk.t = time.Date(2026, 11, 9, 23, 15, 0, 0, time.UTC)
	r.seed(t, 100, putStrikes...)
	j, err := NewPlanJP(PlanDeps{
		Stored: Stored{Watch: []jobs.WatchEntry{{Market: "JP", Contract: broker.Contract{SecType: "IND", Symbol: "N225", Exchange: "OSE.JPN",
			Currency: "JPY", ConID: 100}}}, Instruments: r.inst, Bars: r.bars, Options: r.opts, Accounts: r.acct, Eod: r.eod},
		Marks: r.marks, Plans: r.plans, Clock: r.clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res := r.run(t, j); res.Err != nil || len(r.plans.items) == 0 {
		t.Fatalf("err=%v items=%d", res.Err, len(r.plans.items))
	}
}

func TestResolveJPAsksTheChainsOfTheIndexAsAnIndex(t *testing.T) {
	t.Parallel()
	rules, _ := parseJPRules(planManifestJP)
	var got broker.Contract
	fb := &spyChains{fakeResolver: &fakeResolver{}, got: &got}
	_, _ = jpBroker{fb, rules}.ListChains(context.Background(), broker.Contract{SecType: "STK", Symbol: "N225", Exchange: "TSEJ"})
	if got.SecType != "IND" || got.Exchange != "OSE.JPN" {
		t.Errorf("asked %+v", got)
	}
}

type spyChains struct {
	*fakeResolver
	got *broker.Contract
}

func (s *spyChains) ListChains(ctx context.Context, u broker.Contract) ([]broker.Chain, error) {
	*s.got = u
	return s.fakeResolver.ListChains(ctx, u)
}

func TestPullJPPlanWaitsForTheStoredPlan(t *testing.T) {
	t.Parallel()
	s := &pullStores{}
	j := newPullJP(t, s, clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC)))
	if keys, _, err := j.Plan(context.Background(), pullDay); err != nil || len(keys) != 0 {
		t.Fatalf("empty plan: keys %v err %v", keys, err)
	}
	s.items = seed(2, 3).items
	if keys, _, err := j.Plan(context.Background(), pullDay); err != nil || len(keys) == 0 {
		t.Fatalf("stored plan: keys %v err %v", keys, err)
	}
}

func TestPullJPKeysCarryTheJPDataset(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	jk, _, err := newPullJP(t, seed(5, 50), clk).Plan(context.Background(), pullDay)
	if err != nil || len(jk) == 0 {
		t.Fatalf("keys %v err %v", jk, err)
	}
	uk, _, err := newPull(t, seed(5, 50), newFake(100)).Plan(context.Background(), pullDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(jk) != len(uk) {
		t.Fatalf("%d JP keys, %d US keys", len(jk), len(uk))
	}
	for i, k := range jk {
		if k.DatasetID != PullDatasetIDJP || k.SubjectKey != uk[i].SubjectKey {
			t.Errorf("key %+v, US %+v", k, uk[i])
		}
	}
}

func TestPullJPWindowIsXTKS(t *testing.T) {
	t.Parallel()
	j := newPullJP(t, seed(1, 1), clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC)))
	cal, open, closeOff := j.(runner.Windowed).Window()
	if cal != "XTKS" || open != 0 || closeOff != 0 {
		t.Errorf("window %s %v %v", cal, open, closeOff)
	}
}

func TestUSAndJPPullKeysAreDisjoint(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	us := newPull(t, seed(5, 50), newFake(100))
	jp := newPullJP(t, seed(5, 50), clk)
	owner := map[contracts.ExpectedKey]string{}
	total := 0
	for _, j := range []contracts.Job{us, jp} { // the first planner of a key owns it, as planLogged does
		keys, _, err := j.Plan(context.Background(), pullDay)
		if err != nil {
			t.Fatal(err)
		}
		total += len(keys)
		for _, k := range keys {
			if _, dup := owner[k]; !dup {
				owner[k] = j.ID()
			}
		}
	}
	if len(owner) != total || total == 0 {
		t.Fatalf("merged %d keys of %d planned", len(owner), total)
	}
	for k, id := range owner {
		if want := map[string]string{PullDatasetID: "option_open", PullDatasetIDJP: "option_open_jp"}[k.DatasetID]; id != want {
			t.Errorf("key %+v owned by %s", k, id)
		}
	}
}

func TestJPPullRunsItsOwnKeys(t *testing.T) {
	t.Parallel()
	clk := clockwork.NewFakeClockAt(time.Date(2026, 11, 11, 0, 5, 0, 0, time.UTC))
	s := seed(1, 2)
	j := newPullJP(t, s, clk)
	keys, _, err := j.Plan(context.Background(), pullDay)
	if err != nil || len(keys) == 0 {
		t.Fatalf("keys %v err %v", keys, err)
	}
	keys = append(keys, contracts.ExpectedKey{DatasetID: PullDatasetID, SubjectKey: keys[0].SubjectKey})
	if res := j.Run(context.Background(), pullDay, keys); res.Err != nil || res.Returned == 0 {
		t.Fatalf("err=%v returned %d", res.Err, res.Returned)
	}
	if len(s.written) == 0 || len(s.written[0]) == 0 {
		t.Errorf("written %+v", s.written)
	}
}
