package recon_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store/recon"
)

type fakeStore struct {
	runs   []recon.Run
	nextID int64
}

func (f *fakeStore) InsertRun(_ context.Context, r recon.Run) (int64, error) {
	if len(r.Items) > recon.RunItemsMax {
		return 0, errors.New("too many items")
	}
	f.nextID++
	f.runs = append(f.runs, r)
	return f.nextID, nil
}

func (f *fakeStore) Latest(context.Context) ([]recon.Run, error) {
	out := make([]recon.Run, len(f.runs))
	copy(out, f.runs)
	return out, nil
}

func (*fakeStore) Close() error { return nil }

var (
	_ recon.Writer = (*fakeStore)(nil)
	_ recon.Reader = (*fakeStore)(nil)
)

func TestFakeWriterAndReader(t *testing.T) {
	t.Parallel()
	st := &fakeStore{}
	openW := recon.OpenWriterFunc(func(context.Context, string) (recon.Writer, error) { return st, nil })
	openR := recon.OpenReaderFunc(func(context.Context, string) (recon.Reader, error) { return st, nil })
	w, err := openW(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	r, err := openR(context.Background(), "dsn")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()

	run := recon.Run{
		PairID: "R6", SessionDay: "2026-11-17", Status: "partial", Reason: "rate_limited",
		StartedAt: time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC), FinishedAt: time.Date(2026, 11, 18, 1, 5, 0, 0, time.UTC),
		N: 9, Matched: 8, Diff: 1, Skipped: 2, PairsAsked: 10, PairsCompared: 9,
		ToleranceText: "0.01", SourceID: "yfinance",
		Items: []recon.Item{{Key: "2026-11-17", Company: "NVDA", Ours: "181.42", Other: "181.50", Diff: "0.08", Kind: "diff"}},
	}
	id, err := w.InsertRun(context.Background(), run)
	if err != nil || id != 1 {
		t.Fatalf("id = %d, err = %v", id, err)
	}
	got, err := r.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].PairID != "R6" || got[0].PairsCompared != 9 || got[0].Reason != "rate_limited" ||
		len(got[0].Items) != 1 || got[0].Items[0].Company != "NVDA" {
		t.Fatalf("latest = %+v", got)
	}
}

func TestLatestItemsMaxIsTwenty(t *testing.T) {
	t.Parallel()
	if recon.LatestItemsMax != 20 || recon.RunItemsMax != 500 {
		t.Fatalf("LatestItemsMax = %d, RunItemsMax = %d, want 20 and 500", recon.LatestItemsMax, recon.RunItemsMax)
	}
}

func TestDSNEnvName(t *testing.T) {
	t.Parallel()
	if recon.DSNEnvRecon != "OT_PG_DSN_RECON" {
		t.Fatalf("DSNEnvRecon = %q", recon.DSNEnvRecon)
	}
}
