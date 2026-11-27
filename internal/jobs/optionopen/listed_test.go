package optionopen

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"
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
	// parallel, when above 0, replaces parallel of the manifest.
	parallel int
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
		Watch: watchOf(symbols...), Instruments: r.inst, Bars: r.bars, Options: r.opts, Accounts: r.acct, HeldAliases: ownerAliases,
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
	if r.parallel > 0 {
		j.(*resolveJob).m.Parallel = r.parallel
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
	r.parallel = 1 // one worker: the second timeout stops the run before a third ask
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
		r.parallel = 1 // one worker: the second timeout stops the run before a third ask
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

func TestGapPassResolvesOnlyWhatTheCacheDoesNotHold(t *testing.T) {
	t.Parallel()
	first := oneChain()
	first.Strikes = slices.DeleteFunc(slices.Clone(first.Strikes), func(k string) bool { return k == "100.0" })
	r := newRig(first)
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
	r.brk.chains = []broker.Chain{oneChain()}
	r.brk.answer = nil
	r.run(t, NewGap, ResolveOptions{}, "AAPL")
	sawNew := false
	for _, c := range r.brk.asked[done:] {
		if cached[c.Strike+c.Right] {
			t.Fatalf("asked a contract the cache holds: %+v", c)
		}
		sawNew = sawNew || c.Strike == "100.0"
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

func TestMarkSurvivesTheSameStrikeAtAnotherExpiry(t *testing.T) {
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
	if len(r.brk.asked) != first {
		t.Fatalf("a strike stored at another expiry says nothing about this one: %d asked again", len(r.brk.asked)-first)
	}
}

func TestMarkSurvivesTheChainListingTheStrike(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) { return broker.Resolved{}, nil }
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	first, marked := len(r.brk.asked), markedCount(r)
	if marked == 0 {
		t.Fatal("the first run wrote no marks")
	}
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	r.run(t, NewGap, ResolveOptions{}, "AAPL")
	if len(r.brk.asked) != first || markedCount(r) != marked {
		t.Fatalf("neither pass asks a marked contract the chain lists: asked %d more, marks %d of %d",
			len(r.brk.asked)-first, markedCount(r), marked)
	}
	// The chain gains strikes after checked_at: the marked ones are still not asked.
	r.clk.Advance(24 * time.Hour)
	r.bars.closes["2026-11-11"] = "100"
	r.brk.chains = []broker.Chain{chainOf("AAPL", 900, 1001, expiry30)}
	before := map[string]bool{}
	for _, c := range r.brk.asked {
		before[c.Strike+c.Right] = true
	}
	j, _ := NewGap(r.deps("AAPL"), ResolveOptions{})
	keys, _, _ := j.Plan(context.Background(), "2026-11-11")
	j.Run(context.Background(), "2026-11-11", keys)
	for _, c := range r.brk.asked[first:] {
		if before[c.Strike+c.Right] {
			t.Fatalf("a marked contract is asked again: %+v", c)
		}
	}
}

func TestNoCapAsksEveryCandidate(t *testing.T) {
	t.Parallel()
	// 100 trading classes of 100 contracts each: 10,000 candidates.
	var chains []broker.Chain
	for i := range 100 {
		chains = append(chains, chainOf("C"+strconv.Itoa(i), 904, 1001, expiry30))
	}
	r := newRig(chains...)
	res := r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if res.Err != nil || len(r.brk.asked) != 10000 || r.opts.puts != 10000 {
		t.Fatalf("asked %d, stored %d, want 10000; err=%v", len(r.brk.asked), r.opts.puts, res.Err)
	}
	if !strings.Contains(r.logs[0], "deferred=0") {
		t.Fatalf("nothing is deferred: %s", r.logs[0])
	}
}

func TestResolveAsksInParallelAndWritesInOrder(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	cur, peak := 0, 0
	gate := make(chan struct{})
	var once sync.Once
	slow := newRig(oneChain())
	slow.parallel = 5
	slow.brk.hold = func() {
		mu.Lock()
		cur++
		peak = max(peak, cur)
		if cur >= 5 {
			once.Do(func() { close(gate) })
		}
		mu.Unlock()
		<-gate // five calls are in flight before any answers
		mu.Lock()
		cur--
		mu.Unlock()
	}
	res := slow.run(t, NewResolve, ResolveOptions{}, "AAPL")
	one := newRig(oneChain())
	one.parallel = 1
	one.run(t, NewResolve, ResolveOptions{}, "AAPL")
	if res.Err != nil || peak != 5 {
		t.Fatalf("up to 5 requests in flight, got %d; err=%v", peak, res.Err)
	}
	stored := map[string]int{}
	for _, ref := range slow.opts.refs[slow.opts.refs[1][0].UnderlyingID] {
		stored[refKey(ref.Expiry, ref.Strike, ref.Right, ref.TradingClass)]++
	}
	for k, n := range stored {
		if n != 1 {
			t.Fatalf("%s stored %d times", k, n)
		}
	}
	if len(stored) != len(slow.brk.asked) || slow.logs[0] != one.logs[0] {
		t.Fatalf("every answer stored once and the same counters as one worker:\n%s\n%s", slow.logs[0], one.logs[0])
	}
}

func TestParallelRunStopsAfterTwoConsecutiveTimeouts(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.brk.answer = func(int, broker.Contract) (broker.Resolved, error) {
		return broker.Resolved{}, &typedError{kind: contracts.Retryable, err: context.DeadlineExceeded}
	}
	res := r.run(t, NewResolve, ResolveOptions{}, "AAPL") // the manifest's 5 workers
	if n := len(r.brk.asked); n < 2 || n > 2+4 {
		t.Fatalf("the run stops once two timeouts are seen, with at most the others in flight; asked %d", n)
	}
	if res.Err == nil || res.Err.Kind() != contracts.Retryable || strings.Contains(r.logs[0], "unsent=0") {
		t.Fatalf("the rest is counted unsent: %v %s", res.Err, r.logs[0])
	}
}

func TestResolveSummaryCarriesGridSkipped(t *testing.T) {
	t.Parallel()
	// The stored grid of the expiry is 0.5; the chain lists every 0.1.
	coarse := oneChain()
	coarse.Strikes = slices.DeleteFunc(slices.Clone(coarse.Strikes), func(k string) bool { return !strings.HasSuffix(k, ".0") && !strings.HasSuffix(k, ".5") })
	r := newRig(coarse)
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	r.brk.chains = []broker.Chain{oneChain()}
	n := len(r.brk.asked)
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	line := r.logs[1]
	var skipped int
	_, after, found := strings.Cut(line, "grid_skipped=")
	if !found {
		t.Fatalf("no grid_skipped in %s", line)
	}
	skipped, _ = strconv.Atoi(strings.Fields(after)[0])
	if skipped == 0 {
		t.Fatalf("the strikes off the grid are counted: %s", line)
	}
	for _, c := range r.brk.asked[n:] {
		if v := strikeValue(c.Strike) * 2; v != float64(int(v)) {
			t.Fatalf("strike %s is off the grid of 0.5", c.Strike)
		}
	}
}

func TestStoredContractOfAnEarlierDayIsNotAsked(t *testing.T) {
	t.Parallel()
	r := newRig(oneChain())
	r.run(t, NewResolve, ResolveOptions{}, "AAPL")
	first := len(r.brk.asked)
	if first == 0 {
		t.Fatal("the first run asked nothing")
	}
	for _, d := range []string{"2026-11-11", "2026-11-25"} {
		r.clk.Advance(24 * time.Hour)
		r.bars.closes[d] = "100"
		j, _ := NewResolve(r.deps("AAPL"), ResolveOptions{})
		keys, _, _ := j.Plan(context.Background(), contracts.Day(d))
		j.Run(context.Background(), contracts.Day(d), keys)
		if len(r.brk.asked) != first {
			t.Fatalf("day %s asked %d stored contracts again", d, len(r.brk.asked)-first)
		}
	}
}
