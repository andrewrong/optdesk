package optioneod

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/runview"
)

type fakeReasonWriter struct{}

func (fakeReasonWriter) ReplaceReasons(context.Context, contracts.Day, string, string, []runview.Reason) error {
	return nil
}

type foreignJob struct{ contracts.Job }

func TestWithReasonsStoresWriter(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2025-01-06 17:00")
	w := fakeReasonWriter{}
	got, err := WithReasons(r.job, w)
	if err != nil {
		t.Fatal(err)
	}
	if got != contracts.Job(r.job) {
		t.Fatalf("returned a different job")
	}
	if r.job.reasons != runview.ReasonWriter(w) {
		t.Fatalf("writer not stored: %v", r.job.reasons)
	}
}

func TestWithReasonsNilWriterUnchanged(t *testing.T) {
	t.Parallel()
	r := newRig(t, "2025-01-06 17:00")
	got, err := WithReasons(r.job, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != contracts.Job(r.job) || r.job.reasons != nil {
		t.Fatalf("nil writer changed the job")
	}
}

func TestWithReasonsRefusesForeignJob(t *testing.T) {
	t.Parallel()
	got, err := WithReasons(foreignJob{}, fakeReasonWriter{})
	if err == nil || got != nil {
		t.Fatalf("got %v, %v; want nil and an error", got, err)
	}
}
