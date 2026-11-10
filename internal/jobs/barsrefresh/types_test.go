package barsrefresh_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/barsrefresh"
	"github.com/andrewrong/optdesk/internal/store"
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

func TestBarsRefreshContract(t *testing.T) {
	t.Parallel()
	var f barsrefresh.NewBarsRefreshFunc = func(
		broker.Broker, broker.HistoryClient, store.InstrumentResolver, bar.PriceBarWriter, bar.PriceBarReader,
		[]jobs.WatchEntry, clock.Clock,
	) (contracts.Job, error) {
		return fakeJob{id: "bars_refresh"}, nil
	}
	j, err := f(nil, nil, nil, nil, nil, nil, nil)
	if err != nil || j.ID() != "bars_refresh" {
		t.Fatalf("got %v, %v", j, err)
	}
}
