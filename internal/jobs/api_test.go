package jobs_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

func TestRegistryContract(t *testing.T) {
	t.Parallel()
	var newJob jobs.NewFredSeriesFunc = func(sources.Source, store.SeriesWriter, clock.Clock) (contracts.Job, error) {
		return fakeJob{"fred_series"}, nil
	}
	j, err := newJob(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	reg := jobs.Registry{j}
	var list []contracts.Job = reg // a Registry is the slice the runner takes
	if len(list) != 1 || list[0].ID() != "fred_series" {
		t.Fatalf("registry = %v", list)
	}
}
