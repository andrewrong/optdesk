package jobrun_test

import (
	"context"
	"errors"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrun"
)

// fakeStore is one ledger that is also its own reader, as the table is for both roles.
type fakeStore struct {
	mu   sync.Mutex
	rows map[string]jobrun.JobRun
}

func newFake() *fakeStore { return &fakeStore{rows: map[string]jobrun.JobRun{}} }

func (f *fakeStore) Start(_ context.Context, r jobrun.JobRun) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows[r.ID] = r
	return nil
}

func (f *fakeStore) update(id string, fn func(*jobrun.JobRun)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return errors.New("no such run")
	}
	fn(&r)
	f.rows[id] = r
	return nil
}

func (f *fakeStore) Progress(_ context.Context, id string, done, total int, step string, at time.Time) error {
	return f.update(id, func(r *jobrun.JobRun) {
		r.KeysDone, r.KeysTotal, r.Step, r.HeartbeatAt = done, total, step, at
	})
}

func (f *fakeStore) Close(_ context.Context, id, outcome string, at time.Time) error {
	return f.update(id, func(r *jobrun.JobRun) { r.Outcome, r.FinishedAt = outcome, at })
}

func (f *fakeStore) CloseOpen(_ context.Context, at time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for id, r := range f.rows {
		if r.Outcome == "" {
			r.Outcome, r.FinishedAt = "cut", at
			f.rows[id] = r
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) MarkSlowNotified(_ context.Context, id string, at time.Time) error {
	return f.update(id, func(r *jobrun.JobRun) { r.SlowNotifiedAt = at })
}

func (f *fakeStore) Running(context.Context) ([]jobrun.JobRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []jobrun.JobRun
	for _, r := range f.rows {
		if r.Outcome == "" {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) Usuals(context.Context) ([]jobrun.UsualStat, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	by := map[string][]jobrun.JobRun{}
	for _, r := range f.rows {
		if r.Outcome == "ok" {
			by[r.JobID] = append(by[r.JobID], r)
		}
	}
	var out []jobrun.UsualStat
	for id, rs := range by {
		sort.Slice(rs, func(i, j int) bool { return rs[i].FinishedAt.After(rs[j].FinishedAt) })
		if len(rs) > 20 {
			rs = rs[:20]
		}
		d := make([]time.Duration, len(rs))
		for i, r := range rs {
			d[i] = r.FinishedAt.Sub(r.StartedAt)
		}
		slices.Sort(d)
		var med time.Duration
		if n := len(d); n%2 == 1 {
			med = d[n/2]
		} else {
			med = (d[n/2-1] + d[n/2]) / 2
		}
		out = append(out, jobrun.UsualStat{JobID: id, Median: med, Samples: len(rs)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].JobID < out[j].JobID })
	return out, nil
}

// The runner holds only the ledger, the page only the reader.
func runnerSide(ctx context.Context, l jobrun.JobRunLedger, at time.Time) error {
	if err := l.Start(ctx, jobrun.JobRun{ID: "r1", JobID: "j", Source: "scheduler", StartedAt: at}); err != nil {
		return err
	}
	return l.Progress(ctx, "r1", 2, 5, "k2", at.Add(time.Minute))
}

func TestJobRunContractBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFake()
	var l jobrun.JobRunLedger = f
	var r jobrun.JobRunReader = f
	t0 := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)

	if err := runnerSide(ctx, l, t0); err != nil {
		t.Fatal(err)
	}
	// A second run of the same job id also starts: the row is no guard.
	if err := l.Start(ctx, jobrun.JobRun{ID: "r2", JobID: "j", StartedAt: t0}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Running(ctx)
	if err != nil || len(got) != 2 {
		t.Fatalf("Running = %v, %v; want 2 open runs", got, err)
	}
	if got[0].KeysDone != 2 || got[0].KeysTotal != 5 || got[0].Step != "k2" {
		t.Fatalf("progress not seen by reader: %+v", got[0])
	}
	if err := l.Close(ctx, "r1", "ok", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := l.MarkSlowNotified(ctx, "r2", t0.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ = r.Running(ctx)
	if len(got) != 1 || got[0].ID != "r2" || got[0].SlowNotifiedAt.IsZero() {
		t.Fatalf("after close: %+v", got)
	}
	n, err := l.CloseOpen(ctx, t0.Add(2*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("CloseOpen = %d, %v; want 1", n, err)
	}
	if got, _ = r.Running(ctx); len(got) != 0 {
		t.Fatalf("open runs left: %+v", got)
	}
	if f.rows["r2"].Outcome != "cut" {
		t.Fatalf("outcome = %q, want cut", f.rows["r2"].Outcome)
	}
}

func TestUsualsLeaveOutCutAndFailed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := newFake()
	t0 := time.Date(2026, 3, 4, 12, 0, 0, 0, time.UTC)
	runs := []struct {
		id, outcome string
		dur         time.Duration
	}{
		{"a", "ok", 10 * time.Minute},
		{"b", "ok", 30 * time.Minute},
		{"c", "cut", 3 * time.Hour},
		{"d", "failed", time.Second},
	}
	for i, x := range runs {
		start := t0.Add(time.Duration(i) * time.Hour)
		if err := f.Start(ctx, jobrun.JobRun{ID: x.id, JobID: "j", StartedAt: start}); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(ctx, x.id, x.outcome, start.Add(x.dur)); err != nil {
			t.Fatal(err)
		}
	}
	var r jobrun.JobRunReader = f
	got, err := r.Usuals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := jobrun.UsualStat{JobID: "j", Median: 20 * time.Minute, Samples: 2}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("Usuals = %+v, want [%+v]", got, want)
	}
}
