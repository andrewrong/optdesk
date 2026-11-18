package base_data_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// TestFactConversionKeepsEveryField: every field set to a distinct value arrives unchanged.
func TestFactConversionKeepsEveryField(t *testing.T) {
	t.Parallel()
	in := basefact.Fact{Dataset: "quotes", Rule: "crossed", Subject: "USD", N: 11, Bad: 7, Day: "2026-11-17", Value: "SPY last 590.10"}
	want := basecheckapi.Fact{Dataset: "quotes", Rule: "crossed", Subject: "USD", N: 11, Bad: 7, Day: "2026-11-17", Value: "SPY last 590.10"}
	if got := convertFact(in); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestCellsSumLedgerDatasetsPerRow: the three chain datasets sum into the row chains, a day at a time; other
// datasets go to their own rows and an unknown dataset is dropped.
func TestCellsSumLedgerDatasetsPerRow(t *testing.T) {
	t.Parallel()
	cells := []coverage.Cell{
		{Day: "2026-11-16", Dataset: "obs.option_eod", Keys: 10, Missing: 1, Expired: 2},
		{Day: "2026-11-16", Dataset: "obs.option_eod_jp", Keys: 20, Missing: 3, Expired: 0},
		{Day: "2026-11-16", Dataset: "obs.option_eod_index", Keys: 5, Missing: 0, Expired: 4},
		{Day: "2026-11-16", Dataset: "obs.quote_snapshot", Keys: 7},
		{Day: "2026-11-16", Dataset: "readings.reading", Keys: 99},
		{Day: "2026-11-17", Dataset: "obs.option_eod", Keys: 1},
	}
	got := sumCells(cells)
	want := []basecheckapi.Cell{
		{Day: "2026-11-16", Dataset: "quotes", Keys: 7},
		{Day: "2026-11-16", Dataset: "chains", Keys: 35, Missing: 4, Expired: 6},
		{Day: "2026-11-17", Dataset: "chains", Keys: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cell %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}
