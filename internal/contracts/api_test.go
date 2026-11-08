package contracts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/contracts/gen"
)

type fakeErr struct{ kind contracts.ErrorKind }

func (e fakeErr) Error() string             { return "fake: " + string(e.kind) }
func (e fakeErr) Kind() contracts.ErrorKind { return e.kind }

type fakeJob struct{ fail bool }

func (fakeJob) ID() string { return "fake" }
func (fakeJob) Plan(_ context.Context, day contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return []contracts.ExpectedKey{{DatasetID: "obs.series_point", SubjectKey: "DGS10:" + string(day)}}, "h1", nil
}
func (j fakeJob) Run(_ context.Context, _ contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	if j.fail {
		return contracts.JobResult{Returned: len(keys), Err: fakeErr{contracts.Retryable}}
	}
	return contracts.JobResult{Returned: len(keys), Usable: len(keys)}
}

// caller uses a Job through the interface only.
func runJob(ctx context.Context, j contracts.Job, day contracts.Day) (contracts.JobResult, string) {
	keys, hash, _ := j.Plan(ctx, day)
	return j.Run(ctx, day, keys), hash
}

func TestJobContract(t *testing.T) {
	t.Parallel()
	var _ contracts.Job = fakeJob{}
	res, hash := runJob(context.Background(), fakeJob{}, "2026-11-04")
	if res.Returned != 1 || res.Usable != 1 || res.Err != nil || hash != "h1" {
		t.Fatalf("unexpected %+v %q", res, hash)
	}
	res, _ = runJob(context.Background(), fakeJob{fail: true}, "2026-11-04")
	var te contracts.TypedError
	if !errors.As(res.Err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("want a Retryable TypedError, got %v", res.Err)
	}
}

func TestRecordsAreGenerated(t *testing.T) {
	t.Parallel()
	v := 4.1
	p := contracts.SeriesPoint{SeriesID: "DGS10", Unit: contracts.SeriesUnit("percent")}
	p.Value = &v
	if p.Value == nil || *p.Value != 4.1 {
		t.Fatal("value is a pointer")
	}
	var d contracts.SessionDay
	d.CalendarID = "XNYS"
	if d.CalendarID != "XNYS" {
		t.Fatal("alias of the generated record")
	}
}

func TestReadingRecordAlias(t *testing.T) {
	t.Parallel()
	var r contracts.ReadingRecord
	g := gen.ReadingRecord(r)
	g.Name = "market.ivp_own"
	g.Inputs = append(g.Inputs, gen.ReadingRecordInputsElem{InputTable: "obs.option_eod"})
	r = g
	if r.Name != "market.ivp_own" || len(r.Inputs) != 1 {
		t.Fatal("alias of the generated record")
	}
}
