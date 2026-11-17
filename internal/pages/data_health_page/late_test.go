package data_health_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

func TestLateTextOnFredLag(t *testing.T) {
	t.Parallel()
	days := []contracts.Day{"2026-03-05", "2026-03-06", "2026-03-09", "2026-03-10"}
	vix := func(day contracts.Day) []coverage.Newest { return []coverage.Newest{{SeriesID: "VIXCLS", Day: day}} }
	missing := func(ds string, due string, keys ...string) Cell {
		return Cell{Day: "2026-03-10", State: "missing", Missing: len(keys), MissingKeys: keys, DueText: due}
	}
	tests := []struct {
		name   string
		ds     string
		cell   Cell
		newest []coverage.Newest
		want   string
	}{
		{"fred previous day", fredDataset, missing(fredDataset, "", "VIXCLS"), vix("2026-03-09"), lateText},
		{"fred two sessions back", fredDataset, missing(fredDataset, "", "VIXCLS"), vix("2026-03-06"), ""},
		{"reading key", "readings.reading", missing("readings.reading", "", "market.vix_pctile_3y|market|VIXCLS"), vix("2026-03-09"), lateText},
		{"non-fred key too", "readings.reading", missing("readings.reading", "", "market.vix_pctile_3y|market|VIXCLS", "a.b|c|AAPL"), vix("2026-03-09"), ""},
		{"existing due text", fredDataset, missing(fredDataset, "due 5pm", "VIXCLS"), vix("2026-03-09"), "due 5pm"},
		{"no newest point", fredDataset, missing(fredDataset, "", "VIXCLS"), nil, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cells := []Cell{tc.cell}
			LateText(tc.ds, cells, days, tc.newest)
			if cells[0].DueText != tc.want {
				t.Fatalf("due_text %q, want %q", cells[0].DueText, tc.want)
			}
		})
	}
}
