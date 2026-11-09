package gaphistory_test

import (
	"context"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/gaphistory"
)

type fakeReader struct {
	rows   []gaphistory.Gap
	closed bool
}

func (f *fakeReader) GapsSince(_ context.Context, since time.Time) ([]gaphistory.Gap, error) {
	out := []gaphistory.Gap{}
	for _, g := range f.rows {
		if !g.FirstSeenAt.Before(since) || (!g.ClearedAt.IsZero() && !g.ClearedAt.Before(since)) {
			out = append(out, g)
		}
	}
	return out, nil
}

func (f *fakeReader) Close() error { f.closed = true; return nil }

var _ gaphistory.Reader = (*fakeReader)(nil)

func TestReaderContract(t *testing.T) {
	t.Parallel()
	t0 := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	rows := []gaphistory.Gap{
		{JobID: "j1", DatasetID: "d1", Day: contracts.Day("2026-11-02"), Reason: "missed", FirstSeenAt: t0, ReportedAt: t0.Add(time.Hour), ClearedAt: t0.Add(2 * time.Hour)},
		{JobID: "j2", DatasetID: "d2", Day: contracts.Day("2026-11-03"), Reason: "failed", FirstSeenAt: t0.Add(24 * time.Hour)},
	}
	var open gaphistory.OpenFunc = func(context.Context, string) (gaphistory.Reader, error) {
		return &fakeReader{rows: rows}, nil
	}
	r, err := open(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.GapsSince(context.Background(), t0.Add(12*time.Hour))
	if err != nil || len(got) != 1 {
		t.Fatalf("got %v, %v", got, err)
	}
	if got[0].JobID != "j2" || !got[0].ClearedAt.IsZero() || !got[0].ReportedAt.IsZero() || got[0].Day != "2026-11-03" {
		t.Fatalf("row %+v", got[0])
	}
	none, err := r.GapsSince(context.Background(), t0.Add(100*time.Hour))
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("want empty non-nil, got %#v, %v", none, err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
}
