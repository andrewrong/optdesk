package optioncandidates

import (
	"context"
	"testing"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

// indexNames names instrument 7, the underlying of the chain rows, as an index called SPX.
type indexNames struct{ *fakes }

func (indexNames) InstrumentNames(context.Context, []int64) ([]store.InstrumentName, error) {
	return []store.InstrumentName{{InstrumentID: 7, Type: "index", Name: "SPX"}}, nil
}

func TestIndexUnderlyingRowsAreNotCandidates(t *testing.T) {
	t.Parallel()
	f := newFakes()
	st := f.stores()
	st.Names = indexNames{f}
	entry := jobs.WatchEntry{Issuer: "cik:0000000002", Market: "US", BuyPrice: "91.5",
		Contract: broker.Contract{Symbol: "SPX", SecType: "STK"}}
	j, err := New(st, f, []jobs.WatchEntry{entry}, fixedClock{})
	if err != nil {
		t.Fatal(err)
	}
	res := j.Run(context.Background(), runDay, nil)
	if res.Err != nil {
		t.Fatalf("result %+v", res)
	}
	rows := 0
	for _, w := range f.written {
		rows += len(w)
	}
	if rows != 0 || len(f.held) != 0 {
		t.Fatalf("rows written for an index instrument: %d rows, %d held", rows, len(f.held))
	}
}
