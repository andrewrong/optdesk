package optionopen

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

// recLister records every request the job sends, whole.
type recLister struct {
	*fakeLister
	mu     sync.Mutex
	chains []broker.Contract
	lists  []listReq
}

type listReq struct {
	u      broker.Contract
	expiry contracts.Day
	right  string
	class  string
}

func (r *recLister) ListChains(ctx context.Context, c broker.Contract) ([]broker.Chain, error) {
	r.mu.Lock()
	r.chains = append(r.chains, c)
	r.mu.Unlock()
	return r.fakeLister.ListChains(ctx, c)
}

func (r *recLister) ListContracts(ctx context.Context, u broker.Contract, e contracts.Day, right, class string) ([]broker.Contract, error) {
	r.mu.Lock()
	r.lists = append(r.lists, listReq{u, e, right, class})
	r.mu.Unlock()
	return r.fakeLister.ListContracts(ctx, u, e, right, class)
}

// jpRig is a list rig of the JP names 7203 (a stock) and N225 (the index of class 225).
type jpRig struct {
	*lrig
	rec *recLister
}

func newJPRig(t *testing.T, offs map[string][]int) *jpRig {
	t.Helper()
	g := newLRig(t, "7203", "N225")
	g.offs = offs
	g.lst.chains = func(sym string) ([]broker.Chain, error) {
		ch := g.chainsOf(sym)
		ch[0].Exchange, ch[0].TradingClass = "OSE.JPN", sym
		if sym == "N225" {
			ch[0].TradingClass = "225"
		}
		return ch, nil
	}
	rec := &recLister{fakeLister: g.lst}
	g.tweak = func(d *ListDeps) {
		d.Lister = rec
		d.Watch = []jobs.WatchEntry{
			{Market: "JP", Contract: broker.Contract{SecType: "STK", Symbol: "7203", Exchange: "TSEJ", Currency: "JPY", ConID: 100}},
			{Market: "JP", Contract: broker.Contract{SecType: "IND", Symbol: "N225", Exchange: "OSE.JPN", Currency: "JPY", ConID: 101}},
		}
	}
	return &jpRig{g, rec}
}

func (g *jpRig) job() *listJob {
	g.t.Helper()
	j, err := NewListPreopenJP(g.deps())
	require.NoError(g.t, err)
	return j.(*listJob)
}

func (g *jpRig) run(j *listJob) contracts.JobResult {
	return j.Run(context.Background(), g.day, []contracts.ExpectedKey{{DatasetID: "master.option_list.jp", SubjectKey: "preopen"}})
}

func TestJPRequestsAreFullySpecified(t *testing.T) {
	t.Parallel()
	g := newJPRig(t, map[string][]int{"7203": {20}, "N225": {20}})
	require.NoError(t, errOf(g.run(g.job())))
	require.Len(t, g.rec.chains, 2)
	require.Len(t, g.rec.lists, 2)
	want := map[string]string{"7203": "STK", "N225": "IND"}
	classes := map[string]string{"7203": "7203", "N225": "225"}
	var all []broker.Contract
	all = append(all, g.rec.chains...)
	for _, l := range g.rec.lists {
		all = append(all, l.u)
		require.Equal(t, "", l.right, "both rights in one call")
		require.Equal(t, classes[l.u.Symbol], l.class)
	}
	for _, c := range all {
		require.Equal(t, "OSE.JPN", c.Exchange)
		require.Equal(t, "JPY", c.Currency)
		require.Equal(t, want[c.Symbol], c.SecType)
		require.NotZero(t, c.ConID)
	}
}

func TestJPListUsesOseClassAndMultiplier(t *testing.T) {
	t.Parallel()
	g := newJPRig(t, map[string][]int{"7203": {20}, "N225": {20}})
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return append(g.lst.sample(class, ""), g.lst.sample(class, "10", "200")...), nil
	}
	require.NoError(t, errOf(g.run(g.job())))
	for sym, class := range map[string]string{"7203": "7203", "N225": "225"} {
		rows, err := g.ls.Lists(context.Background(), g.id(sym))
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, class, rows[0].Class)
		require.Equal(t, "listed", rows[0].Outcome)
		require.Equal(t, 6, rows[0].Count, "the contracts of multiplier 10 are not stored")
	}
}

func TestJPWindowIs14To90(t *testing.T) {
	t.Parallel()
	g := newJPRig(t, map[string][]int{"7203": {13, 14, 90, 91}, "N225": {13, 14, 90, 91}})
	require.NoError(t, errOf(g.run(g.job())))
	got := map[contracts.Day]bool{}
	for _, l := range g.rec.lists {
		got[l.expiry] = true
	}
	require.Equal(t, map[contracts.Day]bool{g.exp(14): true, g.exp(90): true}, got)
}

func TestJPStartsAtSessionOpenBeforeStop(t *testing.T) {
	t.Parallel()
	j := newJPRig(t, nil).job()
	cal, anchor, zone, off := j.Schedule()
	require.Equal(t, []any{"XTKS", "session_open", "Asia/Tokyo", time.Duration(0)}, []any{cal, anchor, zone, off})
	require.Equal(t, []int{9, 20}, []int{j.m.stopH, j.m.stopM})
	require.Equal(t, "option_list_preopen_jp", j.ID())
	wc, _, _ := j.Window()
	require.Equal(t, "XTKS", wc)
	require.Equal(t, 7, j.m.TTLDays)
	keys, _, err := j.Plan(context.Background(), "2026-11-12")
	require.NoError(t, err)
	require.Equal(t, []contracts.ExpectedKey{{DatasetID: "master.option_list.jp", SubjectKey: "preopen"}}, keys)
}

func TestZoneSlotIsBeforePlanSlot(t *testing.T) {
	t.Parallel()
	z, err := parseList(listManifestZone)
	require.NoError(t, err)
	p, err := parseManifest(planManifest)
	require.NoError(t, err)
	_, _, _, planOff := p.schedule()
	require.Less(t, z.offset, planOff)
	require.Equal(t, p.Schedule.Anchor, z.Schedule.Anchor)
	require.Equal(t, p.Schedule.Calendar, z.Schedule.Calendar)
}

func TestNewListZoneUsesZoneToml(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA")
	_, err := NewListZone(g.deps())
	require.Error(t, err, "the zone mode needs a spot reader")
	g.tweak = func(d *ListDeps) { d.Spot = &fakeSpot{} }
	j, err := NewListZone(g.deps())
	require.NoError(t, err)
	require.Equal(t, "option_list_zone", j.ID())
	lj := j.(*zoneJob).listJob
	_, _, _, off := lj.Schedule()
	require.Equal(t, -75*time.Minute, off)
	keys, _, err := lj.Plan(context.Background(), g.day)
	require.NoError(t, err)
	require.Equal(t, []contracts.ExpectedKey{{DatasetID: "master.option_list", SubjectKey: "zone"}}, keys)
	require.Equal(t, modeZone, lj.m.Mode)
	require.Equal(t, []int{8, 40}, []int{lj.m.stopH, lj.m.stopM})
}

func TestJPHasNoSecondSlot(t *testing.T) {
	t.Parallel()
	g := newJPRig(t, map[string][]int{"7203": {20}, "N225": {20}})
	spot := &fakeSpot{}
	inner := g.tweak
	g.tweak = func(d *ListDeps) { inner(d); d.Spot = spot }
	j := g.job()
	require.Equal(t, modePreopen, j.m.Mode)
	require.NoError(t, errOf(g.run(j)))
	require.Zero(t, spot.n, "the JP list reads no pre-open spot")
	keys, _, err := j.Plan(context.Background(), g.day)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	ok, err := j.Postcondition(context.Background(), string(g.day), "master.option_list.jp", "preopen")
	require.NoError(t, err)
	require.True(t, ok)
}

func zoneKey() []contracts.ExpectedKey {
	return []contracts.ExpectedKey{{DatasetID: ListDatasetID, SubjectKey: "zone"}}
}

func TestZoneRunWithoutSpotWritesNoSpot(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA", "BBB")
	rs := &fakeReasons{}
	g.tweak = func(d *ListDeps) {
		d.ReasonWriter = rs
		d.Spot = &fakeSpot{q: map[string]broker.Quote{"BBB": g.quote(100, true)}}
	}
	for _, s := range g.watch {
		g.seedGroup(s, 22, 2, "90", "115")
	}
	j, err := NewListZone(g.deps())
	require.NoError(t, err)
	require.NoError(t, errOf(j.Run(context.Background(), g.day, zoneKey())))
	rows := rs.of(g.day, ListDatasetID, "zone")
	require.Len(t, rows, 1)
	require.Equal(t, "AAA", rows[0].Scope)
	require.Equal(t, "no_spot", rows[0].Code)
	require.Empty(t, g.lst.calls)
}

func TestZoneOutsideRangeWritesReasonAndIsPresent(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA")
	rs := &fakeReasons{}
	g.tweak = func(d *ListDeps) {
		d.ReasonWriter = rs
		d.Spot = &fakeSpot{q: map[string]broker.Quote{"AAA": g.quote(130, true)}}
	}
	g.seedGroup("AAA", 22, 2, "90", "115")
	g.seedGroup("AAA", 29, 2, "90", "115")
	g.lst.list = func(int, string, contracts.Day, string) ([]broker.Contract, error) { return nil, errDeadline }
	j, err := NewListZone(g.deps())
	require.NoError(t, err)
	require.NoError(t, errOf(j.Run(context.Background(), g.day, zoneKey())))
	rows := rs.of(g.day, ListDatasetID, "zone")
	require.Len(t, rows, 1)
	require.Equal(t, "AAA", rows[0].Scope)
	require.Equal(t, "outside_range", rows[0].Code)
	require.Equal(t, string(g.exp(22))+","+string(g.exp(29)), rows[0].Detail)
	require.Zero(t, g.ls.puts, "no option_list_run row")
	ok, err := j.(*zoneJob).Postcondition(context.Background(), string(g.day), ListDatasetID, "zone")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, g.exp(-2), g.group("AAA", 22).ListedSession, "the stored plan is kept")
}

func TestZoneSpotAgeUsesEventTsNotReceivedAt(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA", "BBB")
	rs := &fakeReasons{}
	now := g.clk.Now()
	old := now.Add(-200 * time.Hour)
	price := 130.0
	g.tweak = func(d *ListDeps) {
		d.ReasonWriter = rs
		d.Spot = &fakeSpot{q: map[string]broker.Quote{
			"AAA": {Last: &price, EventTs: &old, ReceivedAt: now},
			"BBB": {Last: &price, EventTs: &now, ReceivedAt: old},
		}}
	}
	for _, s := range g.watch {
		g.seedGroup(s, 22, 2, "90", "115")
	}
	j, err := NewListZone(g.deps())
	require.NoError(t, err)
	require.NoError(t, errOf(j.Run(context.Background(), g.day, zoneKey())))
	require.Equal(t, []string{"list:BBB@" + string(g.exp(22))}, g.lst.only("list:"), "only the fresh trade time lists")
	var codes []string
	for _, r := range rs.of(g.day, ListDatasetID, "zone") {
		codes = append(codes, r.Scope+":"+r.Code)
	}
	require.Equal(t, []string{"AAA:no_spot"}, codes)
}

func TestJPEmptyAnswerIsListedZeroNotShort(t *testing.T) {
	t.Parallel()
	g := newJPRig(t, map[string][]int{"7203": {20}, "N225": {20}})
	g.lst.list = func(int, string, contracts.Day, string) ([]broker.Contract, error) { return nil, nil }
	g.run(g.job())
	for _, sym := range []string{"7203", "N225"} {
		rows, err := g.ls.Lists(context.Background(), g.id(sym))
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, "listed", rows[0].Outcome)
		require.Zero(t, rows[0].Count)
	}
}

// onlyContracts lists contracts but not chains.
type onlyContracts struct{ broker.ContractLister }

func TestJPListerWithoutChainsIsRefused(t *testing.T) {
	t.Parallel()
	g := newJPRig(t, nil)
	inner := g.tweak
	g.tweak = func(d *ListDeps) { inner(d); d.Lister = onlyContracts{g.lst} }
	_, err := NewListPreopenJP(g.deps())
	require.Error(t, err, "an index would be sent as a stock")
}

func TestZoneRunAfterStopTimeLists(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA")
	rs := &fakeReasons{}
	g.tweak = func(d *ListDeps) {
		d.ReasonWriter = rs
		d.Spot = &fakeSpot{q: map[string]broker.Quote{"AAA": g.quote(130, true)}}
	}
	g.seedGroup("AAA", 22, 2, "90", "115")
	g.clk.Advance(2*time.Hour + 30*time.Minute) // 08:45 ET, after the stop time 08:40
	j, err := NewListZone(g.deps())
	require.NoError(t, err)
	require.NoError(t, errOf(j.Run(context.Background(), g.day, zoneKey())))
	require.NotEmpty(t, g.lst.calls, "a zone run after the stop time lists as any other run")
	for _, r := range rs.of(g.day, ListDatasetID, "zone") {
		require.NotEqual(t, "late"+"_start", r.Code)
	}
}
