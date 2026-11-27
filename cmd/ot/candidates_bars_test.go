package main

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optioncandidates"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

// barsRun builds option_candidates over a factory holding bars and returns the stores the job constructor received.
func barsRun(t *testing.T, bars bar.BarStore) optioncandidates.CandidateStores {
	t.Helper()
	a, _ := testApp(t, candidateEnv)
	var got optioncandidates.CandidateStores
	a.deps.newCandidateRule = func() compute.OptionCandidates { return fakeRule{} }
	a.deps.newOptionCandidates = func(st optioncandidates.CandidateStores, _ compute.OptionCandidates, _ []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
		got = st
		return candJobStub{id: jobOptionCandidates}, nil
	}
	f := jobFactory{d: a.deps}.withList(listRowsOf(jobs.WatchEntry{Market: "US", Contract: broker.Contract{Symbol: "AAA"}})...)
	f.bars = bars
	if _, err := f.optionCandidates(); err != nil {
		t.Fatalf("optionCandidates: %v", err)
	}
	return got
}

func TestOptionCandidatesGetsBars(t *testing.T) {
	t.Parallel()
	rd := noBarStore{}
	got := barsRun(t, rd)
	if got.Bars != bar.PriceBarReader(rd) {
		t.Fatalf("Bars %v, want the reader of the factory", got.Bars)
	}
}

func TestOptionCandidatesNoBars(t *testing.T) {
	t.Parallel()
	if got := barsRun(t, nil); got.Bars != nil {
		t.Fatalf("Bars %v, want nil", got.Bars)
	}
}
