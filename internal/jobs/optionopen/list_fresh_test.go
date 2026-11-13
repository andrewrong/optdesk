package optionopen

import (
	"context"
	"testing"
	"time"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// seedGroup stores a listed group of sym at expiry offset off, listed back days before the key day with the range
// lo to hi built with sigma 0.3 (lo empty: no range).
func (g *lrig) seedGroup(sym string, off, back int, lo, hi string) {
	g.t.Helper()
	t, _ := dayTime(g.day)
	rec := openquote.ListRecord{UnderlyingID: g.id(sym), Expiry: g.exp(off), Class: sym, Outcome: "listed", Count: 6,
		ListedSession: contracts.Day(t.AddDate(0, 0, -back).Format(time.DateOnly)), BandLo: lo, BandHi: hi, Sigma: 0.3, SigmaSource: "iv", InBand: 6}
	require.NoError(g.t, g.ls.PutLists(context.Background(), []openquote.ListRecord{rec}))
}

func (g *lrig) group(sym string, off int) openquote.ListRecord {
	g.t.Helper()
	rows, err := g.ls.Lists(context.Background(), g.id(sym))
	require.NoError(g.t, err)
	for _, r := range rows {
		if r.Expiry == g.exp(off) {
			return r
		}
	}
	return openquote.ListRecord{}
}

func TestFreshGroupIsNotAsked(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.seedGroup("AAA", 22, 3, "80", "125")
	g.seedGroup("AAA", 29, 0, "80", "125")
	j := g.job(nil)
	res := g.run(j)
	require.NoError(t, errOf(res))
	require.Empty(t, g.lst.only("list:"), "a group listed in the last seven days costs no ListContracts")
	require.Equal(t, 2, g.row("AAA").Listed)
	require.Equal(t, reasonNone, g.row("AAA").Reason)
	require.True(t, g.present(j))
}

func TestStaleGroupAfterSevenDaysIsListed(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.seedGroup("AAA", 22, 6, "80", "125") // D-6 is the oldest fresh day
	g.seedGroup("AAA", 29, 7, "80", "125")
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Equal(t, []string{"list:AAA@" + string(g.exp(29))}, g.lst.only("list:"))
	require.Equal(t, g.day, g.group("AAA", 29).ListedSession)
}

func TestTtlOneIsSessionOnly(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.seedGroup("AAA", 22, 1, "80", "125")
	g.seedGroup("AAA", 29, 0, "80", "125")
	require.NoError(t, errOf(g.run(g.job(replaceKey(t, "list_ttl_days = 7", "list_ttl_days = 1")))))
	require.Equal(t, []string{"list:AAA@" + string(g.exp(22))}, g.lst.only("list:"))
}

func TestSpotOutsideBandRelistsFreshGroup(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.closes = map[string]string{string(g.exp(-1)): "120"}
	g.seedGroup("AAA", 22, 2, "90", "110")
	g.seedGroup("AAA", 29, 2, "90", "150")
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Equal(t, []string{"list:AAA@" + string(g.exp(22))}, g.lst.only("list:"), "only the group whose zone left its range")
	rec := g.group("AAA", 22)
	require.Equal(t, g.day, rec.ListedSession)
	lo, hi := strikeValue(rec.BandLo), strikeValue(rec.BandHi)
	require.Less(t, lo, 120.0)
	require.Greater(t, hi, 120.0)
}

func TestSpotInsideBandKeepsFreshGroup(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.closes = map[string]string{string(g.exp(-1)): "101"}
	g.seedGroup("AAA", 22, 2, "90", "115")
	g.seedGroup("AAA", 29, 2, "90", "115")
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Empty(t, g.lst.only("list:"))
}

func TestNoRangeRelistsGroup(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.seedGroup("AAA", 22, 2, "", "")
	g.seedGroup("AAA", 29, 2, "80", "125")
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Equal(t, []string{"list:AAA@" + string(g.exp(22))}, g.lst.only("list:"))
	require.NotEmpty(t, g.group("AAA", 22).BandLo)
}

func TestTimeoutNeverMakesGroupFresh(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22}
	g.lst.list = func(int, string, contracts.Day, string) ([]broker.Contract, error) { return nil, errDeadline }
	res := g.run(g.job(nil))
	require.NotNil(t, res.Err)
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.Equal(t, openquote.ListRecord{}, g.group("AAA", 22), "a timeout writes no group")
	g.lst.list = nil
	g.lst.reset()
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Len(t, g.lst.only("list:"), 1, "the next run asks it again")
}

func TestShortListNeverMakesGroupFresh(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22}
	seedContracts(t, g, g.id("AAA"), "AAA", g.exp(22), 10)
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return g.lst.sample(class, "", "100"), nil
	}
	require.NotNil(t, g.run(g.job(nil)).Err)
	require.Equal(t, "short", g.group("AAA", 22).Outcome)
	g.lst.reset()
	g.run(g.job(nil))
	require.Len(t, g.lst.only("list:"), 1, "a short group is asked again")
}

func TestEmptyAnswerNoStoredIsListedZero(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22}
	g.lst.list = func(int, string, contracts.Day, string) ([]broker.Contract, error) { return nil, nil }
	require.NoError(t, errOf(g.run(g.job(nil))))
	rec := g.group("AAA", 22)
	require.Equal(t, "listed", rec.Outcome)
	require.Equal(t, 0, rec.Count)
	require.Equal(t, 0, rec.InBand)
	require.NotEmpty(t, rec.BandLo, "the negative entry carries its range")
	g.lst.reset()
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Empty(t, g.lst.only("list:"), "and is fresh like any other")
}

func TestDefaultSigmaStoresReason(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22}
	require.NoError(t, errOf(g.run(g.job(nil))))
	rec := g.group("AAA", 22)
	require.Equal(t, "default", rec.SigmaSource)
	require.Equal(t, DefaultSigma, rec.Sigma)
	require.Equal(t, 6, rec.InBand)
	// A band of the default sigma is fresh for its day only.
	g.lst.reset()
	g.run(g.job(nil))
	require.Empty(t, g.lst.only("list:"))
	g.setDay(g.exp(1))
	g.closes = map[string]string{string(g.exp(-1)): "100"}
	g.offs["AAA"] = []int{21}
	g.run(g.job(nil))
	require.Len(t, g.lst.only("list:"), 1, "the next day it is listed again")
}

func TestSigmaIsLargerOfIvAndRv20(t *testing.T) {
	t.Parallel()
	setup := func(calm bool) openquote.ListRecord {
		g := newLRig(t, "AAA")
		g.offs["AAA"] = []int{22}
		g.closes = map[string]string{}
		for i := 1; i <= 25; i++ {
			c := "100"
			if !calm && i%2 == 0 {
				c = "103"
			}
			g.closes[string(g.exp(-i))] = c
		}
		id := g.id("AAA")
		require.NoError(t, g.opts.PutOption(context.Background(), openquote.OptionRef{UnderlyingID: id, TradingClass: "AAA", Expiry: g.exp(22),
			Strike: "100", Right: "P", Multiplier: "100", InstrumentID: 77}))
		iv := 0.25
		day, _ := dayTime(g.exp(-2))
		g.tweak = func(d *ListDeps) {
			d.Eod = fakeEod{rows: []contracts.OptionEod{{InstrumentID: 77, IvDecimal: &iv, SessionDay: types.SerializableDate{Time: day}}}}
		}
		require.NoError(t, errOf(g.run(g.job(nil))))
		return g.group("AAA", 22)
	}
	calm := setup(true)
	require.Equal(t, []any{"iv", 0.25}, []any{calm.SigmaSource, calm.Sigma}, "the stored implied vol when the closes are flat")
	rough := setup(false)
	require.Equal(t, "rv20", rough.SigmaSource)
	require.Greater(t, rough.Sigma, 0.25)
}

func TestMissingBarAsksGateway(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.closes = map[string]string{}
	h := &fakeHistory{bars: []broker.Bar{{Day: string(g.exp(-1)), Close: "100"}}}
	g.tweak = func(d *ListDeps) { d.History = h }
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Equal(t, 1, h.asked, "one request for the name, whatever its groups")
	require.Len(t, g.lst.only("list:"), 2)

	// A gateway that cannot answer: no listing and the run is Retryable, never NoValue.
	g2 := newLRig(t, "AAA")
	g2.closes = map[string]string{}
	h2 := &fakeHistory{err: downErr{}}
	g2.tweak = func(d *ListDeps) { d.History = h2 }
	res := g2.run(g2.job(nil))
	require.NotNil(t, res.Err)
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.Empty(t, g2.lst.only("list:"))
	require.Equal(t, 1, h2.asked)
	require.Equal(t, reasonNoSpot, g2.row("AAA").Reason)

	g3 := newLRig(t, "AAA")
	g3.closes = map[string]string{}
	res = g3.run(g3.job(nil))
	require.NotNil(t, res.Err, "no history client: the same")
	require.Equal(t, contracts.Retryable, res.Err.Kind())
}

// heldRig has AAA held and BBB a name of the watch list, one expiry each.
func heldRig(t *testing.T) *lrig {
	g := newLRig(t, "AAA", "BBB")
	g.acc.pos = []store.Position{{SecType: "stock", Currency: "USD", Symbol: "AAA", ConID: 100, Quantity: "100"}}
	g.offs["AAA"], g.offs["BBB"] = []int{22}, []int{22}
	return g
}

func failing(g *lrig, sym string, err error) {
	g.lst.list = func(_ int, s string, _ contracts.Day, class string) ([]broker.Contract, error) {
		if s == sym {
			return nil, err
		}
		return g.lst.sample(class, ""), nil
	}
}

func TestHeldStaleGroupMakesPartialRunRetryable(t *testing.T) {
	t.Parallel()
	g := heldRig(t)
	failing(g, "AAA", errDeadline)
	res := g.run(g.job(nil))
	require.NotNil(t, res.Err, "BBB listed, but the held name has a stale group")
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.True(t, g.present(g.job(nil)))
	require.Equal(t, 1, res.Usable)
	g.lst.list = nil
	g.lst.reset()
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Equal(t, []string{"list:AAA@" + string(g.exp(22))}, g.lst.only("list:"), "the retry asks the held group only")
}

func TestOtherStaleGroupDoesNotMakeRunRetryable(t *testing.T) {
	t.Parallel()
	g := heldRig(t)
	failing(g, "BBB", errDeadline)
	res := g.run(g.job(nil))
	require.NoError(t, errOf(res))
	require.Equal(t, 1, res.Usable)
}

func TestNotServedIsNotRetryableUnderTtl(t *testing.T) {
	t.Parallel()
	g := heldRig(t)
	failing(g, "AAA", &notServed{})
	res := g.run(g.job(nil))
	require.NoError(t, errOf(res))
	require.Equal(t, reasonUnserved, g.row("AAA").Reason)
}

func TestStaleHeldWritesHeldStaleReason(t *testing.T) {
	t.Parallel()
	g := heldRig(t)
	rs := &fakeReasons{}
	g.tweak = func(d *ListDeps) { d.ReasonWriter = rs }
	failing(g, "AAA", errDeadline)
	g.run(g.job(nil))
	got := rs.of(g.day, ListDatasetID, ListSubject)
	require.Len(t, got, 1)
	require.Equal(t, "held_stale", got[0].Code)
	require.Contains(t, got[0].Detail, "AAA "+string(g.exp(22)))
	require.NotContains(t, got[0].Detail, "BBB")
	g.lst.list = nil
	require.NoError(t, errOf(g.run(g.job(nil))))
	require.Empty(t, rs.of(g.day, ListDatasetID, ListSubject), "a clean run clears it")
}

func TestListSetsUsable(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	res := g.run(g.job(nil))
	require.NoError(t, errOf(res))
	require.Equal(t, []int{4, 4}, []int{res.Returned, res.Usable})
}

// fakeSpot answers Snapshot by symbol.
type fakeSpot struct {
	q   map[string]broker.Quote
	err error
	n   int
}

func (f *fakeSpot) Snapshot(_ context.Context, _ broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	f.n++
	if f.err != nil {
		return nil, f.err
	}
	var out []broker.Quote
	for _, c := range cs {
		out = append(out, f.q[c.Symbol])
	}
	return out, nil
}

func zoneRig(t *testing.T, symbols ...string) *lrig {
	g := newLRig(t, symbols...)
	g.offs = map[string][]int{}
	for _, s := range symbols {
		g.offs[s] = []int{22, 29}
	}
	return g
}

func (g *lrig) zoneJob() *listJob {
	return g.job(replaceKey(g.t, `subject = "preopen"`, "subject = \"zone\"\nmode = \"zone\""))
}

func (g *lrig) quote(price float64, withEvent bool) broker.Quote {
	q := broker.Quote{Last: &price, ReceivedAt: g.clk.Now()}
	if withEvent {
		now := g.clk.Now()
		q.EventTs = &now
	}
	return q
}

func TestZoneModeListsOnlyOutsideRange(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA", "BBB", "CCC")
	rs := &fakeReasons{}
	spot := &fakeSpot{q: map[string]broker.Quote{"AAA": g.quote(120, true), "BBB": g.quote(100, true), "CCC": g.quote(100, false)}}
	g.tweak = func(d *ListDeps) { d.ReasonWriter, d.Spot = rs, spot }
	for _, s := range g.watch {
		g.seedGroup(s, 22, 2, "90", "115")
		g.seedGroup(s, 29, 2, "90", "115")
	}
	g.lst.list = func(n int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		if n == 1 {
			return nil, errDeadline
		}
		return g.lst.sample(class, ""), nil
	}
	j := g.zoneJob()
	res := j.Run(context.Background(), g.day, []contracts.ExpectedKey{{DatasetID: ListDatasetID, SubjectKey: "zone"}})
	require.NoError(t, errOf(res))
	require.Equal(t, 1, res.Usable)
	require.Equal(t, []string{"list:AAA@" + string(g.exp(22)), "list:AAA@" + string(g.exp(29))}, g.lst.only("list:"))
	require.Equal(t, 1, g.lst.peak)
	require.Equal(t, 0, g.ls.puts, "no run row")
	require.Equal(t, g.day, g.group("AAA", 22).ListedSession)
	rows := rs.of(g.day, ListDatasetID, "zone")
	require.Len(t, rows, 2)
	byScope := map[string]string{}
	for _, r := range rows {
		byScope[r.Scope] = r.Code + ":" + r.Detail
	}
	require.Equal(t, map[string]string{"AAA": "outside_range:" + string(g.exp(29)), "CCC": "no_spot:no usable pre-open quote with an EventTs"}, byScope)
	ok, err := j.Postcondition(context.Background(), string(g.day), ListDatasetID, "zone")
	require.NoError(t, err)
	require.True(t, ok)
}

func TestZoneModeNoSpotListsNothing(t *testing.T) {
	t.Parallel()
	g := zoneRig(t, "AAA", "BBB")
	rs := &fakeReasons{}
	spot := &fakeSpot{err: downErr{}}
	g.tweak = func(d *ListDeps) { d.ReasonWriter, d.Spot = rs, spot }
	for _, s := range g.watch {
		g.seedGroup(s, 22, 2, "99", "101")
	}
	res := g.zoneJob().Run(context.Background(), g.day, []contracts.ExpectedKey{{DatasetID: ListDatasetID, SubjectKey: "zone"}})
	require.NoError(t, errOf(res))
	require.Empty(t, g.lst.calls)
	rows := rs.of(g.day, ListDatasetID, "zone")
	require.Len(t, rows, 2)
	for _, r := range rows {
		require.Equal(t, "no_spot", r.Code)
	}
	require.Equal(t, 0, g.ls.puts)
}
