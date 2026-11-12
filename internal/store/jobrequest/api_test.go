package jobrequest_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/jobrequest"
)

// fakeQueue keeps the promises of the three interfaces over one slice.
type fakeQueue struct {
	rows      []jobrequest.Row
	withdrawn map[int64]bool
	closed    int
}

func newFake() *fakeQueue { return &fakeQueue{withdrawn: map[int64]bool{}} }

func (f *fakeQueue) find(id int64) int {
	for i, r := range f.rows {
		if r.ID == id {
			return i
		}
	}
	return -1
}

// fakeReader is the pages side.
type fakeReader struct{ q *fakeQueue }

func (r fakeReader) Recent(_ context.Context, limit int) ([]jobrequest.Row, error) {
	out := []jobrequest.Row{}
	for i := len(r.q.rows) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, r.q.rows[i])
	}
	return out, nil
}

func (r fakeReader) Get(_ context.Context, id int64) (jobrequest.Row, bool, error) {
	if i := r.q.find(id); i >= 0 {
		return r.q.rows[i], true, nil
	}
	return jobrequest.Row{}, false, nil
}

func (r fakeReader) Close() error { r.q.closed++; return nil }

// fakePoster is the web side.
type fakePoster struct{ fakeReader }

func (p fakePoster) Enqueue(_ context.Context, req jobrequest.Request) (jobrequest.Enqueued, error) {
	for _, r := range p.q.rows {
		if r.Request == req && (r.State == jobrequest.StateQueued || r.State == jobrequest.StateRunning) {
			if p.q.withdrawn[r.ID] {
				return jobrequest.Enqueued{Row: r, Outcome: jobrequest.EnqueueWithdrawing}, nil
			}
			return jobrequest.Enqueued{Row: r, Outcome: jobrequest.EnqueueExisting}, nil
		}
	}
	row := jobrequest.Row{ID: int64(len(p.q.rows) + 1), Request: req, State: jobrequest.StateQueued}
	p.q.rows = append(p.q.rows, row)
	return jobrequest.Enqueued{Row: row, Outcome: jobrequest.EnqueueCreated}, nil
}

func (p fakePoster) Withdraw(_ context.Context, id int64) error {
	p.q.withdrawn[id] = true
	return nil
}

// fakeConsumer is the runner side.
type fakeConsumer struct{ q *fakeQueue }

func (c fakeConsumer) Queued(_ context.Context, limit int) ([]jobrequest.Row, error) {
	out := []jobrequest.Row{}
	for _, r := range c.q.rows {
		if r.State == jobrequest.StateQueued && !c.q.withdrawn[r.ID] && len(out) < limit {
			out = append(out, r)
		}
	}
	return out, nil
}

func (c fakeConsumer) Claim(_ context.Context, id int64, now time.Time) (bool, error) {
	i := c.q.find(id)
	if i < 0 || c.q.rows[i].State != jobrequest.StateQueued || c.q.withdrawn[id] {
		return false, nil
	}
	c.q.rows[i].State, c.q.rows[i].StartedAt = jobrequest.StateRunning, now
	return true, nil
}

func (c fakeConsumer) Hold(_ context.Context, id int64, job string, at time.Time) error {
	if i := c.q.find(id); i >= 0 {
		c.q.rows[i].HoldJob, c.q.rows[i].HoldAt = job, at
	}
	return nil
}

func (c fakeConsumer) Finish(_ context.Context, id int64, s jobrequest.State, reason string, lines []jobrequest.KeyLine, now time.Time) error {
	if i := c.q.find(id); i >= 0 {
		c.q.rows[i].State, c.q.rows[i].Reason, c.q.rows[i].Lines, c.q.rows[i].FinishedAt = s, reason, lines, now
	}
	return nil
}

func (c fakeConsumer) FailRunning(_ context.Context, reason string, now time.Time) error {
	for i := range c.q.rows {
		if c.q.rows[i].State == jobrequest.StateRunning {
			c.q.rows[i].State, c.q.rows[i].Reason, c.q.rows[i].FinishedAt = jobrequest.StateFailed, reason, now
		}
	}
	return nil
}

func (c fakeConsumer) Close() error { c.q.closed++; return nil }

var (
	_ jobrequest.Reader   = fakeReader{}
	_ jobrequest.Poster   = fakePoster{}
	_ jobrequest.Consumer = fakeConsumer{}

	_ jobrequest.OpenReaderFunc   = func(context.Context, string) (jobrequest.Reader, error) { return fakeReader{}, nil }
	_ jobrequest.OpenPosterFunc   = func(context.Context, string) (jobrequest.Poster, error) { return fakePoster{}, nil }
	_ jobrequest.OpenConsumerFunc = func(context.Context, string) (jobrequest.Consumer, error) { return fakeConsumer{}, nil }
)

// useReader lists the newest requests and reads one back.
func useReader(ctx context.Context, r jobrequest.Reader) ([]jobrequest.Row, bool, error) {
	defer func() { _ = r.Close() }()
	rows, err := r.Recent(ctx, 20)
	if err != nil || len(rows) == 0 {
		return rows, false, err
	}
	_, ok, err := r.Get(ctx, rows[0].ID)
	return rows, ok, err
}

// usePoster presses a button twice and withdraws the request.
func usePoster(ctx context.Context, p jobrequest.Poster, req jobrequest.Request) (first, second jobrequest.EnqueueOutcome, err error) {
	defer func() { _ = p.Close() }()
	a, err := p.Enqueue(ctx, req)
	if err != nil {
		return 0, 0, err
	}
	b, err := p.Enqueue(ctx, req)
	if err != nil {
		return 0, 0, err
	}
	return a.Outcome, b.Outcome, p.Withdraw(ctx, a.Row.ID)
}

// useConsumer takes the oldest queued request and finishes it.
func useConsumer(ctx context.Context, c jobrequest.Consumer, now time.Time) (int64, error) {
	defer func() { _ = c.Close() }()
	rows, err := c.Queued(ctx, 1)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	ok, err := c.Claim(ctx, rows[0].ID, now)
	if err != nil || !ok {
		return 0, err
	}
	return rows[0].ID, c.Finish(ctx, rows[0].ID, jobrequest.StateDone, "", []jobrequest.KeyLine{{Dataset: "d", Group: "ran"}}, now)
}

func TestPosterAndConsumerMeetThroughTheQueue(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newFake()
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	req := jobrequest.Request{JobID: "calendar_refresh", Day: "2026-11-12", Force: true}

	first, second, err := usePoster(ctx, fakePoster{fakeReader{q}}, req)
	if err != nil || first != jobrequest.EnqueueCreated || second != jobrequest.EnqueueExisting {
		t.Fatalf("outcomes %v %v err %v", first, second, err)
	}
	// A withdrawn request is not claimed.
	if id, err := useConsumer(ctx, fakeConsumer{q}, now); err != nil || id != 0 {
		t.Fatalf("claimed a withdrawn request: %d %v", id, err)
	}
	other := jobrequest.Request{JobID: "calendar_refresh", Day: "2026-11-11"}
	if _, err := (fakePoster{fakeReader{q}}).Enqueue(ctx, other); err != nil {
		t.Fatal(err)
	}
	if id, err := useConsumer(ctx, fakeConsumer{q}, now); err != nil || id != 2 {
		t.Fatalf("claimed %d %v, want 2", id, err)
	}
	rows, ok, err := useReader(ctx, fakeReader{q})
	if err != nil || !ok || rows[0].ID != 2 || rows[0].State != jobrequest.StateDone || len(rows[0].Lines) != 1 {
		t.Fatalf("rows %+v ok %v err %v", rows, ok, err)
	}
	if q.closed != 4 {
		t.Fatalf("closed %d times, want 4", q.closed)
	}
}

func TestFailRunningEndsOnlyRunningRequests(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	q := newFake()
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	p := fakePoster{fakeReader{q}}
	c := fakeConsumer{q}
	for _, d := range []string{"2026-11-10", "2026-11-11"} {
		if _, err := p.Enqueue(ctx, jobrequest.Request{JobID: "j", Day: d}); err != nil {
			t.Fatal(err)
		}
	}
	if ok, _ := c.Claim(ctx, 1, now); !ok {
		t.Fatal("claim")
	}
	if err := c.FailRunning(ctx, "host_restart", now); err != nil {
		t.Fatal(err)
	}
	a, _, _ := p.Get(ctx, 1)
	b, _, _ := p.Get(ctx, 2)
	if a.State != jobrequest.StateFailed || a.Reason != "host_restart" || b.State != jobrequest.StateQueued {
		t.Fatalf("a %+v b %+v", a, b)
	}
}
