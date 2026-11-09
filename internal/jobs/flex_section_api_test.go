package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store/flexsection"
)

type fakeSections struct{ calls int }

func (f *fakeSections) RecordSections(_ context.Context, _ string, rows []flexsection.Row, _ []flexsection.Status) (flexsection.Insert, error) {
	f.calls++
	return flexsection.Insert{New: len(rows)}, nil
}

// sectionJob records whether the section step is on: Sections set.
type sectionJob struct{ w flexsection.Writer }

func (sectionJob) ID() string { return "option_ledger" }
func (sectionJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (j sectionJob) Run(ctx context.Context, _ contracts.Day, _ []contracts.ExpectedKey) contracts.JobResult {
	if j.w == nil {
		return contracts.JobResult{}
	}
	ins, err := j.w.RecordSections(ctx, "f1", []flexsection.Row{{Alias: "a", Section: "CashReport", RowKey: "k", RecordedAt: time.Time{}}}, nil)
	if err != nil {
		return contracts.JobResult{Returned: -1}
	}
	return contracts.JobResult{Returned: ins.New}
}

func TestOptionLedgerStoresSectionsContract(t *testing.T) {
	t.Parallel()
	newJob := jobs.NewOptionLedgerJobFunc(func(s jobs.OptionLedgerStores, _ jobs.OptionLedgerConfig, _ clock.Clock) (contracts.Job, error) {
		return sectionJob{w: s.Sections}, nil
	})

	fake := &fakeSections{}
	var w flexsection.Writer = fake
	on, err := newJob(jobs.OptionLedgerStores{Sections: w}, jobs.OptionLedgerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := on.Run(context.Background(), "2026-11-09", nil).Returned; got != 1 || fake.calls != 1 {
		t.Fatalf("Sections set: returned %d, calls %d", got, fake.calls)
	}

	off, err := newJob(jobs.OptionLedgerStores{}, jobs.OptionLedgerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := off.Run(context.Background(), "2026-11-09", nil).Returned; got != 0 || fake.calls != 1 {
		t.Fatalf("Sections nil: returned %d, calls %d", got, fake.calls)
	}
}
