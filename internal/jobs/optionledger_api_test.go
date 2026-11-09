package jobs_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

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
	newJob := jobs.NewOptionLedgerJobFunc(func(_ jobs.OptionLedgerStores, cfg jobs.OptionLedgerConfig, _ clock.Clock) (contracts.Job, error) {
		return ledgerJob{logins: len(cfg.Logins)}, nil
	})
	// A login without a token reaches the job with a nil Source; the constructor accepts it.
	cfg := jobs.OptionLedgerConfig{
		Logins:   []jobs.FlexRun{{Name: "main", QueryIDs: []string{"1"}, Required: true}},
		Accounts: map[string]string{"ACC": "a"},
	}
	job, err := newJob(jobs.OptionLedgerStores{}, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if job.ID() != "option_ledger" {
		t.Fatalf("id %q", job.ID())
	}
	if got := job.Run(context.Background(), "2026-11-09", nil).Returned; got != 1 {
		t.Fatalf("logins seen: %d", got)
	}
	var reg jobs.Registry = []contracts.Job{job}
	if len(reg) != 1 {
		t.Fatal("registry")
	}
}

func TestOptionLedgerImportContract(t *testing.T) {
	t.Parallel()
	imp := jobs.ImportOptionTraderFunc(func(
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
