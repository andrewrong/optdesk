package series

import (
	"context"
	"testing"
	"time"
)

func TestActiveSeriesReadsRegistryRows(t *testing.T) {
	t.Parallel()
	f := newFakeDB()
	first := time.Date(1962, 1, 2, 0, 0, 0, 0, time.UTC)
	f.rows = [][]any{
		{"DGS10", "10-Year", "rates", "daily", "fred", "", "", "", "percent", "", &first, true},
		{"SOFR", "SOFR", "liquidity", "daily", "fred", "", "nyfed", "", "percent", "", (*time.Time)(nil), true},
	}
	got, err := (&seriesStore{db: f}).ActiveSeries(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].FirstObservation != "1962-01-02" || got[1].SecondarySource != "nyfed" ||
		got[1].FirstObservation != "" || got[0].Unit != "percent" {
		t.Errorf("got %+v", got)
	}
}
