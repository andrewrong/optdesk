package mark_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/mark"
)

// fakeStore keeps the promises of the interface in memory.
type fakeStore struct {
	rows   []mark.PositionMark
	closed bool
}

var (
	_ mark.Writer = (*fakeStore)(nil)
	_ mark.Reader = (*fakeStore)(nil)
	_ mark.Store  = (*fakeStore)(nil)
)

func (f *fakeStore) WriteMarks(_ context.Context, rows []mark.PositionMark) error {
	for _, r := range rows {
		exists := false
		for _, o := range f.rows {
			if o.CaptureID == r.CaptureID && o.ConID == r.ConID && o.Attempt == r.Attempt {
				exists = true
			}
		}
		if !exists {
			f.rows = append(f.rows, r)
		}
	}
	return nil
}

func (f *fakeStore) Marks(_ context.Context, alias, day string) ([]mark.PositionMark, error) {
	newest := time.Time{}
	newestID := ""
	for _, r := range f.rows {
		if r.Alias == alias && r.SessionDay == day && (newestID == "" || r.MarkedAt.After(newest)) {
			newest, newestID = r.MarkedAt, r.CaptureID
		}
	}
	best := map[int64]mark.PositionMark{}
	for _, r := range f.rows {
		if r.Alias != alias || r.CaptureID != newestID {
			continue
		}
		if o, ok := best[r.ConID]; !ok || r.Attempt > o.Attempt {
			best[r.ConID] = r
		}
	}
	out := []mark.PositionMark{}
	for _, r := range best {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ConID < out[j].ConID })
	return out, nil
}

func (f *fakeStore) Close() error { f.closed = true; return nil }

func row(capture string, con int64, attempt int, basis, price string, at time.Time) mark.PositionMark {
	return mark.PositionMark{CaptureID: capture, Alias: "main", MarkedAt: at, SessionDay: "2026-11-09",
		ConID: con, Attempt: attempt, Currency: "USD", Basis: basis, Price: price, Status: "ok"}
}

func TestWriterAndReaderFromBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	t0 := time.Date(2026, 11, 9, 15, 0, 0, 0, time.UTC)
	var s mark.Store = &fakeStore{}

	empty, err := s.Marks(ctx, "main", "2026-11-09")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("no rows must be an empty slice, got %#v, %v", empty, err)
	}

	first := []mark.PositionMark{
		row("c1", 20, 1, mark.BasisNone, "", t0),
		row("c1", 10, 1, mark.BasisMid, "1.25", t0),
	}
	if err := s.WriteMarks(ctx, first); err != nil {
		t.Fatal(err)
	}
	// A repeated key is left as it is; a correction is a higher attempt.
	if err := s.WriteMarks(ctx, []mark.PositionMark{
		row("c1", 10, 1, mark.BasisLast, "9", t0),
		row("c1", 20, 2, mark.BasisClose, "3.5", t0.Add(time.Minute)),
	}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Marks(ctx, "main", "2026-11-09")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ConID != 10 || got[1].ConID != 20 {
		t.Fatalf("want rows sorted by ConID 10, 20, got %#v", got)
	}
	if got[0].Basis != mark.BasisMid || got[0].Price != "1.25" {
		t.Fatalf("an existing key must be left as it is: %#v", got[0])
	}
	if got[1].Attempt != 2 || got[1].Basis != mark.BasisClose {
		t.Fatalf("the highest attempt must win: %#v", got[1])
	}
	if other, _ := s.Marks(ctx, "spouse", "2026-11-09"); len(other) != 0 {
		t.Fatalf("accounts are never pooled: %#v", other)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestOpenFuncReturnsStore(t *testing.T) {
	t.Parallel()
	var open mark.OpenFunc = func(_ context.Context, dsn string) (mark.Store, error) {
		if dsn == "" {
			return nil, context.Canceled
		}
		return &fakeStore{}, nil
	}
	if _, err := open(context.Background(), ""); err == nil {
		t.Fatal("an empty DSN must fail")
	}
	s, err := open(context.Background(), "postgres://x")
	if err != nil || s == nil {
		t.Fatalf("open: %v", err)
	}
	if mark.DSNEnvPositionMark != "OT_PG_DSN_POSMARK" {
		t.Fatal("DSNEnvPositionMark changed")
	}
}
