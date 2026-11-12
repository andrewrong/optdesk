package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

// listRowsOf is the stored row of each entry: active, options and fundamentals on, the fields of the entry kept.
func listRowsOf(entries ...jobs.WatchEntry) []underlying.Row {
	rows := make([]underlying.Row, 0, len(entries))
	for _, e := range entries {
		c := e.Contract
		rows = append(rows, underlying.Row{
			Key: nameKey(e.Market, c.Symbol), Symbol: c.Symbol, Market: e.Market, Kind: underlying.KindStock,
			Exchange: c.Exchange, Currency: c.Currency, ConID: c.ConID, Issuer: e.Issuer, Status: underlying.StatusActive,
			Options: true, Fundamentals: true, BuyPrice: e.BuyPrice, SalePrice: e.SalePrice, ReportDateOverride: e.ReportDateOverride,
		})
	}
	return rows
}

// listRowsFromWatch is listRowsOf over the entries of a watch.toml text.
func listRowsFromWatch(t *testing.T, body string) []underlying.Row {
	t.Helper()
	entries, err := parseWatch([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	return listRowsOf(entries...)
}

// withList gives f a reader over rows and nobody held.
func (f jobFactory) withList(rows ...underlying.Row) jobFactory {
	f.list = &fakeListReader{rows: rows}
	if f.clk == nil {
		f.clk = fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)}
	}
	f.held = func(context.Context) (map[string]bool, error) { return nil, nil }
	return f
}

// wireRig is a rig that wires every family over a stored list of one active name.
func wireRig(t *testing.T, env string, rows ...underlying.Row) *rig {
	t.Helper()
	r := newRig(t, env)
	r.list.rows = rows
	return r
}

func oneUSRow() underlying.Row {
	return listRowsOf(jobs.WatchEntry{Issuer: "cik:0000320193", Market: "US",
		Contract: broker.Contract{SecType: "STK", Symbol: "AAPL", ConID: 265598}})[0]
}

func TestWiredFamiliesAreListedJobs(t *testing.T) {
	t.Parallel()
	r := wireRig(t, fullEnv, oneUSRow())
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"option_eod": true, "compute_figures": true, "earnings_refresh": true}
	for _, j := range r.jobs {
		_, listed := listedOf(j)
		if want[j.ID()] != listed {
			t.Errorf("%s: listed %v, want %v", j.ID(), listed, want[j.ID()])
		}
		delete(want, j.ID())
	}
	if len(want) != 0 {
		t.Errorf("not wired: %v", want)
	}
}

func TestEmptyStoreFailsEveryFamilyPlan(t *testing.T) {
	t.Parallel()
	r := wireRig(t, fullEnv)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, j := range r.jobs {
		if _, ok := listedOf(j); !ok {
			continue
		}
		checked++
		_, _, err := j.Plan(context.Background(), "2026-11-04")
		var te contracts.TypedError
		if err == nil || !strings.Contains(err.Error(), "underlying list is empty") || !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("%s: Plan err %v", j.ID(), err)
		}
	}
	if checked == 0 {
		t.Fatal("no listed family")
	}
}

func TestUnderlyingResolveFamily(t *testing.T) {
	t.Parallel()
	fam, ok := jobFamilies[jobUnderlyingResolve]
	if !ok || fam.order != 200 {
		t.Fatalf("family %+v registered %v", fam, ok)
	}
	// An empty line leaves only this job out, with one logged reason that names the line.
	r := newRig(t, fullEnv)
	jc := &jobCtx{f: &jobFactory{d: r.a.deps, clk: fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)}}, extra: &closerSet{}}
	j, reason, err := fam.build(context.Background(), r.a, jc)
	if j != nil || reason != "" || err != nil {
		t.Fatalf("job %v reason %q err %v", j, reason, err)
	}
	if !strings.Contains(r.log.String(), underlying.DSNEnvResolve+" is not set") {
		t.Fatalf("log %q", r.log.String())
	}
	// With the line set the job is built over the store the line opens.
	r = newRig(t, fullEnv+underlying.DSNEnvResolve+"=resolve-dsn\n")
	var dsn string
	r.a.deps.openUnderlying = func(_ context.Context, d string) (underlying.Store, error) { dsn = d; return r.list, nil }
	jc = &jobCtx{
		f: &jobFactory{d: r.a.deps, clk: fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)}}, brk: unreachableBroker{},
		cfg: runOnceConfig{resolveDSN: "resolve-dsn"}, extra: &closerSet{},
	}
	j, _, err = fam.build(context.Background(), r.a, jc)
	if err != nil || j == nil || j.ID() != jobUnderlyingResolve || dsn != "resolve-dsn" {
		t.Fatalf("job %v err %v dsn %q", j, err, dsn)
	}
}

// markerJob is the job a constructor returns: Run answers a result that names it, so a test sees which job ran.
type markerJob struct {
	contracts.Job
	// version stands for the settings version the constructor sets on the job it returns.
	version int
}

func (markerJob) ID() string { return jobOptionCandidates }
func (m markerJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{Returned: m.version}
}
func (markerJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}

// option_candidates is built by d.newOptionCandidates at each Plan, over the names of the store at that Plan, and the
// job that constructor returns (the one that carries the settings version) is the job that runs.
func TestOptionCandidatesKeepsItsConstructorJob(t *testing.T) {
	t.Parallel()
	d := newDeps()
	var seen [][]string
	d.newOptionCandidates = func(_ optioncandidates.CandidateStores, _ compute.OptionCandidates, es []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
		var syms []string
		for _, e := range es {
			syms = append(syms, e.Contract.Symbol)
		}
		seen = append(seen, syms)
		return markerJob{version: 42}, nil
	}
	d.newCandidateRule = func() compute.OptionCandidates { return nil }
	f := jobFactory{d: d, clk: fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)}}.withList(oneUSRow())
	j, err := f.optionCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := listedOf(j); !ok {
		t.Fatalf("job is %T, want a listedJob", j)
	}
	if _, _, err := j.Plan(context.Background(), "2026-11-04"); err != nil {
		t.Fatal(err)
	}
	if len(seen) == 0 || len(seen[len(seen)-1]) != 1 || seen[len(seen)-1][0] != "AAPL" {
		t.Fatalf("constructor saw %v, want AAPL", seen)
	}
	if res := j.Run(context.Background(), "2026-11-04", nil); res.Returned != 42 {
		t.Fatalf("result %+v, want the job of the constructor", res)
	}
}

// TestEveryListFamilyIsListed wires every family that follows the stored list, with every line and fake it needs, and
// finds each one as a listedJob.
func TestEveryListFamilyIsListed(t *testing.T) {
	t.Parallel()
	r := newDivRigEnv(t, fullEnv+barEnv+store.DSNEnvFundamental+"=fund-dsn\n"+store.DSNEnvCandidate+"=cand-dsn\n"+envSECUserAgent+"=agent mail\n")
	withFundamentalsFakes(&r.a.deps)
	withBarFakes(&r.a.deps)
	r.a.deps.openCandidateRW = func(context.Context, string) (candidateRW, error) {
		return &fakeCandidateRW{closed: &atomic.Int32{}}, nil
	}
	r.a.deps.newOptionCandidates = func(optioncandidates.CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
		return candJobStub{id: jobOptionCandidates}, nil
	}
	r.a.deps.newCandidateRule = func() compute.OptionCandidates { return fakeRule{} }
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"bars_refresh", "option_candidates", "dividend_capture", "fundamentals", "iv_history"} {
		found := false
		for _, j := range r.jobs {
			if j.ID() == id {
				_, found = listedOf(j)
			}
		}
		if !found {
			t.Errorf("%s is not a wired listedJob (jobs %v)", id, jobIDs(r.jobs))
		}
	}
}

// TestOptionOpenFamiliesAreListed builds the families of option_open, US and JP, as wireJobs does, and finds each one a
// listedJob; option_eod_jp is built over the same list.
func TestOptionOpenFamiliesAreListed(t *testing.T) {
	t.Parallel()
	h := newOpenHarness(t, deadEnv, true, nil)
	// The two pull jobs take no names (they read the stored plans of the day), so they are not listed jobs.
	pulls := map[string]bool{jobOptionOpen: true, jobOptionOpenJP: true}
	ids := append([]string{jobOptionOpenResolve, jobOptionOpenPlan, jobOptionOpen, jobOptionOpenResolveGap}, jpOpenIDs...)
	for _, id := range ids {
		j, _ := h.build(t, id)
		if _, ok := listedOf(j); ok == pulls[id] {
			t.Errorf("%s is %T, listed %v", id, j, ok)
		}
	}
	d := newDeps()
	d.newOptionEodJP = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
		return namedJob{id: jobOptionEodJP}, nil
	}
	d.newOptionEodJPWatch = nil
	j, err := jobFactory{d: d}.withList(oneUSRow()).optionEodJP(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := listedOf(j); !ok {
		t.Errorf("%s is %T, want a listedJob", jobOptionEodJP, j)
	}
}

type posAcctStore struct {
	store.AccountStore
	rows []store.Position
}

func (s posAcctStore) Positions(_ context.Context, alias, _ string) ([]store.Position, error) {
	if alias != "A" {
		return nil, nil
	}
	return s.rows, nil
}
func (posAcctStore) Close() error { return nil }

func TestHeldKeysOptionPositions(t *testing.T) {
	t.Parallel()
	st := posAcctStore{rows: []store.Position{
		{Symbol: "AAA", SecType: "option", Quantity: "-2"},
		{Symbol: "BBB", SecType: "option", Quantity: "0"},
		{Symbol: "CCC", SecType: "stock", Quantity: "100"},
	}}
	d := deps{openAccount: func(context.Context, string) (store.AccountStore, error) { return st, nil }}
	cfg := runOnceConfig{accountDSN: "dsn"}
	var extra closerSet
	now := func() time.Time { return time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC) }
	got, err := heldKeys(d, cfg, now, &extra)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"option:US:AAA": true, "option:JP:AAA": true, "US:CCC": true, "JP:CCC": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("missing %s in %v", k, got)
		}
	}
}

func heldKeysOf(t *testing.T, rows ...store.Position) map[string]bool {
	t.Helper()
	d := deps{openAccount: func(context.Context, string) (store.AccountStore, error) { return posAcctStore{rows: rows}, nil }}
	var extra closerSet
	now := func() time.Time { return time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC) }
	got, err := heldKeys(d, runOnceConfig{accountDSN: "dsn"}, now, &extra)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestHeldKeysListsStoredStockRow(t *testing.T) {
	t.Parallel()
	got := heldKeysOf(t, store.Position{Symbol: "CCC", SecType: "stock", Quantity: "100"})
	if !got["US:CCC"] || !got["JP:CCC"] || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestHeldKeysListsStoredOptionRow(t *testing.T) {
	t.Parallel()
	got := heldKeysOf(t, store.Position{Symbol: "AAA", SecType: "option", Quantity: "-2"})
	if !got["option:US:AAA"] || !got["option:JP:AAA"] || len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}
