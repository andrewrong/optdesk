package computefigures_test

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/computefigures"
	"github.com/andrewrong/optdesk/internal/store"
)

// fakeIndexIDs answers the index instruments whose name is asked for, as the real reader does.
type fakeIndexIDs struct {
	rows   []store.InstrumentName
	closed bool
}

func (f *fakeIndexIDs) IndexIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	out := []store.InstrumentName{}
	for _, r := range f.rows {
		for _, n := range names {
			if r.Name == n {
				out = append(out, r)
			}
		}
	}
	return out, nil
}

func (f *fakeIndexIDs) Close() error { f.closed = true; return nil }

func TestComputeStoresCarriesIndexIDReader(t *testing.T) {
	t.Parallel()
	fake := &fakeIndexIDs{rows: []store.InstrumentName{{InstrumentID: 7, Name: "SPX"}, {InstrumentID: 8, Name: "NDX"}}}
	stores := computefigures.ComputeStores{IndexIDs: fake}
	got, err := stores.IndexIDs.IndexIDs(context.Background(), []string{"SPX", "RUT"})
	if err != nil || len(got) != 1 || got[0].InstrumentID != 7 {
		t.Fatalf("IndexIDs = %v, %v; want the SPX row only", got, err)
	}
	var open computefigures.OpenIndexIDsFunc = func(context.Context, string) (computefigures.IndexIDStore, error) {
		return fake, nil
	}
	s, err := open(context.Background(), "dsn")
	if err != nil || s.Close() != nil || !fake.closed {
		t.Fatalf("open/close: %v", err)
	}
}
