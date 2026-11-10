package optionledger_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/optionledger"
	"github.com/andrewrong/optdesk/internal/store"
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

func newSectionJob(t *testing.T, st optionledger.OptionLedgerStores) contracts.Job {
	t.Helper()
	newJob := optionledger.NewOptionLedgerJobFunc(func(
		s optionledger.OptionLedgerStores, _ optionledger.OptionLedgerConfig, _ clock.Clock,
	) (contracts.Job, error) {
		return sectionJob{w: s.Sections}, nil
	})
	job, err := newJob(st, optionledger.OptionLedgerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return job
}

func TestOptionLedgerStoresSectionsSet(t *testing.T) {
	t.Parallel()
	fake := &fakeSections{}
	var w flexsection.Writer = fake
	job := newSectionJob(t, optionledger.OptionLedgerStores{Sections: w})
	if got := job.Run(context.Background(), "2026-11-09", nil).Returned; got != 1 || fake.calls != 1 {
		t.Fatalf("Sections set: returned %d, calls %d", got, fake.calls)
	}
}

func TestOptionLedgerStoresSectionsNil(t *testing.T) {
	t.Parallel()
	job := newSectionJob(t, optionledger.OptionLedgerStores{})
	if got := job.Run(context.Background(), "2026-11-09", nil).Returned; got != 0 {
		t.Fatalf("Sections nil: returned %d", got)
	}
}

type ledgerJob struct{ logins int }

func (ledgerJob) ID() string { return "option_ledger" }
func (ledgerJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (j ledgerJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{Returned: j.logins}
}

func TestOptionLedgerJobContract(t *testing.T) {
	t.Parallel()
	newJob := optionledger.NewOptionLedgerJobFunc(func(
		_ optionledger.OptionLedgerStores, cfg optionledger.OptionLedgerConfig, _ clock.Clock,
	) (contracts.Job, error) {
		return ledgerJob{logins: len(cfg.Logins)}, nil
	})
	// A login without a token reaches the job with a nil Source; the constructor accepts it.
	cfg := optionledger.OptionLedgerConfig{
		Logins:   []optionledger.FlexRun{{Name: "main", QueryIDs: []string{"1"}, Required: true}},
		Accounts: map[string]string{"ACC": "a"},
	}
	job, err := newJob(optionledger.OptionLedgerStores{}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID() != "option_ledger" {
		t.Fatalf("id %q", job.ID())
	}
	if got := job.Run(context.Background(), "2026-11-09", nil).Returned; got != 1 {
		t.Fatalf("logins seen: %d", got)
	}
}

func TestOptionLedgerImportContract(t *testing.T) {
	t.Parallel()
	imp := optionledger.ImportOptionTraderFunc(func(
		_ context.Context, r io.Reader, _ store.OptionLedgerWriter, m map[string]string, _ time.Time,
	) (jobs.ImportResult, error) {
		b, err := io.ReadAll(r)
		if err != nil {
			return jobs.ImportResult{}, err
		}
		lines := strings.Count(string(b), "\n")
		return jobs.ImportResult{Lines: lines, Inserted: len(m)}, nil
	})
	res, err := imp(context.Background(), strings.NewReader("{}\n{}\n"), nil, map[string]string{"x": "a"}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if res != (jobs.ImportResult{Lines: 2, Inserted: 1}) {
		t.Fatalf("result %+v", res)
	}
}
