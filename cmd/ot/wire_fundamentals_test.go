package main

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

type fundamentalsRec struct {
	dsns, agents []string
	closed       int
	src          sources.FactSource
	writer       store.FactWriter
	entries      []jobs.WatchEntry
	val          computefigures.ValuationDeps
	figures      int
}

type fakeFactStore struct {
	rec *fundamentalsRec
}

func (fakeFactStore) InsertFacts(context.Context, []contracts.FundamentalFact) error { return nil }
func (fakeFactStore) FactsFiledBy(context.Context, string, string, contracts.Day) ([]contracts.FundamentalFact, error) {
	return nil, nil
}
func (s fakeFactStore) Close() error { s.rec.closed++; return nil }

type fakeFactSource struct{}

func (fakeFactSource) Fetch(context.Context, sources.FactPlan) ([]contracts.FundamentalFact, error) {
	return nil, nil
}

// withFundamentalsFakes puts fakes of every constructor of fundamentalsDeps in the ext deps of d.
func withFundamentalsFakes(d *deps) *fundamentalsRec {
	rec := &fundamentalsRec{}
	real := newExt()[extFundamentals].(fundamentalsDeps)
	d.ext = map[string]any{extFundamentals: fundamentalsDeps{
		newSEC: func(c sources.SECConfig) (sources.FactSource, error) {
			rec.agents = append(rec.agents, c.UserAgent)
			return fakeFactSource{}, nil
		},
		newJob: func(src sources.FactSource, w store.FactWriter, entries []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
			rec.src, rec.writer, rec.entries = src, w, entries
			return namedJob{id: jobFundamentals}, nil
		},
		openFacts: func(_ context.Context, dsn string) (store.FactStore, error) {
			rec.dsns = append(rec.dsns, dsn)
			return fakeFactStore{rec}, nil
		},
		newFigures: func(_ computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps,
			v computefigures.ValuationDeps, _ clock.Clock) (contracts.Job, error) {
			rec.val = v
			rec.figures++
			return &figJob{}, nil
		},
		fn: real.fn,
	}}
	return rec
}

func TestFundamentalsLeftOutUnset(t *testing.T) {
	t.Parallel()
	for name, env := range map[string]string{
		"no dsn":        fullEnv + "SEC_USER_AGENT=agent\n",
		"no user agent": fullEnv + "OT_PG_DSN_FUNDAMENTAL=fund-dsn\n",
		"neither":       fullEnv,
	} {
		r := newRig(t, env)
		rec := withFundamentalsFakes(&r.a.deps)
		if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := jobIDs(r.jobs); slices.Contains(got, jobFundamentals) {
			t.Errorf("%s: jobs %v hold fundamentals", name, got)
		}
		if rec.src != nil {
			t.Errorf("%s: the job was built", name)
		}
		log := r.log.String()
		if !strings.Contains(log, jobFundamentals) || !strings.Contains(log, "is not set") {
			t.Errorf("%s: log %q names no reason", name, log)
		}
		if name == "no dsn" || name == "neither" {
			if len(rec.dsns) != 0 || rec.figures != 0 {
				t.Errorf("%s: fact store opened %v, valuation step built %d times", name, rec.dsns, rec.figures)
			}
		}
	}
}

func TestFundamentalsJobBuiltWhenSet(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv+"OT_PG_DSN_FUNDAMENTAL=fund-dsn\nSEC_USER_AGENT=agent mail\n")
	rec := withFundamentalsFakes(&r.a.deps)
	if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := jobIDs(r.jobs)
	if !slices.Contains(got, jobFundamentals) || slices.Index(got, jobFundamentals) < slices.Index(got, "compute_figures") {
		t.Fatalf("jobs %v", got)
	}
	if len(rec.dsns) != 1 || rec.dsns[0] != "fund-dsn" || len(rec.agents) != 1 || rec.agents[0] != "agent mail" {
		t.Fatalf("fact store dsns %v, user agents %v", rec.dsns, rec.agents)
	}
	if _, ok := rec.src.(fakeFactSource); !ok || rec.writer != store.FactWriter(fakeFactStore{rec}) || !slices.Equal(rec.entries, r.earnWatch) {
		t.Fatalf("job got source %v, writer %v, entries %v", rec.src, rec.writer, rec.entries)
	}
	v := rec.val
	if rec.figures == 0 || v.Facts != store.FactReader(fakeFactStore{rec}) || !slices.Equal(v.Entries, r.earnWatch) ||
		v.StandardOf == nil || v.BuildFilings == nil || v.Band == nil || v.Questions == nil || v.Diagnosis == nil {
		t.Fatalf("valuation step %+v built %d times, want at least once (at start and at each Plan)", v, rec.figures)
	}
	if rec.closed != 1 {
		t.Errorf("fact store closed %d times, want once", rec.closed)
	}
}

// figJob is a scheduled job that plans the given subject keys and records the keys it is run with.
type figJob struct {
	keys []string
	ran  []string
}

func (j *figJob) ID() string { return "compute_figures" }
func (j *figJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	out := make([]contracts.ExpectedKey, 0, len(j.keys))
	for _, k := range j.keys {
		out = append(out, contracts.ExpectedKey{DatasetID: "figures", SubjectKey: k})
	}
	return out, strings.Join(j.keys, ","), nil
}
func (j *figJob) Run(_ context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	for _, k := range keys {
		j.ran = append(j.ran, k.SubjectKey)
	}
	return contracts.JobResult{Returned: len(keys), Usable: len(keys)}
}
func (*figJob) Schedule() (string, string, string, time.Duration) { return "cal", "close", "UTC", 0 }
func (*figJob) RunBound() time.Duration                           { return time.Minute }
func (*figJob) RestartSensitive() bool                            { return false }
func (*figJob) Postcondition(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func TestComputeFiguresKeepsEaseWithFundamentals(t *testing.T) {
	t.Parallel()
	const easeKey = "macro.ease|market|WALCL"
	d := newDeps()
	easeJob := &figJob{keys: []string{"ratio|underlying|AAA", easeKey}}
	valJob := &figJob{keys: []string{"ratio|underlying|AAA", "valuation.buy_band|underlying|AAA"}}
	d.newComputeFigures = func(computefigures.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps,
		compute.Ease, clock.Clock) (contracts.Job, error) {
		return easeJob, nil
	}
	d.ext = map[string]any{extFundamentals: fundamentalsDeps{
		newFigures: func(computefigures.ComputeStores, compute.Figures, compute.Rules, compute.Units, compute.SeriesOps,
			computefigures.ValuationDeps, clock.Clock) (contracts.Job, error) {
			return valJob, nil
		},
	}}
	fd := fundamentalsDeps{newFigures: d.ext[extFundamentals].(fundamentalsDeps).newFigures}
	job, err := (jobFactory{d: d, fd: fd, facts: fakeFactStore{&fundamentalsRec{}}}).withList(oneUSRow()).computeFigures()
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := job.Plan(context.Background(), "2026-11-04")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, k := range keys {
		got = append(got, k.SubjectKey)
	}
	want := []string{easeKey, "ratio|underlying|AAA", "valuation.buy_band|underlying|AAA"}
	if !slices.Equal(got, want) {
		t.Fatalf("plan %v, want %v", got, want)
	}
	if res := job.Run(context.Background(), "2026-11-04", keys); res.Returned != 3 || res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	if !slices.Equal(easeJob.ran, []string{easeKey}) || !slices.Equal(valJob.ran, want[1:]) {
		t.Fatalf("ease job ran %v, valuation job ran %v", easeJob.ran, valJob.ran)
	}
}
