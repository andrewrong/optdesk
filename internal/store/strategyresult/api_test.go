package strategyresult_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/strategyresult"
)

// fakeStore is an in-memory strategyresult.Store that keeps the promises of the interface: insert-only by key,
// reads return the newest run of the day, ranked rows first.
type fakeStore struct{ rows []strategyresult.Row }

var _ strategyresult.Store = (*fakeStore)(nil)

func (f *fakeStore) InsertResults(_ context.Context, rows []strategyresult.Row) (int, error) {
	n := 0
	for _, r := range rows {
		dup := false
		for _, e := range f.rows {
			if e.SessionDay == r.SessionDay && e.RunID == r.RunID && e.Strategy == r.Strategy && e.Key == r.Key {
				dup = true
				break
			}
		}
		if !dup {
			f.rows = append(f.rows, r)
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) Results(_ context.Context, day contracts.Day, strategy string) ([]strategyresult.Row, error) {
	var newest time.Time
	run := ""
	for _, r := range f.rows {
		if r.SessionDay == day && r.Strategy == strategy && (run == "" || r.ComputedAt.After(newest)) {
			newest, run = r.ComputedAt, r.RunID
		}
	}
	out := []strategyresult.Row{}
	for _, r := range f.rows {
		if r.SessionDay == day && r.Strategy == strategy && r.RunID == run {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Rank == nil) != (out[j].Rank == nil) {
			return out[i].Rank != nil
		}
		if out[i].Rank != nil {
			return *out[i].Rank < *out[j].Rank
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func (f *fakeStore) Close() error { return nil }

func TestResultStoreFromBothSides(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := contracts.Day("2026-11-13")
	t0 := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	one, ver := 1, int64(7)
	mk := func(run, key string, rank *int, at time.Time) strategyresult.Row {
		return strategyresult.Row{
			SessionDay: day, RunID: run, Strategy: "short_put", RuleVersion: "put_v1", ParamHash: "abc", Key: key,
			UnderlyingID: 42, Status: "ok", Rank: rank, Legs: "[]", Checks: "[]", Facts: "{}",
			ComputedAt: at, SettingsVersion: &ver,
		}
	}

	var w strategyresult.Writer = &fakeStore{}
	old := []strategyresult.Row{mk("r1", "a", &one, t0)}
	fresh := []strategyresult.Row{mk("r2", "b", nil, t0.Add(time.Hour)), mk("r2", "c", &one, t0.Add(time.Hour))}
	if n, err := w.InsertResults(ctx, old); err != nil || n != 1 {
		t.Fatalf("first insert = %d, %v", n, err)
	}
	if n, err := w.InsertResults(ctx, fresh); err != nil || n != 2 {
		t.Fatalf("second insert = %d, %v", n, err)
	}
	// The same key again inserts nothing and is no error.
	if n, err := w.InsertResults(ctx, fresh); err != nil || n != 0 {
		t.Fatalf("repeat insert = %d, %v", n, err)
	}

	// A caller that holds only the Reader gets the newest run, ranked rows first; a nil Rank stays nil.
	var r strategyresult.Reader = w.(*fakeStore)
	got, err := r.Results(ctx, day, "short_put")
	if err != nil || len(got) != 2 {
		t.Fatalf("results = %+v, %v", got, err)
	}
	if got[0].Key != "c" || got[0].Rank == nil || *got[0].Rank != 1 {
		t.Errorf("first row = %+v, want ranked c", got[0])
	}
	if got[1].Key != "b" || got[1].Rank != nil || got[1].RankInUnderlying != nil {
		t.Errorf("second row = %+v, want unranked b with nil ranks", got[1])
	}
	if none, err := r.Results(ctx, day, "covered_call"); err != nil || len(none) != 0 {
		t.Errorf("other strategy = %+v, %v", none, err)
	}
}
