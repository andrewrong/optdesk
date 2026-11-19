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
