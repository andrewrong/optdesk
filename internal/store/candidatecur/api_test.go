package candidatecur

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

type fake struct{ rows []Row }

func (f *fake) CandidatesIn(_ context.Context, _ contracts.Day, _, currency string) ([]Row, error) {
	out := []Row{}
	for _, r := range f.rows {
		if r.Currency == currency {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fake) InsertCandidatesIn(_ context.Context, rows []Row) (int, error) {
	f.rows = append(f.rows, rows...)
	return len(rows), nil
}

var (
	_ Reader = (*fake)(nil)
	_ Writer = (*fake)(nil)
)

// TestCandidateCurContractFake: a fake Reader and Writer compile against the interface and a caller drives both.
func TestCandidateCurContractFake(t *testing.T) {
	t.Parallel()
	var w Writer = &fake{}
	f := w.(*fake)
	row := Row{Candidate: store.Candidate{RunID: "r1"}, Currency: "JPY"}
	if n, err := w.InsertCandidatesIn(context.Background(), []Row{row}); err != nil || n != 1 {
		t.Fatalf("insert = %d, %v", n, err)
	}
	var r Reader = f
	got, err := r.CandidatesIn(context.Background(), "2026-11-19", "short_put", "JPY")
	if err != nil || len(got) != 1 || got[0].RunID != "r1" {
		t.Fatalf("read = %+v, %v", got, err)
	}
}
