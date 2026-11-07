package jobs_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
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

// captureOf is the caller side: it plans and runs the job and derives the capture id of a key the way the
// design says (uuid.NewSHA1 over day|dataset|subject).
func captureOf(day contracts.Day, k contracts.ExpectedKey) jobs.CaptureID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+k.DatasetID+"|"+k.SubjectKey))
}

func planAndRun(t *testing.T, j contracts.Job) (jobs.CaptureID, contracts.JobResult) {
	t.Helper()
	day := contracts.Day("2026-11-06")
	keys, _, err := j.Plan(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), day, keys)
	return captureOf(day, contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "SPY"}), res
}

func TestNewOptionEodFuncContract(t *testing.T) {
	t.Parallel()
	var newJob jobs.NewOptionEodFunc = func(broker.Broker, store.InstrumentResolver, store.OptionEodWriter, clock.Clock) (contracts.Job, error) {
		return fakeJob{"option_eod"}, nil
	}
	j, err := newJob(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := planAndRun(t, j)
	again := captureOf("2026-11-06", contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "SPY"})
	if id != again || id == (jobs.CaptureID{}) {
		t.Fatalf("capture id = %v, again %v", id, again)
	}
}

func TestNewQuoteSnapshotFuncContract(t *testing.T) {
	t.Parallel()
	var newJob jobs.NewQuoteSnapshotFunc = func(broker.Broker, store.InstrumentResolver, store.QuoteSnapshotWriter, clock.Clock) (contracts.Job, error) {
		return fakeJob{"quote_snapshot"}, nil
	}
	j, err := newJob(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := planAndRun(t, j)
	other := captureOf("2026-11-06", contracts.ExpectedKey{DatasetID: "obs.quote_snapshot", SubjectKey: "0945"})
	if id == other {
		t.Fatal("different keys gave one capture id")
	}
}
