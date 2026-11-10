package positionmarks_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/positionmarks"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/mark"
)

type fakeJob struct{ id string }

func (j fakeJob) ID() string { return j.id }
func (fakeJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (fakeJob) Run(context.Context, contracts.Day, []contracts.ExpectedKey) contracts.JobResult {
	return contracts.JobResult{}
}

// fakeBroker answers one empty Quote per contract asked and counts the calls.
type fakeBroker struct {
	broker.Broker
	snapshots int
}

func (b *fakeBroker) Snapshot(_ context.Context, _ broker.Kind, cs []broker.Contract) ([]broker.Quote, error) {
	b.snapshots++
	return make([]broker.Quote, len(cs)), nil
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

type fakeMarkWriter struct{ rows []mark.PositionMark }

func (w *fakeMarkWriter) WriteMarks(_ context.Context, rows []mark.PositionMark) error {
	w.rows = append(w.rows, rows...)
	return nil
}

// marksJob is a fake position_marks job: one key per alias, in the order given, and on Run one mark per key.
type marksJob struct {
	fakeJob
	aliases []string
	w       mark.Writer
}

func (j marksJob) Plan(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	keys := make([]contracts.ExpectedKey, 0, len(j.aliases))
	for _, a := range j.aliases {
		keys = append(keys, contracts.ExpectedKey{DatasetID: "account.position_mark", SubjectKey: a})
	}
	return keys, "hash", nil
}

func (j marksJob) Run(ctx context.Context, day contracts.Day, keys []contracts.ExpectedKey) contracts.JobResult {
	rows := make([]mark.PositionMark, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, mark.PositionMark{Alias: k.SubjectKey, SessionDay: string(day), Basis: mark.BasisNone})
	}
	if err := j.w.WriteMarks(ctx, rows); err != nil {
		return contracts.JobResult{}
	}
	return contracts.JobResult{Returned: len(rows), Usable: len(rows)}
}

func TestNewPositionMarksFuncContract(t *testing.T) {
	t.Parallel()
	w := &fakeMarkWriter{}
	var newJob positionmarks.NewPositionMarksFunc = func(
		_ broker.Broker, aliases []string, _ store.AccountReader, mw mark.Writer, _ clock.Clock,
	) (contracts.Job, error) {
		return marksJob{fakeJob: fakeJob{"position_marks"}, aliases: aliases, w: mw}, nil
	}
	j, err := newJob(&fakeBroker{}, []string{"main", "second"}, nil, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if j.ID() != "position_marks" {
		t.Fatalf("id %q", j.ID())
	}
	_, got := planAndRun(t, j, "2026-11-06")
	if got.Returned != 2 || len(w.rows) != 2 || w.rows[0].Alias != "main" || w.rows[1].Alias != "second" {
		t.Fatalf("result %+v, rows %+v: the aliases are not marked main first", got, w.rows)
	}
}
