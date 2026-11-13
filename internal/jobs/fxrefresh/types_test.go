package fxrefresh_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs/fxrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store/bar"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

func TestFxRefreshContract(t *testing.T) {
	t.Parallel()
	var f fxrefresh.NewFxRefreshFunc = func(
		broker.Broker, broker.HistoryClient, bar.FxRateWriter, bar.FxRateReader, clock.Clock,
	) (contracts.Job, error) {
		return fakeJob{id: "fx_refresh"}, nil
	}
	j, err := f(nil, nil, nil, nil, nil)
	if err != nil || j.ID() != "fx_refresh" {
		t.Fatalf("got %v, %v", j, err)
	}
}

// recordingJob is a fake job that keeps the source attached to it.
type recordingJob struct {
	fakeJob
	got sources.Source
}

func TestAttachSecondaryContract(t *testing.T) {
	t.Parallel()
	known := &recordingJob{fakeJob: fakeJob{id: "fx_refresh"}}
	var f fxrefresh.AttachSecondaryFunc = func(j contracts.Job, second sources.Source) bool {
		r, ok := j.(*recordingJob)
		if !ok {
			return false
		}
		r.got = second
		return true
	}
	var src sources.Source
	if !f(known, src) {
		t.Fatal("known job: got false, want true")
	}
	if known.got != src {
		t.Fatalf("source not recorded: %v", known.got)
	}
	if f(fakeJob{id: "other"}, src) {
		t.Fatal("unknown job: got true, want false")
	}
}
