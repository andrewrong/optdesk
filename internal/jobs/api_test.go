package jobs_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

// captureOf is the capture id of a key, as the design says: uuid.NewSHA1 over day|dataset|subject.
func captureOf(day contracts.Day, k contracts.ExpectedKey) jobs.CaptureID {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(day)+"|"+k.DatasetID+"|"+k.SubjectKey))
}

// planAndRun is the caller: it plans the job for a day, runs the planned keys, and takes the CaptureID the
// design derives for each key.
func planAndRun(t *testing.T, j contracts.Job, day contracts.Day) ([]jobs.CaptureID, contracts.JobResult) {
	t.Helper()
	keys, _, err := j.Plan(context.Background(), day)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]jobs.CaptureID, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, captureOf(day, k))
	}
	return ids, j.Run(context.Background(), day, keys)
}

type otherKeyJob struct{ fakeJob }

func (otherKeyJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "obs.option_eod", SubjectKey: "QQQ"}}, "", nil
}

func TestCaptureFollowsPlannedKey(t *testing.T) {
	t.Parallel()
	ids, _ := planAndRun(t, otherKeyJob{fakeJob{"option_eod"}}, "2026-11-06")
	want := captureOf("2026-11-06", contracts.ExpectedKey{DatasetID: "obs.option_eod", SubjectKey: "QQQ"})
	if len(ids) != 1 || ids[0] != want {
		t.Fatalf("capture id %v ignores the planned key; want %v", ids, want)
	}
}

// watchLoader is a fake of the cmd/ot loader: a caller reads the prices of an entry from it.
type watchLoader struct{ entries []jobs.WatchEntry }

func (l watchLoader) Load() []jobs.WatchEntry { return l.entries }

func TestWatchEntryPrices(t *testing.T) {
	t.Parallel()
	l := watchLoader{entries: []jobs.WatchEntry{
		{Issuer: "cik:0000320193", Market: "US", BuyPrice: "150.5", SalePrice: "190", Contract: broker.Contract{Symbol: "AAPL"}},
		{Issuer: "edinet:E02144", Market: "JP", Contract: broker.Contract{Symbol: "7203"}},
	}}
	got := l.Load()
	if got[0].BuyPrice != "150.5" || got[0].SalePrice != "190" {
		t.Fatalf("prices %q %q", got[0].BuyPrice, got[0].SalePrice)
	}
	if got[1].BuyPrice != "" || got[1].SalePrice != "" {
		t.Fatal("an unset price is the empty text")
	}
}

func TestImportResultContract(t *testing.T) {
	t.Parallel()
	// one side returns an ImportResult, the other reads its fields
	produce := func() jobs.ImportResult {
		return jobs.ImportResult{Lines: 10, Inserted: 6, Same: 3, Conflict: 1}
	}
	got := produce()
	if got.Lines != 10 || got.Inserted != 6 || got.Same != 3 || got.Conflict != 1 {
		t.Fatalf("fields lost: %+v", got)
	}
	if got.Inserted+got.Same+got.Conflict != got.Lines {
		t.Fatalf("rows do not add up to the lines: %+v", got)
	}
}

func TestNeedFromBothSides(t *testing.T) {
	t.Parallel()
	// one side fills every field, the other reads each back
	declare := func() jobs.Need {
		return jobs.Need{
			Strategy: "s", Group: "watch:options", Rights: []string{"P", "C"}, MinDTE: 21, MaxDTE: 45,
			Ref: "buy_price", Low: "0.80", High: "1.05", Expiry: "monthly", TradingClass: "SPXW",
		}
	}
	n := declare()
	if n.Strategy != "s" || n.Group != "watch:options" || len(n.Rights) != 2 || n.Rights[0] != "P" || n.Rights[1] != "C" {
		t.Fatalf("fields lost: %+v", n)
	}
	if n.MinDTE != 21 || n.MaxDTE != 45 || n.Ref != "buy_price" || n.Low != "0.80" || n.High != "1.05" {
		t.Fatalf("fields lost: %+v", n)
	}
	if n.Expiry != "monthly" || n.TradingClass != "SPXW" {
		t.Fatalf("fields lost: %+v", n)
	}
}
