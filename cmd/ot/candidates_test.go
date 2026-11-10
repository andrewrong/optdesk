package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

// init adds ot candidates to the exclusions of the leak table (env_test.go) from this file.
func init() {
	leakExcluded["candidates"] = "the leak deps hold no candidate opener; its open error is withheld and TestCandidatesOpenErrorWithheld covers it"
}

const candidateEnv = "OT_PG_DSN_CANDIDATE=cand-dsn\n"

// fakeCandidateRW is the candidate store of the job and of the command; it records its close.
type fakeCandidateRW struct {
	store.CandidateWriter
	closed *atomic.Int32
	rows   []store.Candidate
	day    contracts.Day
	kind   string
}

func (f *fakeCandidateRW) Close() error { f.closed.Add(1); return nil }
func (f *fakeCandidateRW) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	f.day, f.kind = day, kind
	return f.rows, nil
}

// candJobStub is a named job whose plan is empty.
type candJobStub struct {
	fakeJob
	id string
}

func (j candJobStub) ID() string { return j.id }

// fakeRule is the pure rule of the job; the fake job never calls it.
type fakeRule struct{}

func (fakeRule) Evaluate(compute.CandidateInput, compute.CandidateParams) []compute.Candidate {
	return nil
}

// namedEodWatch is a newOptionEodWatch that builds a job with the given id.
func namedEodWatch(id string) jobs.NewOptionEodWatchFunc {
	return func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
		return candJobStub{id: id}, nil
	}
}

func TestCandidatesRegisteredAfterInputs(t *testing.T) {
	t.Parallel()
	a, logs := testApp(t, fullEnv+candidateEnv)
	var op, cl, rwOpened atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &op, &cl, &day)
	var ids []string
	a.deps.newOptionEodWatch = namedEodWatch(jobOptionEod)
	a.deps.newEarningsRefresh = func(
		map[string]sources.EarningsSource, store.EarningsDateWriter, store.EarningsDateReader, []jobs.WatchEntry, clock.Clock,
	) (contracts.Job, error) {
		return candJobStub{id: jobEarningsRefresh}, nil
	}
	a.deps.openCandidateRW = func(_ context.Context, dsn string) (candidateRW, error) {
		rwOpened.Add(1)
		if dsn != "cand-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return &fakeCandidateRW{closed: &cl}, nil
	}
	a.deps.newOptionCandidates = func(st jobs.CandidateStores, rule compute.OptionCandidates, _ []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
		if st.Candidates == nil || st.Earnings == nil || rule == nil {
			t.Errorf("stores %+v rule %v", st, rule)
		}
		return candJobStub{id: jobOptionCandidates}, nil
	}
	a.deps.newCandidateRule = func() compute.OptionCandidates { return fakeRule{} }
	a.deps.newRunner = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, js []contracts.Job, _ runner.WindowRule) runner.Reconciler {
		for _, j := range js {
			ids = append(ids, j.ID())
		}
		return recRunner{&day}
	}
	if err := runOnce(a, nil); err != nil {
		t.Fatalf("%v\n%s", err, logs)
	}
	ic, ie, ix := slices.Index(ids, jobOptionCandidates), slices.Index(ids, jobOptionEod), slices.Index(ids, jobEarningsRefresh)
	if ic < 0 || ie < 0 || ix < 0 || ic < ie || ic < ix || ic != len(ids)-1 {
		t.Fatalf("job order %v", ids)
	}
	if rwOpened.Load() != 1 {
		t.Fatalf("candidate store opened %d times", rwOpened.Load())
	}
}

func TestMissingCandidateDSNDisablesOnlyIt(t *testing.T) {
	t.Parallel()
	a, logs := testApp(t, fullEnv)
	var op, cl, rwOpened atomic.Int32
	var day contracts.Day
	runOnceFakes(a, &op, &cl, &day)
	var ids []string
	a.deps.newOptionEodWatch = namedEodWatch(jobOptionEod)
	a.deps.openCandidateRW = func(context.Context, string) (candidateRW, error) {
		rwOpened.Add(1)
		return &fakeCandidateRW{closed: &cl}, nil
	}
	a.deps.newOptionCandidates = func(jobs.CandidateStores, compute.OptionCandidates, []jobs.WatchEntry, clock.Clock) (contracts.Job, error) {
		return candJobStub{id: jobOptionCandidates}, nil
	}
	a.deps.newCandidateRule = func() compute.OptionCandidates { return fakeRule{} }
	a.deps.newRunner = func(_ store.OpsLedger, _ clock.Calendar, _ clock.Clock, js []contracts.Job, _ runner.WindowRule) runner.Reconciler {
		for _, j := range js {
			ids = append(ids, j.ID())
		}
		return recRunner{&day}
	}
	if err := runOnce(a, nil); err != nil {
		t.Fatalf("%v\n%s", err, logs)
	}
	if slices.Contains(ids, jobOptionCandidates) || !slices.Contains(ids, jobOptionEod) {
		t.Fatalf("job ids %v", ids)
	}
	if rwOpened.Load() != 0 {
		t.Fatalf("candidate store opened without its line")
	}
	if !strings.Contains(logs.String(), "OT_PG_DSN_CANDIDATE is not set") || !strings.Contains(logs.String(), jobOptionCandidates) {
		t.Fatalf("no named line: %s", logs)
	}
	// The scheduler leaves the job out and its probe keeps it out until a restart.
	cfg := runOnceConfig{}
	if cfg.candidatesOff() != reasonNoCandidateDSN {
		t.Fatalf("candidatesOff %q", cfg.candidatesOff())
	}
	probe := candidateProbe(wiring{candidatesOff: reasonNoCandidateDSN}, func(context.Context, string) (contracts.Job, string) {
		t.Error("the next probe was called for option_candidates")
		return nil, ""
	})
	if j, reason := probe(context.Background(), jobOptionCandidates); j != nil || reason != reasonNoCandidateDSN {
		t.Fatalf("probe %v %q", j, reason)
	}
}

func TestCandidatesCommandPrints(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var cl atomic.Int32
	yield := "0.123456"
	rw := &fakeCandidateRW{closed: &cl, rows: []store.Candidate{
		{Issuer: "cik:0000320193", Kind: "short_put", Expiry: "2026-12-18", Strike: "95.00", BidEod: "1.20", BreakEven: "93.80",
			AnnualizedYield: &yield, Status: "ok", Marks: "[]", Gates: "[]"},
	}}
	a.deps.openCandidate = func(_ context.Context, dsn string) (candidateStore, error) {
		if dsn != "pages-dsn" {
			t.Errorf("dsn %q", dsn)
		}
		return rw, nil
	}
	if err := candidatesCmd(a, []string{"--day", "2026-11-05"}); err != nil {
		t.Fatal(err)
	}
	if rw.day != "2026-11-05" || rw.kind != "short_put" || cl.Load() != 1 {
		t.Fatalf("day %q kind %q closed %d", rw.day, rw.kind, cl.Load())
	}
	for _, want := range []string{"cik:0000320193", "strike 95.00", "yield 0.123456", "breakeven 93.80"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("output lacks %q: %q", want, out.String())
		}
	}
	if err := candidatesCmd(a, []string{"--bad"}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	a2, _ := testApp(t, "OT_PG_DSN_RUNNER=r\n")
	if err := candidatesCmd(a2, nil); err == nil || !strings.Contains(err.Error(), "OT_PG_DSN_PAGES") {
		t.Fatalf("missing line: %v", err)
	}
}

func TestCandidatesOpenErrorWithheld(t *testing.T) {
	t.Parallel()
	const dsn = "postgres://sentinel-user:sentinel-pw@h/db"
	a, _ := testApp(t, "OT_PG_DSN_PAGES="+dsn+"\n")
	a.deps.openCandidate = func(_ context.Context, d string) (candidateStore, error) { return nil, errors.New("dial " + d) }
	err := candidatesCmd(a, nil)
	if err == nil || strings.Contains(err.Error(), "sentinel") || !strings.Contains(err.Error(), "OT_PG_DSN_PAGES") {
		t.Fatalf("err %v", err)
	}
}

func TestCandidatesNamesJPEntry(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	var out bytes.Buffer
	a.stdout = &out
	var cl atomic.Int32
	a.deps.openCandidate = func(context.Context, string) (candidateStore, error) { return &fakeCandidateRW{closed: &cl}, nil }
	a.deps.readFile = func(string) ([]byte, error) { return []byte(watchTwo), nil }
	if err := candidatesCmd(a, nil); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || lines[0] != "no candidates" || lines[1] != "7203: no option route: see option-candidates-jp" {
		t.Fatalf("lines %q", lines)
	}
}

func TestCandidatesReadsThroughPagesDSN(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=pages-dsn\n")
	a.stdout = io.Discard
	var got string
	a.deps.openCandidate = func(_ context.Context, dsn string) (candidateStore, error) {
		got = dsn
		return &fakeCandidateRW{closed: new(atomic.Int32)}, nil
	}
	if err := candidatesCmd(a, []string{"--day", "2026-11-05"}); err != nil || got != "pages-dsn" {
		t.Fatalf("err %v dsn %q", err, got)
	}
}
