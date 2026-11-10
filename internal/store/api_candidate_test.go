package store_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// memCandidates is a fake of both interfaces: it keeps every row (insert-only), and answers a day and kind
// with the rows of the run whose ComputedAt is latest, ranked rows first by rank, then the rest by instrument id.
type memCandidates struct{ rows []store.Candidate }

func (m *memCandidates) InsertCandidates(_ context.Context, rows []store.Candidate) (int, error) {
	m.rows = append(m.rows, rows...)
	return len(rows), nil
}

func (m *memCandidates) Candidates(_ context.Context, day contracts.Day, kind string) ([]store.Candidate, error) {
	var newest time.Time
	for _, r := range m.rows {
		if r.SessionDay == day && r.Kind == kind && r.ComputedAt.After(newest) {
			newest = r.ComputedAt
		}
	}
	out := []store.Candidate{}
	for _, r := range m.rows {
		if r.SessionDay == day && r.Kind == kind && r.ComputedAt.Equal(newest) {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Rank == nil) != (b.Rank == nil) {
			return a.Rank != nil
		}
		if a.Rank != nil {
			return *a.Rank < *b.Rank
		}
		return a.InstrumentID < b.InstrumentID
	})
	return out, nil
}

var (
	_ store.CandidateWriter = (*memCandidates)(nil)
	_ store.CandidateReader = (*memCandidates)(nil)
)

func ptrInt(n int) *int { return &n }

// TestCandidateContract drives the fake as the job (writer) and as the page (reader): a day written by two
// runs reads back as the newer run only, ranked rows first.
func TestCandidateContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := contracts.Day("2026-11-09")
	t0 := time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC)
	m := &memCandidates{}
	var w store.CandidateWriter = m
	var r store.CandidateReader = m

	if store.DSNEnvCandidate != "OT_PG_DSN_CANDIDATE" {
		t.Fatalf("DSNEnvCandidate = %q", store.DSNEnvCandidate)
	}
	empty, err := r.Candidates(ctx, day, "short_put")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty day = %v, %v; want empty non-nil slice", empty, err)
	}

	old := []store.Candidate{
		{SessionDay: day, RunID: "run-1", Kind: "short_put", InstrumentID: 1, Status: "ok", Rank: ptrInt(1), ComputedAt: t0},
	}
	if n, err := w.InsertCandidates(ctx, old); err != nil || n != 1 {
		t.Fatalf("first run inserted %d, %v", n, err)
	}
	newer := []store.Candidate{
		{SessionDay: day, RunID: "run-2", Kind: "short_put", InstrumentID: 3, Status: "blocked", ComputedAt: t0.Add(time.Hour)},
		{SessionDay: day, RunID: "run-2", Kind: "short_put", InstrumentID: 9, Status: "ok", Rank: ptrInt(2), ComputedAt: t0.Add(time.Hour)},
		{SessionDay: day, RunID: "run-2", Kind: "short_put", InstrumentID: 7, Status: "ok", Rank: ptrInt(1), ComputedAt: t0.Add(time.Hour)},
		{SessionDay: day, RunID: "run-2", Kind: "covered_call", InstrumentID: 5, Status: "ok", Rank: ptrInt(1), ComputedAt: t0.Add(time.Hour)},
	}
	if n, err := w.InsertCandidates(ctx, newer); err != nil || n != 4 {
		t.Fatalf("second run inserted %d, %v", n, err)
	}

	got, err := r.Candidates(ctx, day, "short_put")
	if err != nil {
		t.Fatal(err)
	}
	var ids []int
	for _, c := range got {
		if c.RunID != "run-2" {
			t.Fatalf("row of %s read, want only the newer run", c.RunID)
		}
		ids = append(ids, c.InstrumentID)
	}
	want := []int{7, 9, 3}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want ranked rows first: %v", ids, want)
		}
	}
}

// TestCandidateContractSettingsVersion holds the settings version from both sides: a writer sets it on the rows of
// a run, a row of an older run keeps nil, and the reader returns each as written.
func TestCandidateContractSettingsVersion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	day := contracts.Day("2026-11-09")
	m := &memCandidates{}
	var w store.CandidateWriter = m
	var r store.CandidateReader = m
	v := int64(12)
	t0 := time.Date(2026, 11, 9, 21, 0, 0, 0, time.UTC)
	if _, err := w.InsertCandidates(ctx, []store.Candidate{{SessionDay: day, RunID: "old", Kind: "short_put", InstrumentID: 1, ComputedAt: t0}}); err != nil {
		t.Fatal(err)
	}
	got, err := r.Candidates(ctx, day, "short_put")
	if err != nil || len(got) != 1 || got[0].SettingsVersion != nil {
		t.Fatalf("old row = %+v, %v; want a nil settings version", got, err)
	}
	if _, err := w.InsertCandidates(ctx, []store.Candidate{{SessionDay: day, RunID: "new", Kind: "short_put", InstrumentID: 1, SettingsVersion: &v, ComputedAt: t0.Add(time.Hour)}}); err != nil {
		t.Fatal(err)
	}
	got, err = r.Candidates(ctx, day, "short_put")
	if err != nil || len(got) != 1 || got[0].SettingsVersion == nil || *got[0].SettingsVersion != 12 {
		t.Fatalf("new row = %+v, %v; want settings version 12", got, err)
	}
}
