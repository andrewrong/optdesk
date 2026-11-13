package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/techstate"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

func TestComputeFiguresJobGetsEase(t *testing.T) {
	t.Parallel()
	d := newDeps()
	if d.computeEase == nil {
		t.Fatal("computeEase is nil")
	}
	var got compute.Ease
	d.newComputeFigures = func(
		_ computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, e compute.Ease, _ clock.Clock,
	) (contracts.Job, error) {
		got = e
		return namedJob{id: "compute_figures"}, nil
	}
	if _, err := (jobFactory{d: d}).withList(oneUSRow()).computeFigures(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got != d.computeEase {
		t.Fatalf("job got ease %v, want the one of newDeps", got)
	}
}

func TestTechDepsAreTheTrialOnes(t *testing.T) {
	t.Parallel()
	got := techDeps()
	if got.Classify == nil {
		t.Fatal("Classify is nil")
	}
	if got.Params != techstate.TrialParams() {
		t.Fatalf("Params %+v, want the trial ones %+v", got.Params, techstate.TrialParams())
	}
}

func TestComputeFiguresWithoutIndexReaderIsNotWrapped(t *testing.T) {
	t.Parallel()
	d := newDeps()
	want := namedJob{id: "compute_figures"}
	d.newComputeFigures = func(
		_ computefigures.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, _ compute.Ease, _ clock.Clock,
	) (contracts.Job, error) {
		return want, nil
	}
	got, err := (jobFactory{d: d}).computeFiguresOf(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != contracts.Job(want) {
		t.Fatalf("job %#v, want the one the constructor returned", got)
	}
}

// barRec records what the bar fakes were given.
type barRec struct {
	dsns                []string
	stores              []*fakeBarStore
	barsB, fxB          broker.Broker
	barsH, fxH          broker.HistoryClient
	barsW, barsR        any
	fxW, fxR            any
	entries             []jobs.WatchEntry
	historyDials        int
	historyErr          error
	closedStores, hists int
}

type fakeBarStore struct {
	bar.BarStore
	rec *barRec
}

func (s *fakeBarStore) Close() error { s.rec.closedStores++; return nil }

type fakeHistory struct{ rec *barRec }

func (fakeHistory) Bars(context.Context, broker.BarQuery) ([]broker.Bar, error) { return nil, nil }

func (h fakeHistory) Close() error { h.rec.hists++; return nil }

// withBarFakes sets the four bar deps on d to fakes that record into the returned barRec.
func withBarFakes(d *deps) *barRec {
	rec := &barRec{}
	d.openBar = func(_ context.Context, dsn string) (bar.BarStore, error) {
		rec.dsns = append(rec.dsns, dsn)
		s := &fakeBarStore{rec: rec}
		rec.stores = append(rec.stores, s)
		return s, nil
	}
	d.newHistory = func(context.Context, string, broker.Options) (broker.HistoryClient, error) {
		rec.historyDials++
		if rec.historyErr != nil {
			return nil, rec.historyErr
		}
		return fakeHistory{rec}, nil
	}
	d.newBarsRefresh = func(b broker.Broker, h broker.HistoryClient, _ store.InstrumentResolver, w bar.PriceBarWriter,
		r bar.PriceBarReader, entries []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
		rec.barsB, rec.barsH, rec.barsW, rec.barsR, rec.entries = b, h, w, r, entries
		return namedJob{id: jobBarsRefresh}, nil
	}
	d.newFxRefresh = func(b broker.Broker, h broker.HistoryClient, w bar.FxRateWriter, r bar.FxRateReader, _ clock.Clock) (contracts.Job, error) {
		rec.fxB, rec.fxH, rec.fxW, rec.fxR = b, h, w, r
		return namedJob{id: jobFxRefresh}, nil
	}
	return rec
}

var barEnv = bar.DSNEnvBar + "=bar-dsn\n"

func barRunOnce(t *testing.T, env string) (*rig, *barRec) {
	t.Helper()
	r := newRig(t, env)
	rec := withBarFakes(&r.a.deps)
	if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	return r, rec
}

func TestBarsRefreshRegistered(t *testing.T) {
	t.Parallel()
	r, rec := barRunOnce(t, fullEnv+barEnv)
	if got := jobIDs(r.jobs); !slices.Contains(got, jobBarsRefresh) {
		t.Fatalf("run-once jobs %v lack bars_refresh", got)
	}
	if len(rec.stores) == 0 || rec.dsns[0] != "bar-dsn" || rec.barsW != any(rec.stores[0]) || rec.barsR != any(rec.stores[0]) {
		t.Fatalf("bars_refresh not built over the store opened with the bar line: dsns %v", rec.dsns)
	}
	if rec.barsH == nil || rec.barsH == broker.HistoryClient(unreachableHistory{}) || rec.historyDials != 1 {
		t.Fatalf("history client %v after %d dials", rec.barsH, rec.historyDials)
	}
	if !slices.Equal(rec.entries, r.earnWatch) {
		t.Fatalf("entries %v, want the watch list %v", rec.entries, r.earnWatch)
	}
	s := newSchedRig(t, fullEnv+barEnv)
	withBarFakes(&s.a.deps)
	done := s.start()
	s.waitStarted(t, done)
	if got := s.lastBuild(); !slices.Contains(got, jobBarsRefresh) || len(s.sched.disabled) != 0 {
		t.Fatalf("scheduler jobs %v, disabled %v", got, s.sched.disabled)
	}
	_ = s.stop(t, done)
}

func TestFxRefreshRegistered(t *testing.T) {
	t.Parallel()
	r, rec := barRunOnce(t, fullEnv+barEnv)
	got := jobIDs(r.jobs)
	if !slices.Contains(got, jobFxRefresh) || slices.Index(got, jobFxRefresh) < slices.Index(got, jobBarsRefresh) {
		t.Fatalf("run-once jobs %v", got)
	}
	if rec.fxW != any(rec.stores[0]) || rec.fxR != any(rec.stores[0]) || rec.fxH != rec.barsH {
		t.Fatal("fx_refresh is not built over the bar store and the history client of bars_refresh")
	}
	s := newSchedRig(t, fullEnv+barEnv)
	withBarFakes(&s.a.deps)
	done := s.start()
	s.waitStarted(t, done)
	if got := s.lastBuild(); !slices.Contains(got, jobFxRefresh) {
		t.Fatalf("scheduler jobs %v", got)
	}
	_ = s.stop(t, done)
	if rec.closedStores != len(rec.stores) || rec.hists != 1 {
		t.Fatalf("closed %d of %d stores, %d history clients", rec.closedStores, len(rec.stores), rec.hists)
	}
}

func TestMissingBarDSNDisablesOnlyBars(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	rec := withBarFakes(&r.a.deps)
	if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "option_eod_index", "calendar_refresh", "earnings_refresh"}
	if got := jobIDs(r.jobs); !slices.Equal(got, want) {
		t.Fatalf("jobs %v, want %v", got, want)
	}
	log := r.log.String()
	if !strings.Contains(log, "bars_refresh") || !strings.Contains(log, "fx_refresh") || !strings.Contains(log, "OT_PG_DSN_BAR is not set") {
		t.Fatalf("log %q lacks the named line", log)
	}
	if len(rec.dsns) != 0 || rec.historyDials != 0 || r.compute.PriceBars != nil {
		t.Fatalf("a bar store was opened (%v), a history client dialled or the HV step kept", rec.dsns)
	}
	s := newSchedRig(t, fullEnv)
	withBarFakes(&s.a.deps)
	done := s.start()
	s.waitStarted(t, done)
	if len(s.sched.disabled) != 2 || s.sched.disabled[jobBarsRefresh] != reasonNoBarDSN || s.sched.disabled[jobFxRefresh] != reasonNoBarDSN {
		t.Fatalf("disabled %v", s.sched.disabled)
	}
	if got := s.lastBuild(); slices.Contains(got, jobBarsRefresh) || !slices.Contains(got, jobOptionEod) {
		t.Fatalf("scheduler jobs %v", got)
	}
	_ = s.stop(t, done)
}

func TestComputeFiguresGetsPriceBars(t *testing.T) {
	t.Parallel()
	r, rec := barRunOnce(t, fullEnv+barEnv)
	if r.compute.PriceBars == nil {
		t.Fatal("compute_figures got no PriceBars")
	}
	if len(rec.dsns) != 2 || rec.dsns[1] != "pages-dsn" || r.compute.PriceBars != bar.PriceBarReader(rec.stores[1]) {
		t.Fatalf("the reader is not the store opened with the pages line: dsns %v", rec.dsns)
	}
}

func TestBrokerDownDisablesBarJobs(t *testing.T) {
	t.Parallel()
	for name, fail := range map[string]func(*rig, *barRec){
		"broker":  func(r *rig, _ *barRec) { r.fail["broker"] = errors.New("dial: /secret/path refused") },
		"history": func(_ *rig, rec *barRec) { rec.historyErr = errors.New("dial: /secret/path refused") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fullEnv+barEnv)
			rec := withBarFakes(&r.a.deps)
			fail(r, rec)
			if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
				t.Fatal(err)
			}
			if got := jobIDs(r.jobs); slices.Contains(got, jobBarsRefresh) || slices.Contains(got, jobFxRefresh) {
				t.Fatalf("jobs %v hold a bar job", got)
			}
			log := r.log.String()
			if !strings.Contains(log, "bars_refresh") || !strings.Contains(log, reasonBrokerDown) || strings.Contains(log, "/secret/path") {
				t.Fatalf("log %q", log)
			}
			if r.compute.PriceBars != nil {
				t.Fatal("the HV step kept its reader")
			}
		})
	}
	s := newSchedRig(t, fullEnv+barEnv)
	withBarFakes(&s.a.deps)
	s.a.deps.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) { return nil, errors.New("down") }
	done := s.start()
	s.waitStarted(t, done)
	if s.sched.disabled[jobBarsRefresh] != reasonBrokerDown || s.sched.disabled[jobFxRefresh] != reasonBrokerDown {
		t.Fatalf("disabled %v", s.sched.disabled)
	}
	_ = s.stop(t, done)
}
