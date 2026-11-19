package option_candidates_page

import (
	"strings"
	"testing"
)

func TestMarksCellShowsOpenSourceHints(t *testing.T) {
	t.Parallel()
	c := cand("short_put", 1, "ok")
	c.Marks = `["eod_fallback","oi_prior_session"]`
	cell := marksCell(c).(valueCell)
	if len(cell.Badges) != 2 {
		t.Fatalf("badges = %v", cell.Badges)
	}
	for i, hint := range []string{
		"Price is from the last close, the open pull had no usable quote for this stock",
		"Open interest is from an earlier session",
	} {
		if !strings.Contains(cell.Badges[i], hint) {
			t.Errorf("badge %d = %q, want hint %q", i, cell.Badges[i], hint)
		}
	}
}

func TestMarksCellUnknownMarkStillAsStored(t *testing.T) {
	t.Parallel()
	c := cand("short_put", 1, "ok")
	c.Marks = `["some_future_mark"]`
	cell := marksCell(c).(valueCell)
	if len(cell.Badges) != 1 || cell.Badges[0] != "some_future_mark" {
		t.Errorf("badges = %v", cell.Badges)
	}
}

func TestMarksCellHidesSourceMarks(t *testing.T) {
	t.Parallel()
	c := cand("short_put", 1, "ok")
	c.Marks = `["buy_from_valuation"]`
	if cell := marksCell(c).(valueCell); len(cell.Badges) != 0 || cell.Text != dash {
		t.Errorf("valuation only: %+v", cell)
	}
	c.Marks = `["buy_from_hand"]`
	if cell := marksCell(c).(valueCell); len(cell.Badges) != 0 || cell.Text != dash {
		t.Errorf("hand only: %+v", cell)
	}
	c.Marks = `["buy_from_valuation","eod_fallback","some_future_mark","buy_from_hand"]`
	cell := marksCell(c).(valueCell)
	if len(cell.Badges) != 2 || !strings.HasPrefix(cell.Badges[0], "eod_fallback") || cell.Badges[1] != "some_future_mark" {
		t.Errorf("mixed: %v", cell.Badges)
	}
}
