package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type memFence struct {
	lost   chan struct{}
	closed bool
}

func (f *memFence) Lost() <-chan struct{} { return f.lost }
func (f *memFence) Close() error {
	if !f.closed {
		f.closed = true
		close(f.lost)
	}
	return nil
}

type heldError struct{}

func (heldError) Error() string             { return "fence held" }
func (heldError) Kind() contracts.ErrorKind { return contracts.Retryable }

type memInstanceFence struct{ held bool }

func (m *memInstanceFence) TryLock(_ context.Context, _ string) (store.Fence, error) {
	if m.held {
		return nil, heldError{}
	}
	m.held = true
	return &memFence{lost: make(chan struct{})}, nil
}

type memSched struct {
	build  string
	gaps   []store.Gap
	states []store.JobState
	lease  store.Lease
}

func (m *memSched) RenewLease(_ context.Context, l store.Lease, ttl time.Duration) (store.Lease, error) {
	m.lease = store.Lease{Owner: l.Owner, ExpiresAt: time.Unix(0, 0).Add(ttl)}
	return m.lease, nil
}
func (m *memSched) PutGap(_ context.Context, g store.Gap) error {
	m.gaps = append(m.gaps, g)
	return nil
}
func (m *memSched) ClearGap(_ context.Context, g store.Gap) error {
	for i := range m.gaps {
		if m.gaps[i].DatasetID == g.DatasetID && m.gaps[i].Day == g.Day {
			m.gaps[i].ClearedAt = time.Unix(1, 0)
		}
	}
	return nil
}
func (m *memSched) MarkGapReported(_ context.Context, g store.Gap) error {
	for i := range m.gaps {
		if m.gaps[i].DatasetID == g.DatasetID && m.gaps[i].Day == g.Day {
			m.gaps[i].ReportedAt = time.Unix(2, 0)
		}
	}
	return nil
}
func (m *memSched) SetState(_ context.Context, build string) error { m.build = build; return nil }
func (m *memSched) PutJobStates(_ context.Context, s []store.JobState) error {
	m.states = s
	return nil
}
func (m *memSched) Status(context.Context) (store.SchedulerStatus, error) {
	st := store.SchedulerStatus{Build: m.build, Jobs: m.states}
	for _, g := range m.gaps {
		if g.ClearedAt.IsZero() {
			st.Gaps = append(st.Gaps, g)
		}
	}
	return st, nil
}
func (m *memSched) Close() error { return nil }

type memGapReader struct{ gaps []store.Gap }

func (m *memGapReader) OpenGaps(context.Context) ([]store.Gap, error) { return m.gaps, nil }
func (m *memGapReader) Expectations(_ context.Context, from, to contracts.Day) ([]store.Expectation, error) {
	return []store.Expectation{{DatasetID: "d", ExpectedDay: from, PresentDay: to, Closed: false}}, nil
}
func (m *memGapReader) Close() error { return nil }

var _ store.OpenFenceFunc = func(context.Context, string) (store.InstanceFence, error) { return nil, nil }
var _ store.OpenSchedulerLedgerFunc = func(context.Context, string) (store.SchedulerLedger, error) { return nil, nil }
var _ store.OpenGapReaderFunc = func(context.Context, string) (store.GapReader, error) { return nil, nil }

func TestInstanceFenceContract(t *testing.T) {
	t.Parallel()
	var open store.OpenFenceFunc = func(context.Context, string) (store.InstanceFence, error) {
		return &memInstanceFence{}, nil
	}
	f, err := open(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	fence, err := f.TryLock(context.Background(), "b1")
	if err != nil {
		t.Fatal(err)
	}
	var te contracts.TypedError
	if _, err = f.TryLock(context.Background(), "b2"); !errors.As(err, &te) || te.Kind() != contracts.Retryable {
		t.Fatalf("second lock: %v", err)
	}
	select {
	case <-fence.Lost():
		t.Fatal("lost before close")
	default:
	}
	if err = fence.Close(); err != nil {
		t.Fatal(err)
	}
	if err = fence.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	select {
	case <-fence.Lost():
	default:
		t.Fatal("not lost after close")
	}
}

func TestSchedulerLedgerContract(t *testing.T) {
	t.Parallel()
	var open store.OpenSchedulerLedgerFunc = func(context.Context, string) (store.SchedulerLedger, error) {
		return &memSched{}, nil
	}
	l, err := open(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	ctx := context.Background()
	if err = l.SetState(ctx, "b1"); err != nil {
		t.Fatal(err)
	}
	if err = l.PutJobStates(ctx, []store.JobState{{JobID: "j", RestartSensitive: true, DisabledReason: "x"}}); err != nil {
		t.Fatal(err)
	}
	g := store.Gap{DatasetID: "d", Day: "2026-01-02", Reason: "expired"}
	if err = l.PutGap(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err = l.MarkGapReported(ctx, g); err != nil {
		t.Fatal(err)
	}
	st, err := l.Status(ctx)
	if err != nil || st.Build != "b1" || len(st.Jobs) != 1 || len(st.Gaps) != 1 || st.Gaps[0].ReportedAt.IsZero() {
		t.Fatalf("status %+v, %v", st, err)
	}
	if err = l.ClearGap(ctx, g); err != nil {
		t.Fatal(err)
	}
	if st, _ = l.Status(ctx); len(st.Gaps) != 0 {
		t.Fatalf("gap still open: %+v", st.Gaps)
	}
	lease, err := l.RenewLease(ctx, store.Lease{Owner: "o"}, time.Minute)
	if err != nil || lease.Owner != "o" || lease.ExpiresAt.IsZero() {
		t.Fatalf("lease %+v, %v", lease, err)
	}
	if store.RunPartial == store.RunAbandoned || store.RunPartial == store.RunRunning {
		t.Fatal("run statuses collide")
	}
}

func TestGapReaderContract(t *testing.T) {
	t.Parallel()
	var open store.OpenGapReaderFunc = func(context.Context, string) (store.GapReader, error) {
		return &memGapReader{gaps: []store.Gap{{JobID: "j", Reason: "preflight_failed"}}}, nil
	}
	r, err := open(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	gaps, err := r.OpenGaps(context.Background())
	if err != nil || len(gaps) != 1 || gaps[0].JobID != "j" || gaps[0].DatasetID != "" {
		t.Fatalf("gaps %v, %v", gaps, err)
	}
	ex, err := r.Expectations(context.Background(), "2026-01-01", "2026-01-05")
	if err != nil || len(ex) != 1 || ex[0].ExpectedDay != "2026-01-01" || ex[0].PresentDay != "2026-01-05" {
		t.Fatalf("expectations %v, %v", ex, err)
	}
}
