package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type memSeries struct{ rows []contracts.SeriesPoint }

func (m *memSeries) InsertSeriesPoints(_ context.Context, rows []contracts.SeriesPoint) error {
	m.rows = append(m.rows, rows...)
	return nil
}
func (m *memSeries) SeriesPoints(_ context.Context, id string, _, _ contracts.Day) ([]contracts.SeriesPoint, error) {
	var out []contracts.SeriesPoint
	for _, r := range m.rows {
		if r.SeriesID == id {
			out = append(out, r)
		}
	}
	return out, nil
}
func (m *memSeries) Close() error { return nil }

type memDays struct{ rows []contracts.SessionDay }

func (m *memDays) UpsertSessionDays(_ context.Context, rows []contracts.SessionDay) error {
	m.rows = append(m.rows, rows...)
	return nil
}
func (m *memDays) SessionDays(context.Context, string, contracts.Day, contracts.Day) ([]contracts.SessionDay, error) {
	return m.rows, nil
}
func (m *memDays) Close() error { return nil }

type memLedger struct {
	present map[contracts.ExpectedKey]store.PresentStatus
	cursor  store.Cursor
	lease   *store.Lease
}

func (l *memLedger) PutExpected(context.Context, contracts.Day, string, []contracts.ExpectedKey) error {
	return nil
}
func (l *memLedger) Expected(context.Context, contracts.Day) ([]contracts.ExpectedKey, string, error) {
	return nil, "", nil
}
func (l *memLedger) MarkPresent(_ context.Context, _ contracts.Day, k contracts.ExpectedKey, s store.PresentStatus) error {
	l.present[k] = s
	return nil
}
func (l *memLedger) Present(context.Context, contracts.Day) (map[contracts.ExpectedKey]store.PresentStatus, error) {
	return l.present, nil
}
func (l *memLedger) PutWindow(context.Context, store.Window) error { return nil }
func (l *memLedger) Window(context.Context) (store.Window, error)  { return store.Window{}, nil }
func (l *memLedger) SetCursor(_ context.Context, c store.Cursor) error {
	l.cursor = c
	return nil
}
func (l *memLedger) Cursor(context.Context) (store.Cursor, error) { return l.cursor, nil }
func (l *memLedger) Acquire(_ context.Context, owner string, ttl time.Duration) (store.Lease, error) {
	v := store.Lease{Owner: owner, ExpiresAt: time.Unix(0, 0).Add(ttl)}
	l.lease = &v
	return v, nil
}
func (l *memLedger) Release(context.Context, store.Lease) error { l.lease = nil; return nil }
func (l *memLedger) Close() error                               { return nil }

var (
	_ store.SeriesStore        = (*memSeries)(nil)
	_ store.SessionDayStore    = (*memDays)(nil)
	_ store.OpsLedger          = (*memLedger)(nil)
	_ store.OpenSeriesFunc     = func(context.Context, string) (store.SeriesStore, error) { return &memSeries{}, nil }
	_ store.OpenLedgerFunc     = func(context.Context, string) (store.OpsLedger, error) { return nil, nil }
	_ store.SetupFunc          = func(context.Context, store.SetupConfig) error { return nil }
	_ store.OpenSessionDayFunc = func(context.Context, string) (store.SessionDayStore, error) { return &memDays{}, nil }
)

// roundTrip uses the writer and the reader through their interfaces only.
func roundTrip(ctx context.Context, w store.SeriesWriter, r store.SeriesReader) ([]contracts.SeriesPoint, error) {
	if err := w.InsertSeriesPoints(ctx, []contracts.SeriesPoint{{SeriesID: "DGS10"}, {SeriesID: "OTHER"}}); err != nil {
		return nil, err
	}
	return r.SeriesPoints(ctx, "DGS10", "2026-11-01", "2026-11-30")
}

func TestSeriesContract(t *testing.T) {
	t.Parallel()
	s := &memSeries{}
	got, err := roundTrip(context.Background(), s, s)
	if err != nil || len(got) != 1 || got[0].SeriesID != "DGS10" {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestLedgerContract(t *testing.T) {
	t.Parallel()
	var l store.OpsLedger = &memLedger{present: map[contracts.ExpectedKey]store.PresentStatus{}}
	ctx := context.Background()
	k := contracts.ExpectedKey{DatasetID: "d", SubjectKey: "s"}
	if err := l.MarkPresent(ctx, "2026-11-04", k, store.StatusNoValue); err != nil {
		t.Fatal(err)
	}
	p, _ := l.Present(ctx, "2026-11-04")
	if p[k] != store.StatusNoValue {
		t.Fatalf("present = %v", p)
	}
	_ = l.SetCursor(ctx, store.Cursor{Day: "2026-11-04"})
	if c, _ := l.Cursor(ctx); c.Day != "2026-11-04" {
		t.Fatalf("cursor = %v", c)
	}
}
