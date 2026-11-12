package optionopen

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

const (
	runDay      = contracts.Day("2026-11-10")
	expiry30    = "20261210" // 30 days after runDay
	expiry30Day = contracts.Day("2026-12-10")
)

type rig struct {
	clk   *fakeClock
	brk   *fakeResolver
	opts  *fakeOptions
	marks *fakeMarks
	inst  *fakeInstruments
	bars  fakeBars
	pull  fakePull
	acct  fakeAccounts
	logs  []string
}

func newRig(chains ...broker.Chain) *rig {
	return &rig{
		clk:   &fakeClock{t: time.Date(2026, 11, 10, 22, 0, 0, 0, time.UTC)},
		brk:   &fakeResolver{chains: chains},
		opts:  &fakeOptions{},
		marks: &fakeMarks{},
		inst:  &fakeInstruments{},
		bars:  fakeBars{closes: map[string]string{"2026-11-10": "100"}},
	}
}

func (r *rig) deps(symbols ...string) ResolveDeps {
	return ResolveDeps{
		Stored: Stored{Watch: watchOf(symbols...), Instruments: r.inst, Bars: r.bars, Options: r.opts, Accounts: r.acct},
		Broker: r.brk, Marks: r.marks, Pull: r.pull, Clock: r.clk,
		Log:  func(l string) { r.logs = append(r.logs, l) },
		Rand: func() float64 { return 0.5 },
	}
}

func (r *rig) run(t *testing.T, build func(ResolveDeps, ResolveOptions) (contracts.Job, error), o ResolveOptions, symbols ...string) contracts.JobResult {
	t.Helper()
	j, err := build(r.deps(symbols...), o)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	return j.Run(context.Background(), runDay, keys)
}

// oneChain has a few strikes around 100 so that a handful of contracts are asked.
func oneChain() broker.Chain { return chainOf("AAPL", 990, 1010, expiry30) }

func TestEmptyListWritesMarkWithRetryInsideSevenPlusMinusTwoDays(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		u    float64
		want time.Duration
	}{{0, 5 * 24 * time.Hour}, {0.5, 7 * 24 * time.Hour}, {0.999999, 9 * 24 * time.Hour}} {
		r := newRig(oneChain())
		r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) { return broker.Resolved{}, nil }
		d := r.deps("AAPL")
		d.Rand = func() float64 { return tc.u }
		j, _ := NewResolve(d, ResolveOptions{})
		keys, _, _ := j.Plan(context.Background(), runDay)
		j.Run(context.Background(), runDay, keys)
		if len(r.marks.marks) == 0 {
			t.Fatal("no mark for an empty list")
		}
		m := r.marks.marks[0]
		got := m.RetryAt.Sub(m.CheckedAt)
		if diff := got - tc.want; diff < -time.Minute || diff > time.Minute {
			t.Fatalf("u=%v: retry after %v, want about %v", tc.u, got, tc.want)
		}
		if m.ErrorCode != nil || m.CheckedAt != r.clk.Now() {
			t.Fatalf("an empty list has no error code and the clock's checked_at: %+v", m)
		}
		if r.opts.puts != 0 {
			t.Fatal("a not listed contract must not be stored as a contract")
		}
	}
}

func TestClean200NoSecurityDefinitionIsMarked(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) {
		return broker.Resolved{ErrorCode: 200, ErrorText: "No security definition has been found for the request"}, nil
	}
	res := r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if res.Err != nil || len(r.marks.marks) == 0 || len(r.marks.marks) != len(r.brk.asked) {
		t.Fatalf("every asked contract is marked: marks=%d asked=%d err=%v", len(r.marks.marks), len(r.brk.asked), res.Err)
	}
	if m := r.marks.marks[0]; m.ErrorCode == nil || *m.ErrorCode != "200" {
		t.Fatalf("error code of the answer is kept: %+v", m)
	}
}

func TestOther200IsAmbiguousAndNeverMarked(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) {
		return broker.Resolved{ErrorCode: 200, ErrorText: "Ambiguous contract: several results"}, nil
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.marks.marks) != 0 || r.opts.puts != 0 {
		t.Fatalf("an ambiguous answer writes nothing: marks=%d", len(r.marks.marks))
	}
	if !strings.Contains(r.logs[0], "ambiguous=") || strings.Contains(r.logs[0], "ambiguous=0") {
		t.Fatalf("summary counts ambiguous: %s", r.logs[0])
	}
}

func TestAResultThatDiffersFromTheRequestIsAmbiguous(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(_ int, c broker.Contract) (broker.Resolved, error) {
		c.ConID, c.Strike = 5, "1"
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if r.opts.puts != 0 || len(r.marks.marks) != 0 {
		t.Fatal("a differing result is neither stored nor marked")
	}
}

func TestTimeoutNeverWritesAMarkAndTwoStopTheRun(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) {
		return broker.Resolved{}, &typedError{kind: contracts.Retryable, err: context.DeadlineExceeded}
	}
	res := r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.marks.marks) != 0 {
		t.Fatal("a timeout writes no mark")
	}
	if len(r.brk.asked) != 2 {
		t.Fatalf("two consecutive timeouts stop the resolves; asked %d", len(r.brk.asked))
	}
	if res.Err == nil || res.Err.Kind() != contracts.Retryable {
		t.Fatalf("the unsent rest is a retryable gap, got %v", res.Err)
	}
	if !strings.Contains(r.logs[0], "timeout=2") || strings.Contains(r.logs[0], "unsent=0") {
		t.Fatalf("summary counts timeouts and the unsent: %s", r.logs[0])
	}
}

func TestDisconnectCodesNeverMark(t *testing.T) {
	t.Parallel()
	for _, code := range []int{10197, 1100, 1101, 1102} {
		r := newRig(oneChain())
		r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) {
			return broker.Resolved{ErrorCode: code, ErrorText: "not connected"}, nil
		}
		r.run(t, NewResolve, ResolveOptions{}, "AAPL")
		if len(r.marks.marks) != 0 || len(r.brk.asked) != 2 {
			t.Fatalf("code %d: marks=%d asked=%d", code, len(r.marks.marks), len(r.brk.asked))
		}
	}
}

func TestPacingLineAndSubscriptionCodesNeverMark(t *testing.T) {
	t.Parallel()
	for _, code := range []int{101, 100, 162, 354, 10089, 10090, 10091, 999} {
		r := newRig(oneChain())
		r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) {
			return broker.Resolved{ErrorCode: code, ErrorText: "no security definition"}, nil
		}
		r.run(t, NewResolve, ResolveOptions{}, "AAPL")
		if len(r.marks.marks) != 0 {
			t.Fatalf("code %d wrote a mark", code)
		}
	}
}

func TestListedContractGoesToTheOptionContractStore(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	res := r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if res.Err != nil || r.opts.puts == 0 || r.opts.puts != len(r.brk.asked) {
		t.Fatalf("puts=%d asked=%d err=%v", r.opts.puts, len(r.brk.asked), res.Err)
	}
	ref := r.opts.refs[r.opts.refs[1][0].UnderlyingID][0]
	if ref.Expiry != expiry30Day || ref.TradingClass != "AAPL" || ref.InstrumentID == 0 {
		t.Fatalf("stored ref: %+v", ref)
	}
	// A second run finds the cache full and asks nothing.
	before := len(r.brk.asked)
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.brk.asked) != before {
		t.Fatal("a cached contract is not asked again")
	}
}

func TestMarkedContractIsNotAskedBeforeItsRetryAt(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) { return broker.Resolved{}, nil }
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	first := len(r.brk.asked)
	r.clk.Advance(24 * time.Hour)
	r.bars.closes["2026-11-11"] = "100"
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.brk.asked) != first {
		t.Fatalf("a mark inside its validity is not asked again: %d then %d", first, len(r.brk.asked))
	}
	r.clk.Advance(10 * 24 * time.Hour) // past 7 + 2 days
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.brk.asked) != 2*first {
		t.Fatalf("after retry_at the contracts are asked again: %d", len(r.brk.asked))
	}
}

func TestSpotMoveOfFivePercentAsksAgain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		close string
		again bool
	}{{"104", false}, {"105", true}} {
		r := newRig(oneChain())
		r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) { return broker.Resolved{}, nil }
		r.run(t, NewResolve, ResolveOptions{}, "AAPL")
		first := len(r.brk.asked)
		r.clk.Advance(24 * time.Hour)
		r.bars.closes["2026-11-11"] = tc.close
		j, _ := NewResolve(r.deps("AAPL"), ResolveOptions{})
		keys, _, _ := j.Plan(context.Background(), "2026-11-11")
		before := map[string]bool{}
		for _, c := range r.brk.asked[:first] {
			before[c.Strike+c.Right] = true
		}
		j.Run(context.Background(), "2026-11-11", keys)
		again := false
		for _, c := range r.brk.asked[first:] {
			again = again || before[c.Strike+c.Right]
		}
		if again != tc.again {
			t.Fatalf("close %s: marked contracts asked again = %v, want %v", tc.close, again, tc.again)
		}
	}
}

func pullRow(r *rig, ref openquote.OptionRef, status int, code *string) contracts.OptionOpen {
	return contracts.OptionOpen{InstrumentID: int(ref.InstrumentID), UnderlyingID: int(ref.UnderlyingID), Status: status, ErrorCode: code}
}

func TestNotListedRowOfThePullBecomesAMarkOnlyAfterACleanAnswer(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.run(t, NewResolve, ResolveOptions{}, "AAPL") // fills the cache
	refs := r.opts.refs[r.opts.refs[1][0].UnderlyingID]
	clean, dirty := "200", "354"
	r.pull = fakePull{rows: []contracts.OptionOpen{
		pullRow(r, refs[0], 4, &clean), pullRow(r, refs[1], 4, nil), pullRow(r, refs[2], 4, &dirty), pullRow(r, refs[3], 1, nil),
	}}
	asked := len(r.brk.asked)
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.marks.marks) != 2 {
		t.Fatalf("two clean not_listed rows become marks, got %d", len(r.marks.marks))
	}
	if len(r.brk.asked) != asked {
		t.Fatal("a mark from the pull asks the broker nothing")
	}
	if !strings.Contains(r.logs[len(r.logs)-1], "from_pull=2") {
		t.Fatalf("summary counts the marks of the pull: %s", r.logs[len(r.logs)-1])
	}
}

func bigChains() []broker.Chain {
	// 13 trading classes of 100 contracts each: 97 puts at or below 100, the put above, the two calls around.
	var chains []broker.Chain
	for i := 0; i < 13; i++ {
		chains = append(chains, chainOf("C"+string(rune('A'+i)), 904, 1001, expiry30))
	}
	return chains
}

func TestMaxNewOf1500ResolvesAllThirteenHundredInOneRun(t *testing.T) {
	t.Parallel()
	r := newRig(bigChains()...)
	res := r.run(t, NewResolve, ResolveOptions{MaxNew: 1500}, "AAPL")
	if res.Err != nil || r.opts.puts != 1300 {
		t.Fatalf("resolved %d, want 1300; err=%v", r.opts.puts, res.Err)
	}
	if !strings.Contains(r.logs[0], "cap=1500") || !strings.Contains(r.logs[0], "found=1300") {
		t.Fatalf("summary names the cap in force: %s", r.logs[0])
	}
}

func TestCapOf300StopsAtThreeHundredAndSaysSo(t *testing.T) {
	t.Parallel()
	r := newRig(bigChains()...)
	res := r.run(t, NewResolve, ResolveOptions{MaxNew: 300}, "AAPL")
	if r.opts.puts != 300 || len(r.brk.asked) != 300 {
		t.Fatalf("resolved %d, asked %d, want 300", r.opts.puts, len(r.brk.asked))
	}
	if res.Err != nil {
		t.Fatalf("what the cap leaves is deferred, not a failure: %v", res.Err)
	}
	if !strings.Contains(r.logs[0], "cap=300") || !strings.Contains(r.logs[0], "deferred=1000") {
		t.Fatalf("summary names the cap and the deferred: %s", r.logs[0])
	}
}

func TestTheNearestStrikesAreResolvedFirstWhenTheCapBites(t *testing.T) {
	t.Parallel()
	r := newRig(bigChains()...)
	r.run(t, NewResolve, ResolveOptions{MaxNew: 10}, "AAPL")
	for _, c := range r.brk.asked {
		if v := strikeValue(c.Strike); v < 99.9 || v > 100.1 {
			t.Fatalf("strike %s is not among the nearest to the spot", c.Strike)
		}
	}
}

func TestGapPassResolvesOnlyWhatTheCacheDoesNotHold(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	cached := map[string]bool{}
	r.brk.answer = func(n int, c broker.Contract) (broker.Resolved, error) {
		if n%2 == 0 {
			return broker.Resolved{}, nil // marked
		}
		cached[c.Strike+c.Right] = true
		c.ConID = int64(5000 + n)
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	done := len(r.brk.asked)
	// IBKR lists a new strike overnight.
	ch := oneChain()
	ch.Strikes = append(ch.Strikes, "99.55")
	r.brk.chains = []broker.Chain{ch}
	r.brk.answer = nil
	r.run(t, NewGap, ResolveOptions{}, "AAPL")
	sawNew := false
	for _, c := range r.brk.asked[done:] {
		if cached[c.Strike+c.Right] {
			t.Fatalf("asked a contract the cache holds: %+v", c)
		}
		sawNew = sawNew || c.Strike == "99.55"
	}
	if !sawNew {
		t.Fatal("the gap pass does not ask the new strike")
	}
	if r.brk.listed != 2 {
		t.Fatalf("each pass lists the chain first, listed %d", r.brk.listed)
	}
}

// markedCount is how many marks the rig's store holds.
func markedCount(r *rig) int {
	got, _ := r.marks.NotListed(context.Background(), r.clk.Now())
	return len(got)
}

func TestMorningChainListingAMarkedStrikeEndsItsMarkInTheGapPassOnly(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) { return broker.Resolved{}, nil }
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	first, marked := len(r.brk.asked), markedCount(r)
	if marked == 0 {
		t.Fatal("the first run wrote no marks")
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if len(r.brk.asked) != first {
		t.Fatalf("the evening pass keeps the marks, asked %d more", len(r.brk.asked)-first)
	}
	r.run(t, NewGap, ResolveOptions{}, "AAPL")
	if len(r.brk.asked) != first+marked {
		t.Fatalf("the gap pass asks the %d marked contracts the morning chain lists, asked %d", marked, len(r.brk.asked)-first)
	}
}

func TestStrikeListedAtAnotherExpiryEndsItsMark(t *testing.T) {
	t.Parallel()
	const later = "20261218"
	r := newRig(chainOf("AAPL", 990, 1010, expiry30, later))
	r.brk.answer = func(n int, c broker.Contract) (broker.Resolved, error) {
		if c.Expiry == later && c.Strike == "100.0" {
			return broker.Resolved{}, nil // not listed at the later expiry
		}
		c.ConID = int64(7000 + n)
		return broker.Resolved{Listed: true, Contract: c}, nil
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if markedCount(r) == 0 {
		t.Fatal("the first run marked nothing at the later expiry")
	}
	first := len(r.brk.asked)
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	again := 0
	for _, c := range r.brk.asked[first:] {
		if c.Expiry == later && c.Strike == "100.0" {
			again++
		}
	}
	if again == 0 {
		t.Fatal("a strike stored listed at another expiry must end its mark and be asked again (Decision 5 trigger b)")
	}
}

func TestGapPassReadsNoPullRows(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	refs := r.opts.refs[r.opts.refs[1][0].UnderlyingID]
	r.pull = fakePull{rows: []contracts.OptionOpen{pullRow(r, refs[0], 4, nil)}}
	r.run(t, NewGap, ResolveOptions{}, "AAPL")
	if len(r.marks.marks) != 0 {
		t.Fatal("the evening resolve is the one that turns pull rows into marks")
	}
}

func TestHeldOptionContractIsResolvedWhateverItsExpiry(t *testing.T) {
	t.Parallel()
	r := newRig(chainOf("AAPL", 990, 1010, expiry30, "20270219"))
	r.acct = fakeAccounts{pos: []store.Position{{SecType: "option", Symbol: "AAPL", Expiry: "2027-02-19", Strike: "95.0", Right: "put", Quantity: "-1",
		Multiplier: "100"}}}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	found := false
	for _, c := range r.brk.asked {
		found = found || (c.Right == "P" && c.Strike == "95.0")
	}
	if !found {
		t.Fatal("a held contract outside the DTE window is asked")
	}
}

func TestChainErrorIsRetryableAndMarksNothing(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	d := r.deps("AAPL")
	d.Broker = errChains{r.brk}
	j, _ := NewResolve(d, ResolveOptions{})
	keys, _, _ := j.Plan(context.Background(), runDay)
	res := j.Run(context.Background(), runDay, keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || len(r.marks.marks) != 0 {
		t.Fatalf("err=%v", res.Err)
	}
}

type errChains struct{ *fakeResolver }

func (errChains) ListChains(context.Context, broker.Contract) ([]broker.Chain, error) {
	return nil, errors.New("gateway down")
}

func TestResolveSlotValues(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	for _, tc := range []struct {
		build  func(ResolveDeps, ResolveOptions) (contracts.Job, error)
		anchor string
		off    time.Duration
		id     string
	}{
		{NewResolve, "session_close", 60 * time.Minute, "option_open_resolve"},
		{NewGap, "session_open", -90 * time.Minute, "option_open_resolve_gap"},
	} {
		j, err := tc.build(r.deps("AAPL"), ResolveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		cal, anchor, _, off := j.(interface {
			Schedule() (string, string, string, time.Duration)
		}).Schedule()
		if j.ID() != tc.id || cal != "XNYS" || anchor != tc.anchor || off != tc.off {
			t.Fatalf("%s: %s %s %v", j.ID(), cal, anchor, off)
		}
	}
}

func TestStrikeNewlyInTheChainEndsItsMark(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) { return broker.Resolved{}, nil }
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	first := len(r.brk.asked)
	r.clk.Advance(24 * time.Hour)
	r.bars.closes["2026-11-11"] = "100"
	r.brk.chains = []broker.Chain{chainOf("AAPL", 900, 1001, expiry30)} // the chain gained strikes after checked_at
	j, _ := NewGap(r.deps("AAPL"), ResolveOptions{})
	keys, _, _ := j.Plan(context.Background(), "2026-11-11")
	j.Run(context.Background(), "2026-11-11", keys)
	if len(r.brk.asked) == first {
		t.Fatal("a strike newly listed in the chain must end its mark and be asked again (Decision 5 trigger c)")
	}
}

func TestListedAnswerWithoutConIDIsAmbiguousAndNeverMarked(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(_ int, c broker.Contract) (broker.Resolved, error) {
		return broker.Resolved{Listed: true, Contract: c}, nil // ConID 0
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if n := markedCount(r); n != 0 {
		t.Fatalf("a listed answer with no con_id wrote %d marks", n)
	}
}

// noConIDRun runs the resolve job over a watch list whose stock has ConID 0.
func noConIDRun(t *testing.T) (*rig, contracts.JobResult) {
	t.Helper()
	r := newRig(oneChain())
	d := r.deps("AAPL")
	for i := range d.Watch {
		d.Watch[i].Contract.ConID = 0
	}
	j, err := NewResolve(d, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := j.Plan(context.Background(), runDay)
	if err != nil {
		t.Fatal(err)
	}
	return r, j.Run(context.Background(), runDay, keys)
}

func TestResolveNoConIDIsKeyErr(t *testing.T) {
	t.Parallel()
	r, res := noConIDRun(t)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || !strings.Contains(res.Err.Error(), "no con_id of AAPL") {
		t.Fatalf("err=%v", res.Err)
	}
	if len(r.marks.marks) != 0 {
		t.Fatalf("marks=%v", r.marks.marks)
	}
}

func TestResolveSummaryCarriesRunErr(t *testing.T) {
	t.Parallel()
	r, _ := noConIDRun(t)
	var last string
	for _, l := range r.logs {
		if strings.Contains(l, "run_err=") {
			last = l
		}
	}
	if !strings.Contains(last, "asked=0") || !strings.Contains(last, "run_err=retryable: ") || strings.Contains(last, "run_err=none") {
		t.Fatalf("line=%q", last)
	}
}

func TestResolveSummaryRunErrNone(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if last := r.logs[len(r.logs)-1]; !strings.HasSuffix(last, " run_err=none") {
		t.Fatalf("line=%q", last)
	}
}

func TestResolveMissingCloseAsksTheGatewayOnce(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.bars = fakeBars{}
	h := &fakeHistory{err: errors.New("gateway down")}
	d := r.deps("AAPL")
	d.History = h
	j, err := NewResolve(d, ResolveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	keys, _, _ := j.Plan(context.Background(), runDay)
	res := j.Run(context.Background(), runDay, keys)
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || h.asked != 1 {
		t.Fatalf("err=%v asked=%d", res.Err, h.asked)
	}
	var fault bool
	for _, l := range r.logs {
		fault = fault || strings.HasPrefix(l, "fault stage=option_open_resolve day=2026-11-10 key=AAPL reason=")
	}
	if !fault {
		t.Fatalf("logs %q", r.logs)
	}
	h.err = nil
	if res := j.Run(context.Background(), runDay, keys); res.Err == nil || res.Err.Kind() != contracts.Permanent {
		t.Fatalf("a good fetch with no bar: %v", res.Err)
	}
	h.bars = []broker.Bar{{Close: "100"}}
	if res := j.Run(context.Background(), runDay, keys); res.Err != nil {
		t.Fatalf("fetched close: %v", res.Err)
	}
}

func TestHeldOptionRowIsKeptAsHeldCandidate(t *testing.T) {
	t.Parallel()
	chains := []broker.Chain{{Expirations: []string{"20270219"}, Multiplier: "100"}}
	count := func(qty string) int {
		r := &keyRun{st: stock{Symbol: "AAPL"}, pos: []store.Position{{SecType: "option", Symbol: "AAPL", Expiry: "2027-02-19",
			Strike: "95.0", Right: "put", Quantity: qty, Multiplier: "100"}}}
		n := 0
		r.heldCandidates(chains, func(broker.Chain, string, string, string, bool) candidate { return candidate{} }, func(candidate) { n++ })
		return n
	}
	if count("-1") != 1 || count("0") != 0 {
		t.Fatalf("held row candidates: qty -1 → %d, qty 0 → %d", count("-1"), count("0"))
	}
}
