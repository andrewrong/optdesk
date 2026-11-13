package optionopen

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/openquote"
)

// fakeListStore is an openquote.OptionListStore that keeps the rules of the real one: a short or error row keeps
// ListedAt and ListedSession, a non-empty Expected is never replaced, an empty one only by a non-empty one.
type fakeListStore struct {
	mu    sync.Mutex
	lists map[int64]map[string]openquote.ListRecord
	runs  map[int64]openquote.ListRunRecord
	puts  int
}

func (f *fakeListStore) PutLists(_ context.Context, rows []openquote.ListRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lists == nil {
		f.lists = map[int64]map[string]openquote.ListRecord{}
	}
	for _, r := range rows {
		if f.lists[r.UnderlyingID] == nil {
			f.lists[r.UnderlyingID] = map[string]openquote.ListRecord{}
		}
		k := string(r.Expiry) + "|" + r.Class
		if old, ok := f.lists[r.UnderlyingID][k]; ok && r.Outcome != "listed" {
			r.ListedAt, r.ListedSession, r.Count = old.ListedAt, old.ListedSession, old.Count
		}
		f.lists[r.UnderlyingID][k] = r
	}
	return nil
}

func (f *fakeListStore) Lists(_ context.Context, id int64) ([]openquote.ListRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []openquote.ListRecord
	for _, r := range f.lists[id] {
		out = append(out, r)
	}
	sort.Slice(out, func(a, b int) bool {
		return out[a].Expiry+contracts.Day(out[a].Class) < out[b].Expiry+contracts.Day(out[b].Class)
	})
	return out, nil
}

func (f *fakeListStore) PutListRuns(_ context.Context, rows []openquote.ListRunRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.runs == nil {
		f.runs = map[int64]openquote.ListRunRecord{}
	}
	for _, r := range rows {
		if old, ok := f.runs[r.UnderlyingID]; ok && len(old.Expected) > 0 {
			r.Expected = old.Expected
		} else if ok && old.Expected != nil && len(r.Expected) == 0 {
			r.Expected = old.Expected
		}
		f.runs[r.UnderlyingID] = r
		f.puts++
	}
	return nil
}

func (f *fakeListStore) ListRuns(_ context.Context, session contracts.Day) ([]openquote.ListRunRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []openquote.ListRunRecord
	for _, r := range f.runs {
		if r.Session == session {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].UnderlyingID < out[b].UnderlyingID })
	return out, nil
}

// dedupOptions is an OptionContractStore whose key is refKey, as the table's key is.
type dedupOptions struct {
	mu   sync.Mutex
	refs map[int64]map[string]openquote.OptionRef
	puts int
}

func (f *dedupOptions) PutOption(_ context.Context, r openquote.OptionRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.refs == nil {
		f.refs = map[int64]map[string]openquote.OptionRef{}
	}
	if f.refs[r.UnderlyingID] == nil {
		f.refs[r.UnderlyingID] = map[string]openquote.OptionRef{}
	}
	f.refs[r.UnderlyingID][refKey(r.Expiry, r.Strike, r.Right, r.TradingClass)] = r
	f.puts++
	return nil
}

func (f *dedupOptions) ListOptions(_ context.Context, id int64) ([]openquote.OptionRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []openquote.OptionRef
	for _, r := range f.refs[id] {
		out = append(out, r)
	}
	return out, nil
}

func (f *dedupOptions) count(id int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.refs[id])
}

// fakeLister answers ListChains and ListContracts, advances the clock by dur per call (25 s for a timeout) and
// records the calls and the most calls in flight.
type fakeLister struct {
	clk    *fakeClock
	dur    time.Duration
	chains func(sym string) ([]broker.Chain, error)
	list   func(n int, sym string, expiry contracts.Day, class string) ([]broker.Contract, error)

	mu       sync.Mutex
	calls    []string
	lists    int
	conID    int64
	inflight int
	peak     int
}

func (f *fakeLister) enter(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
	if f.inflight++; f.inflight > f.peak {
		f.peak = f.inflight
	}
}

func (f *fakeLister) leave(err error) {
	if err != nil && strings.Contains(err.Error(), "deadline") {
		f.clk.Advance(25 * time.Second)
	} else {
		f.clk.Advance(f.dur)
	}
	f.mu.Lock()
	f.inflight--
	f.mu.Unlock()
}

func (f *fakeLister) ListChains(_ context.Context, c broker.Contract) (chs []broker.Chain, err error) {
	f.enter("chains:" + c.Symbol)
	defer func() { f.leave(err) }()
	return f.chains(c.Symbol)
}

func (f *fakeLister) ListContracts(_ context.Context, u broker.Contract, expiry contracts.Day, right, class string) (got []broker.Contract, err error) {
	f.mu.Lock()
	n := f.lists
	f.lists++
	f.mu.Unlock()
	f.enter(fmt.Sprintf("list:%s@%s", u.Symbol, expiry))
	defer func() { f.leave(err) }()
	if right != "" {
		return nil, fmt.Errorf("the job asks both rights")
	}
	if f.list != nil {
		return f.list(n, u.Symbol, expiry, class)
	}
	return f.sample(class, ""), nil
}

// sample is a list of the class: strikes 100, 105 and 110, both rights; strikes override the strike texts.
func (f *fakeLister) sample(class string, mult string, strikes ...string) []broker.Contract {
	if len(strikes) == 0 {
		strikes = []string{"100", "105", "110"}
	}
	if mult == "" {
		mult = "100"
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []broker.Contract
	for _, s := range strikes {
		for _, r := range []string{"C", "P"} {
			f.conID++
			out = append(out, broker.Contract{ConID: 5000 + f.conID, Strike: s, Right: r, Multiplier: mult, TradingClass: class})
		}
	}
	return out
}

func (f *fakeLister) only(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			out = append(out, c)
		}
	}
	return out
}

func (f *fakeLister) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls, f.lists = nil, 0
}

type countingAccounts struct {
	fakeAccounts
	mu    sync.Mutex
	calls map[string]int
}

func (c *countingAccounts) Positions(ctx context.Context, alias, day string) ([]store.Position, error) {
	c.mu.Lock()
	c.calls[alias]++
	c.mu.Unlock()
	return c.fakeAccounts.Positions(ctx, alias, day)
}

// lrig is one job with its fakes; the clock stands at 06:15 ET of the day.
type lrig struct {
	t      *testing.T
	day    contracts.Day
	clk    *fakeClock
	lst    *fakeLister
	ls     *fakeListStore
	opts   *dedupOptions
	ins    *fakeInstruments
	acc    *countingAccounts
	watch  []string
	offs   map[string][]int // expiry offsets in days per symbol, default 22 and 29
	pauses []time.Duration
	logs   []string
	closes map[string]string // the stored closes by day; nil: 100 on the day before
	tweak  func(*ListDeps)
}

var newYork = func() *time.Location { l, _ := time.LoadLocation("America/New_York"); return l }()

func newLRig(t *testing.T, symbols ...string) *lrig {
	t.Helper()
	g := &lrig{t: t, watch: symbols, offs: map[string][]int{}, opts: &dedupOptions{}, ls: &fakeListStore{}, ins: &fakeInstruments{}}
	g.acc = &countingAccounts{calls: map[string]int{}}
	g.setDay("2026-11-12")
	g.lst = &fakeLister{clk: g.clk, dur: 5 * time.Second, chains: func(sym string) ([]broker.Chain, error) { return g.chainsOf(sym), nil }}
	return g
}

func (g *lrig) setDay(day contracts.Day) {
	g.day = day
	t, _ := dayTime(day)
	at := time.Date(t.Year(), t.Month(), t.Day(), 6, 15, 0, 0, newYork)
	if g.clk == nil {
		g.clk = &fakeClock{t: at}
		return
	}
	g.clk.Advance(at.Sub(g.clk.Now()))
}

func (g *lrig) exp(n int) contracts.Day {
	t, _ := dayTime(g.day)
	return contracts.Day(t.AddDate(0, 0, n).Format(time.DateOnly))
}

func (g *lrig) chainsOf(sym string) []broker.Chain {
	offs := g.offs[sym]
	if offs == nil {
		offs = []int{22, 29}
	}
	ch := broker.Chain{Exchange: "SMART", TradingClass: sym, Multiplier: "100"}
	for _, o := range offs {
		ch.Expirations = append(ch.Expirations, strings.ReplaceAll(string(g.exp(o)), "-", ""))
	}
	return []broker.Chain{ch}
}

// id is the instrument id of the stock the job gives symbol.
func (g *lrig) id(sym string) int64 {
	conID := 0
	for i, s := range g.watch {
		if s == sym {
			conID = 100 + i
		}
	}
	id, err := g.ins.ResolveInstrument(context.Background(), store.Identifier{Scheme: "ibkr_conid", Value: fmt.Sprint(conID)}, store.InstrumentSpec{}, nil)
	require.NoError(g.t, err)
	return id
}

func (g *lrig) deps() ListDeps {
	w := watchOf(g.watch...)
	var acct store.AccountReader
	if g.acc.fakeAccount() {
		acct = g.acc
	}
	bars := fakeBars{closes: map[string]string{string(g.exp(-1)): "100"}}
	if g.closes != nil {
		bars = fakeBars{closes: g.closes}
	}
	d := ListDeps{Stored: Stored{Watch: w, Instruments: g.ins, Options: g.opts, Accounts: acct,
		HeldAliases: ownerAliases, Bars: bars}, Lister: g.lst, ListStore: g.ls, Clock: g.clk,
		Log: func(s string) { g.logs = append(g.logs, s) },
		Pause: func(_ context.Context, d time.Duration) error {
			g.pauses = append(g.pauses, d)
			g.clk.Advance(d)
			return nil
		}}
	if g.tweak != nil {
		g.tweak(&d)
	}
	return d
}

func (c *countingAccounts) fakeAccount() bool { return len(c.pos) > 0 }

// job builds the job from data (nil: the embedded manifest).
func (g *lrig) job(data []byte) *listJob {
	g.t.Helper()
	if data == nil {
		data = listManifest
	}
	j, err := newListPreopen(data, usRules(), g.deps())
	require.NoError(g.t, err)
	return j.(*listJob)
}

func (g *lrig) run(j *listJob) contracts.JobResult {
	return j.Run(context.Background(), g.day, []contracts.ExpectedKey{{DatasetID: ListDatasetID, SubjectKey: ListSubject}})
}

func (g *lrig) row(sym string) openquote.ListRunRecord {
	rows, err := g.ls.ListRuns(context.Background(), g.day)
	require.NoError(g.t, err)
	for _, r := range rows {
		if r.UnderlyingID == g.id(sym) {
			return r
		}
	}
	g.t.Fatalf("no run row for %s", sym)
	return openquote.ListRunRecord{}
}

func (g *lrig) present(j *listJob) bool {
	ok, err := j.Postcondition(context.Background(), string(g.day), ListDatasetID, ListSubject)
	require.NoError(g.t, err)
	return ok
}

// seedRow stores a run row of sym with the groups at the offsets as expected; listed of them are listed for the day.
func (g *lrig) seed(sym string, reason string, listed int, offs ...int) {
	var exp []string
	id := g.id(sym)
	for i, o := range offs {
		exp = append(exp, string(g.exp(o))+"|"+sym)
		if i < listed {
			require.NoError(g.t, g.ls.PutLists(context.Background(), []openquote.ListRecord{{UnderlyingID: id, Expiry: g.exp(o), Class: sym,
				ListedSession: g.day, Outcome: "listed", Count: 6, BandLo: "50", BandHi: "200", Sigma: 0.3, SigmaSource: "iv", InBand: 6}}))
		}
	}
	require.NoError(g.t, g.ls.PutListRuns(context.Background(), []openquote.ListRunRecord{{UnderlyingID: id, Session: g.day, Expected: exp, Reason: reason}}))
}

func replaceKey(t *testing.T, from, to string) []byte {
	t.Helper()
	s := string(listManifest)
	require.Contains(t, s, from)
	return []byte(strings.Replace(s, from, to, 1))
}

func TestListManifestHasItsOwnParser(t *testing.T) {
	t.Parallel()
	_, err := parseManifest(listManifest)
	require.Error(t, err, "the list toml is not a manifest of the other jobs")
	m, err := parseList(listManifest)
	require.NoError(t, err)
	require.Equal(t, 45, m.BudgetMinutes)
	require.Equal(t, []windowTableDTE{{MinDTE: 21, MaxDTE: 45}}, m.Windows)
	require.Equal(t, 0.5, m.MinRatio)
	require.Equal(t, 60000, m.MaxNew)
	require.Equal(t, []int{7, 30}, []int{m.stopH, m.stopM})
	require.Equal(t, "America/New_York", m.Zone)
	require.False(t, m.Schedule.RestartSensitive)
}

func TestListJobValues(t *testing.T) {
	t.Parallel()
	j := newLRig(t, "AAA").job(nil)
	require.Equal(t, "option_list_preopen", j.ID())
	cal, open, closeOff := j.Window()
	require.Equal(t, []any{"XNYS", time.Duration(0), time.Duration(0)}, []any{cal, open, closeOff})
	require.Equal(t, 47*time.Minute, j.RunBound())
	c, anchor, zone, off := j.Schedule()
	require.Equal(t, []any{"XNYS", "session_open", "America/New_York", -195 * time.Minute}, []any{c, anchor, zone, off})
	require.False(t, j.RestartSensitive())
}

func TestListPlanIsOneKey(t *testing.T) {
	t.Parallel()
	for _, syms := range [][]string{{"AAA", "BBB"}, nil} {
		keys, hash, err := newLRig(t, syms...).job(nil).Plan(context.Background(), "2026-11-12")
		require.NoError(t, err)
		require.Equal(t, []contracts.ExpectedKey{{DatasetID: "master.option_list", SubjectKey: "preopen"}}, keys)
		require.NotEmpty(t, hash)
	}
}

func TestChooseExpiries(t *testing.T) {
	t.Parallel()
	day := contracts.Day("2026-11-12")
	at := func(offs ...int) []contracts.Day {
		t0, _ := dayTime(day)
		var out []contracts.Day
		for _, o := range offs {
			out = append(out, contracts.Day(t0.AddDate(0, 0, o).Format(time.DateOnly)))
		}
		return out
	}
	for name, c := range map[string]struct{ in, want []int }{
		"window ends inclusive":  {[]int{20, 21, 45, 46}, []int{21, 45}},
		"nearest to 30 first":    {[]int{21, 28, 29, 31, 45}, []int{29, 31, 28, 21, 45}},
		"none in the window":     {[]int{3, 10, 60}, nil},
		"a duplicate is one":     {[]int{30, 30}, []int{30}},
		"unsorted input ordered": {[]int{45, 21, 31, 29}, []int{29, 31, 21, 45}},
	} {
		got := ChooseExpiries(day, at(c.in...), 21, 45)
		require.Equal(t, at(c.want...), append([]contracts.Day(nil), got...), name)
	}
	require.Nil(t, ChooseExpiries("not a day", at(30), 21, 45))
}

func TestListJobNeedsChains(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	d := g.deps()
	d.Lister = onlyLister{}
	_, err := newListPreopen(listManifest, usRules(), d)
	require.Error(t, err)
	d.Lister = nil
	_, err = NewListPreopen(d)
	require.Error(t, err)
	_, err = NewListPreopen(g.deps())
	require.NoError(t, err)
}

type onlyLister struct{}

func (onlyLister) ListContracts(context.Context, broker.Contract, contracts.Day, string, string) ([]broker.Contract, error) {
	return nil, nil
}

func TestBandFloorTenPercent(t *testing.T) {
	t.Parallel()
	lo, hi := ListBand(100, 0.01, 30, nil, BandParams{K: 1.5, FloorPct: 0.10, MinSide: 3})
	require.InDelta(t, 90, lo, 1e-9)
	require.InDelta(t, 110, hi, 1e-9)
	lo, hi = ListBand(100, 0.50, 30, nil, BandParams{K: 1.5, FloorPct: 0.10})
	s := 0.50 * math.Sqrt(30.0/365)
	require.InDelta(t, 100*math.Exp(-1.5*s), lo, 1e-9, "a wide sigma is the band, not the floor")
	require.InDelta(t, 100*math.Exp(1.5*s), hi, 1e-9)
}

func TestMinThreeStrikesPerSide(t *testing.T) {
	t.Parallel()
	p := BandParams{K: 1.5, FloorPct: 0.10, MinSide: 3}
	lo, hi := ListBand(100, 0.01, 30, []float64{50, 60, 70, 80, 85, 95, 105, 130, 140, 150}, p)
	require.Equal(t, []float64{80, 140}, []float64{lo, hi}, "the third nearest listed strike on each side")
	lo, hi = ListBand(100, 0.01, 30, []float64{95, 105}, p)
	require.InDelta(t, 90, lo, 1e-9, "fewer listed than asked: the floor stands")
	require.InDelta(t, 110, hi, 1e-9)
}

func TestHalfStrikeIsListed(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22}
	g.lst.list = func(_ int, _ string, _ contracts.Day, class string) ([]broker.Contract, error) {
		return g.lst.sample(class, "", "97.5", "100", "102.5"), nil
	}
	require.NoError(t, errOf(g.run(g.job(nil))))
	got := map[string]bool{}
	refs, _ := g.opts.ListOptions(context.Background(), g.id("AAA"))
	for _, r := range refs {
		got[r.Strike] = true
	}
	require.Equal(t, map[string]bool{"97.5": true, "100": true, "102.5": true}, got)
}

func TestSecondWindowIsConfigOnly(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{10, 22, 60}
	data := replaceKey(t, "[window]", "[[window_dte]]\nmin_dte = 7\nmax_dte = 14\n\n[window]")
	require.NoError(t, errOf(g.run(g.job(data))))
	require.Equal(t, []string{fmt.Sprintf("list:AAA@%s", g.exp(10)), fmt.Sprintf("list:AAA@%s", g.exp(22))}, g.lst.only("list:"),
		"the expiries of both pairs are listed, 60 is in neither")
}

func TestWindowCloseIsSessionClose(t *testing.T) {
	t.Parallel()
	data := replaceKey(t, "[window]", "[[window_dte]]\nmin_dte = 7\nmax_dte = 14\n\n[window]")
	for _, d := range [][]byte{nil, data} {
		cal, open, closeOff := newLRig(t, "AAA").job(d).Window()
		require.Equal(t, []any{"XNYS", time.Duration(0), time.Duration(0)}, []any{cal, open, closeOff})
	}
}

func TestKeyReadsDatasetAndSubject(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA")
	g.offs["AAA"] = []int{22}
	data := replaceKey(t, `dataset = "master.option_list"`, `dataset = "master.option_list.jp"`)
	j := g.job(data)
	keys, _, err := j.Plan(context.Background(), g.day)
	require.NoError(t, err)
	require.Equal(t, []contracts.ExpectedKey{{DatasetID: "master.option_list.jp", SubjectKey: "preopen"}}, keys)
	res := j.Run(context.Background(), g.day, []contracts.ExpectedKey{{DatasetID: ListDatasetID, SubjectKey: ListSubject}})
	require.NotNil(t, res.Err, "the US key is not the key of this job")
	require.Equal(t, contracts.Permanent, res.Err.Kind())
	require.NoError(t, errOf(j.Run(context.Background(), g.day, keys)))
}

func TestNameFilterFromRules(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.lst.chains = func(sym string) ([]broker.Chain, error) {
		ch := g.chainsOf(sym)
		ch[0].TradingClass = "C" + sym
		return ch, nil
	}
	rules := usRules()
	all := rules.Names
	rules.Names = func(ctx context.Context, s Stored, day time.Time) ([]stock, []store.Position, error) {
		sts, pos, err := all(ctx, s, day)
		var out []stock
		for _, st := range sts {
			if st.Symbol == "BBB" {
				out = append(out, st)
			}
		}
		return out, pos, err
	}
	rules.Class = func(st stock) string { return "C" + st.Symbol }
	rules.Multiplier = "100"
	j, err := newListPreopen(listManifest, rules, g.deps())
	require.NoError(t, err)
	require.NoError(t, errOf(g.run(j.(*listJob))))
	require.Empty(t, g.lst.only("chains:AAA"), "a name the rules drop is not asked")
	require.Len(t, g.lst.only("list:BBB"), 2)
	l, _ := g.ls.Lists(context.Background(), g.id("BBB"))
	require.Equal(t, "CBBB", l[0].Class)
}

func TestPostconditionScopedToOwnNames(t *testing.T) {
	t.Parallel()
	g := newLRig(t, "AAA", "BBB")
	g.seed("AAA", reasonNone, 2, 22, 29)
	g.seed("BBB", reasonNotFinished, 0, 22)
	require.NoError(t, g.ls.PutListRuns(context.Background(), []openquote.ListRunRecord{
		{UnderlyingID: g.id("AAA"), Session: g.day, Listed: 2, Reason: reasonNone}}))
	require.True(t, g.present(g.job(nil)))
	rules := usRules()
	all := rules.Names
	rules.Names = func(ctx context.Context, s Stored, day time.Time) ([]stock, []store.Position, error) {
		sts, pos, err := all(ctx, s, day)
		var out []stock
		for _, st := range sts {
			if st.Symbol == "BBB" {
				out = append(out, st)
			}
		}
		return out, pos, err
	}
	j, err := newListPreopen(listManifest, rules, g.deps())
	require.NoError(t, err)
	require.False(t, g.present(j.(*listJob)), "the rows of the other names of the day are not read")
}
