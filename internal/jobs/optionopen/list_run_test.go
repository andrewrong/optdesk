package optionopen

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

var errDeadline = fmt.Errorf("list: %w", context.DeadlineExceeded)

type downErr struct{}

func (downErr) Error() string             { return "gateway down" }
func (downErr) Kind() contracts.ErrorKind { return contracts.Retryable }

func TestRunCallOrderAcrossNamesUnderOneBudgetClock(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB", "CCC", "DDD")
	g.acc.calls = map[string]int{}
	g.acc.pos = []store.Position{{SecType: "stock", Symbol: "CCC", ConID: 102, Quantity: "100"}}
	j := g.job(nil)
	j.d.Watch[1].BuyPrice = "50"
	for _, s := range g.watch {
		g.offs[s] = []int{22, 23}
	}
	require.NoError(t, errOf(g.run(j)))
	require.Equal(t, 1, g.acc.calls["A"], "stocks() is called once at the start of the run")
	want := func(sym string, off int) string { return fmt.Sprintf("list:%s@%s", sym, g.exp(off)) }
	require.Equal(t, []string{want("CCC", 22), want("CCC", 23), want("BBB", 22), want("BBB", 23),
		want("AAA", 22), want("DDD", 22), want("AAA", 23), want("DDD", 23)}, g.lst.only("list:"))
	require.Equal(t, []string{"chains:CCC", "chains:BBB", "chains:AAA", "chains:DDD"}, g.lst.only("chains:"))
	require.Equal(t, 1, g.lst.peak, "calls in flight over a whole run")
	require.Equal(t, 12*5*time.Second, g.clk.Now().Sub(time.Date(2026, 11, 12, 6, 15, 0, 0, newYork)))
	for _, s := range g.watch {
		require.Equal(t, reasonNone, g.row(s).Reason)
		require.Equal(t, 2, g.row(s).Listed)
	}
}

func errOf(r contracts.JobResult) error {
	if r.Err == nil {
		return nil
	}
	return r.Err
}

func TestPassListingNothingIsRetryable(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB", "CCC")
	g.lst.chains = func(string) ([]broker.Chain, error) { return nil, errDeadline }
	j := g.job(nil)
	res := g.run(j)
	require.NotNil(t, res.Err)
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.False(t, g.present(j))
	require.Equal(t, []time.Duration{30 * time.Second, 30 * time.Second}, g.pauses)
	require.Len(t, g.lst.only("chains:"), 2, "no third call after two timeouts")
	require.Equal(t, reasonChainsFail, g.row("AAA").Reason, "the first timeout failed its name only")
	for _, s := range g.watch {
		require.Nil(t, g.row(s).Expected)
	}
	require.Equal(t, reasonTimeouts, g.row("BBB").Reason)
	require.Equal(t, reasonTimeouts, g.row("CCC").Reason)
	for _, name := range g.watch {
		require.NotContains(t, res.Err.Error(), name, "the error text carries no name")
	}
}

func TestAllShortIsRetryable(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	id := g.id("AAA")
	for _, o := range []int{22, 29} {
		seedContracts(t, g, id, "AAA", g.exp(o), 10)
	}
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return g.lst.sample(class, "", "100"), nil
	}
	j := g.job(nil)
	res := g.run(j)
	require.NotNil(t, res.Err)
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.False(t, g.present(j))
	require.Equal(t, reasonAllShort, g.row("AAA").Reason)
	require.Equal(t, 20, g.opts.count(id), "a short list keeps the contracts and adds none")
}

func seedContracts(t *testing.T, g *lrig, id int64, class string, expiry contracts.Day, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		r := []string{"C", "P"}[i%2]
		require.NoError(t, g.opts.PutOption(context.Background(), openquote.OptionRef{UnderlyingID: id, TradingClass: class,
			Expiry: expiry, Strike: fmt.Sprint(50 + i/2), Right: r, Multiplier: "100"}))
	}
}

func TestCompletePassThenCrashedRerunIsPresentWithoutCalls(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	j := g.job(nil)
	require.NoError(t, errOf(g.run(j)))
	before, puts := g.row("AAA"), g.ls.puts
	g.lst.reset()
	res := g.run(j)
	require.Nil(t, res.Err)
	require.Empty(t, g.lst.calls, "a re-run with every row none makes no broker call")
	require.True(t, g.present(j))
	after := g.row("AAA")
	require.Equal(t, before.Expected, after.Expected, "a re-run does not change expected")
	require.Equal(t, reasonNone, after.Reason)
	require.Equal(t, puts, g.ls.puts, "and writes no row")
}

func TestListPastStopTimeLists(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.clk.Advance(76 * time.Minute) // 07:31
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	require.NotEmpty(t, g.lst.calls)
	require.True(t, g.present(j))
	for _, s := range g.watch {
		require.Greater(t, g.row(s).Listed, 0)
	}
	require.Contains(t, g.logs[len(g.logs)-1], "past_stop_time=true")
}

func TestRerunAfterCrashBetweenListRowAndRunRowCountsIt(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.seed("AAA", reasonNotFinished, 2, 22, 29) // the option_list rows exist, the run row says 0 listed
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	require.Empty(t, g.lst.calls)
	row := g.row("AAA")
	require.Equal(t, 2, row.Listed)
	require.Equal(t, reasonNone, row.Reason)
	require.True(t, g.present(j))
}

func TestAllNoExpiryIsRetryable(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.offs["AAA"], g.offs["BBB"] = []int{10, 60}, []int{3}
	j := g.job(nil)
	res := g.run(j)
	require.NotNil(t, res.Err)
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.False(t, g.present(j))
	require.Empty(t, g.lst.only("list:"))
	for _, s := range g.watch {
		require.Equal(t, reasonNoExpiry, g.row(s).Reason)
		require.Equal(t, []string{}, g.row(s).Expected)
	}
	g.lst.reset()
	require.NotNil(t, g.run(j).Err)
	require.Equal(t, []string{"chains:AAA", "chains:BBB"}, g.lst.only("chains:"), "the re-run asks every name again")
}

func TestNoExpiryWithOneListedIsPresent(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.offs["BBB"] = []int{10}
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	require.True(t, g.present(j))
	require.Equal(t, reasonNoExpiry, g.row("BBB").Reason)
	require.Equal(t, reasonNone, g.row("AAA").Reason)
	g.lst.reset()
	require.Nil(t, g.run(j).Err)
	require.Empty(t, g.lst.calls, "a name with no expiry is not asked again once something is listed")
}

func TestEmptyListChainsIsListchainsFailed(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.lst.chains = func(sym string) ([]broker.Chain, error) {
		if sym == "AAA" {
			return nil, nil
		}
		return g.chainsOf(sym), nil
	}
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	require.Equal(t, reasonChainsFail, g.row("AAA").Reason)
	require.Nil(t, g.row("AAA").Expected)
	g2 := newLRig(t, "AAA", "BBB")
	g2.lst.chains = func(string) ([]broker.Chain, error) { return nil, nil }
	res := g2.run(g2.job(nil))
	require.NotNil(t, res.Err)
	require.Equal(t, contracts.Retryable, res.Err.Kind())
	require.Equal(t, reasonChainsFail, g2.row("BBB").Reason)
	require.Empty(t, g2.lst.only("list:"))
	require.Contains(t, strings.Join(g2.logs, "\n"), "list_chains_empty: BBB answered no chains")
	require.Contains(t, strings.Join(g.logs, "\n"), "list_chains_empty: AAA answered no chains")
}

func TestListChainsErrorThatDoesNotStopIsLogged(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.lst.chains = func(sym string) ([]broker.Chain, error) {
		if sym == "AAA" {
			return nil, errors.New("rejected by gateway")
		}
		return g.chainsOf(sym), nil
	}
	require.Nil(t, g.run(g.job(nil)).Err)
	require.Equal(t, reasonChainsFail, g.row("AAA").Reason)
	var hit bool
	for _, l := range g.logs {
		hit = hit || (strings.Contains(l, "AAA") && strings.Contains(l, "rejected by gateway"))
	}
	require.True(t, hit, g.logs)
}

func TestOneTimeoutIsOneTimeout(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22, 25, 29}
	g.lst.list = func(n int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		if n == 0 {
			return nil, errDeadline
		}
		return g.lst.sample(class, ""), nil
	}
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	require.Equal(t, []time.Duration{30 * time.Second}, g.pauses)
	require.Len(t, g.lst.only("list:"), 3, "the run goes on after one timeout")
	require.Equal(t, reasonSomeShort, g.row("AAA").Reason)
	require.Equal(t, 2, g.row("AAA").Listed)

	g2 := newLRig(t, "AAA")
	g2.offs["AAA"] = []int{22, 25, 29}
	g2.lst.list = func(int, string, contracts.Day, string) ([]broker.Contract, error) { return nil, errDeadline }
	j2 := g2.job(nil)
	res := g2.run(j2)
	require.NotNil(t, res.Err)
	require.Equal(t, []time.Duration{30 * time.Second, 30 * time.Second}, g2.pauses)
	require.Len(t, g2.lst.only("list:"), 2, "no third call")
	require.Equal(t, reasonTimeouts, g2.row("AAA").Reason)
}

func TestNameWithSomeGroupsShortIsSomeShort(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22, 25, 29}
	seedContracts(t, g, g.id("AAA"), "AAA", g.exp(25), 10)
	g.lst.list = func(n int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		switch n {
		case 0:
			return g.lst.sample(class, ""), nil
		case 1:
			return g.lst.sample(class, "", "100"), nil // short of the 10 stored
		}
		return nil, errors.New("rejected")
	}
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	row := g.row("AAA")
	require.Equal(t, reasonSomeShort, row.Reason)
	require.Equal(t, 1, row.Listed)
	require.Len(t, row.Expected, 3)
	require.True(t, g.present(j))
}

func TestBudgetStopWithSomethingListedIsErrNilAndPartial(t *testing.T) {
	t.Parallel()
	var syms []string
	for i := 0; i < 45; i++ {
		syms = append(syms, fmt.Sprintf("N%02d", i))
	}
	g := newLRig(t, syms...)
	g.lst.dur = 7300 * time.Millisecond
	for i, s := range syms {
		n := 4
		if i < 5 {
			n = 17
		}
		for k := 0; k < n; k++ {
			g.offs[s] = append(g.offs[s], 21+k)
		}
	}
	j := g.job(replaceKey(t, "list_budget_minutes = 45", "list_budget_minutes = 20"))
	res := g.run(j)
	require.Nil(t, res.Err)
	require.True(t, g.present(j))
	require.Len(t, g.lst.only("chains:"), 45)
	require.Len(t, g.lst.only("list:"), 120, "the 121st does not start")
	require.Equal(t, 120, res.Returned)
	budget := 0
	for _, s := range syms {
		if r := g.row(s); r.Reason == reasonBudget {
			budget++
			require.Less(t, r.Listed, len(r.Expected))
		}
	}
	require.Positive(t, budget)
	require.Len(t, g.lst.only("list:"), 120, "no re-run")
}

func TestRerunAsksOnlyTheGroupsNotListedForTheSession(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.seed("AAA", reasonBudget, 3, 22, 24, 26, 28, 30)
	before := g.row("AAA").Expected
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	require.Equal(t, []string{"list:AAA@" + string(g.exp(28)), "list:AAA@" + string(g.exp(30))}, g.lst.only("list:"))
	require.Empty(t, g.lst.only("chains:"))
	require.Equal(t, before, g.row("AAA").Expected)
	require.Equal(t, 5, g.row("AAA").Listed)
	require.Equal(t, reasonNone, g.row("AAA").Reason)
}

func TestAListAndAPerStrikeResolveStoreOneRow(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{29}
	id := g.id("AAA")
	d := ResolveDeps{Stored: g.deps().Stored, Clock: g.clk}
	rj := &resolveJob{d: d}
	exp := g.exp(29)
	req := broker.Contract{Symbol: "AAA", Expiry: strings.ReplaceAll(string(exp), "-", ""), Right: "C", Strike: "660", Multiplier: "100", TradingClass: "AAA"}
	got := req
	got.ConID = 777
	kr := keyRun{j: rj}
	require.Nil(t, kr.store(context.Background(), id, candidate{req: req, expiry: exp}, got))
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return g.lst.sample(class, "", "660", "660.0", "12.5"), nil
	}
	j := g.job(nil)
	require.Nil(t, g.run(j).Err)
	refs, _ := g.opts.ListOptions(context.Background(), id)
	require.Len(t, refs, 4, "660 C once, 660 P, 12.5 C and P: 660.0 is the same strike")
	cached := map[string]bool{}
	strikes := map[string]bool{}
	for _, r := range refs {
		cached[refKey(r.Expiry, r.Strike, r.Right, r.TradingClass)] = true
		strikes[r.Strike] = true
	}
	require.True(t, cached[refKey(exp, "660", "C", "AAA")], "the evening resolve finds the listed contract")
	require.Equal(t, map[string]bool{"660": true, "12.5": true}, strikes, "the strike text is the broker's, never 25/2")
	require.Equal(t, 4, g.opts.puts, "one put by the resolve, three by the list")
}

func TestRunRowsAtStartAndTheReasonsOfAnEnd(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.seed("BBB", reasonBudget, 1, 22, 29)
	started := false
	g.lst.chains = func(sym string) ([]broker.Chain, error) {
		if !started {
			started = true
			require.Equal(t, reasonNotFinished, g.row("AAA").Reason)
			require.Nil(t, g.row("AAA").Expected, "written at the start, expected null")
			require.Equal(t, reasonBudget, g.row("BBB").Reason, "an existing row is never overwritten at the start")
		}
		return nil, errors.New("rejected")
	}
	j := g.job(nil)
	res := g.run(j)
	require.Equal(t, reasonChainsFail, g.row("AAA").Reason)
	require.Equal(t, reasonNone, g.row("BBB").Reason, "BBB has expected groups: no ListChains, its last group is listed")
	require.Equal(t, []string{"chains:AAA"}, g.lst.only("chains:"))
	require.Nil(t, res.Err)
	require.Contains(t, g.logs[len(g.logs)-1], "AAA="+reasonChainsFail)
	require.Contains(t, g.logs[len(g.logs)-1], "option_list_preopen day=2026-11-12")
}

func TestStopsAtTheEndsThatLeaveAReason(t *testing.T) {
	t.Parallel()
	for name, c := range map[string]struct {
		toml   []byte
		err    error
		reason string
	}{
		"broker down": {nil, downErr{}, reasonBroker},
		"unserved":    {nil, &notServed{}, reasonUnserved},
	} {
		g := newLRig(t, "AAA", "BBB")
		g.lst.list = func(int, string, contracts.Day, string) ([]broker.Contract, error) { return nil, c.err }
		j := g.job(c.toml)
		res := g.run(j)
		require.Len(t, g.lst.only("list:"), 1, name)
		for _, s := range g.watch {
			require.Equal(t, c.reason, g.row(s).Reason, name)
			require.Len(t, g.row(s).Expected, 2, name)
		}
		require.False(t, g.present(j), name)
		if c.reason == reasonUnserved {
			require.Nil(t, res.Err, "unserved ends Err nil and the key stays unmarked")
			require.Contains(t, strings.Join(g.logs, "\n"), "list_unserved: the pyside service has not been restarted")
		} else {
			require.NotNil(t, res.Err, name)
			require.Equal(t, contracts.Retryable, res.Err.Kind())
		}
	}
}

const reasonBroker = reasonBrokerDown

type notServed struct{}

func (*notServed) Error() string { return "wrapped: " + broker.ErrNotServed.Error() }
func (*notServed) Unwrap() error { return broker.ErrNotServed }

func TestListMaxNewStopsTheWrites(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	j := g.job(replaceKey(t, "list_max_new = 60000", "list_max_new = 8"))
	res := g.run(j)
	require.Nil(t, res.Err, "something was listed: Err nil and partial")
	require.Equal(t, 8, g.opts.puts, "no more than list_max_new contracts are written")
	require.Len(t, g.lst.only("list:"), 2)
	require.Equal(t, reasonMaxNew, g.row("AAA").Reason)
	require.Equal(t, reasonMaxNew, g.row("BBB").Reason)
	require.Equal(t, 1, g.row("AAA").Listed)
	require.True(t, g.present(j))
	g.lst.reset()
	require.Nil(t, g.run(j).Err, "a re-run has its own count")
	require.Equal(t, 16, g.opts.puts, "8 more: the cap counts per run")
}

func TestAdjustedContractOfAnotherMultiplierIsNotWritten(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{29}
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return append(g.lst.sample(class, ""), g.lst.sample(class, "10", "95")...), nil
	}
	require.Nil(t, g.run(g.job(nil)).Err)
	refs, _ := g.opts.ListOptions(context.Background(), g.id("AAA"))
	require.Len(t, refs, 6)
	for _, r := range refs {
		require.Equal(t, "100", r.Multiplier)
	}
}

func TestShortKeepsContractsAndASecondShortOnALaterSessionIsStored(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{29}
	id := g.id("AAA")
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return g.lst.sample(class, "", "100"), nil
	}
	j := g.job(nil)
	seedContracts(t, g, id, "AAA", g.exp(29), 10)
	first := g.exp(29)
	require.NoError(t, g.ls.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: id, Expiry: first, Class: "AAA",
		ListedSession: "2026-11-10", Outcome: "listed", Count: 10}}))
	require.NotNil(t, g.run(j).Err)
	rows, _ := g.ls.Lists(context.Background(), id)
	require.Equal(t, "short", rows[0].Outcome)
	require.Equal(t, contracts.Day("2026-11-10"), rows[0].ListedSession, "a short keeps the session it was listed for")
	require.Equal(t, 10, g.opts.count(id))
	tried := rows[0].TriedAt

	g2 := *g
	g2.setDay("2026-11-13")
	g2.offs = map[string][]int{"AAA": {28}}
	require.NotNil(t, g2.run(g2.job(nil)).Err)
	rows, _ = g.ls.Lists(context.Background(), id)
	var second openquote.ListRecord
	for _, r := range rows {
		if r.Expiry == g2.exp(28) {
			second = r
		}
	}
	require.Equal(t, "short", second.Outcome)
	require.True(t, second.TriedAt.After(tried))
}

func TestAnswerWithOneRightIsError(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{29}
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		var calls []broker.Contract
		for _, c := range g.lst.sample(class, "") {
			if c.Right == "C" {
				calls = append(calls, c)
			}
		}
		return calls, nil
	}
	require.NotNil(t, g.run(g.job(nil)).Err)
	rows, _ := g.ls.Lists(context.Background(), g.id("AAA"))
	require.Equal(t, "error", rows[0].Outcome)
	require.Zero(t, g.opts.puts)
	require.Equal(t, reasonAllShort, g.row("AAA").Reason)
}
