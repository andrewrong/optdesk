package gaps

import (
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/store"
)

func TestLinesShowsReportedGapWithItsTime(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, 1, 5, 20, 0, 0, 0, time.UTC)
	said := time.Date(2026, 1, 5, 21, 30, 0, 0, time.UTC)
	in := []store.Gap{
		{DatasetID: "fred", Day: "2026-01-05", Reason: "expired", FirstSeenAt: seen},
		{JobID: "option_eod", Day: "2026-01-05", Reason: "preflight_failed", FirstSeenAt: seen, ReportedAt: said},
	}
	got := Lines(in, seen.Add(time.Hour))
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(got), got)
	}
	if !strings.Contains(got[0], "fred") || strings.Contains(got[0], "said at") {
		t.Errorf("unreported line: %q", got[0])
	}
	if !strings.Contains(got[1], "option_eod") || !strings.Contains(got[1], "said at 2026-01-05T21:30:00Z") {
		t.Errorf("reported line: %q", got[1])
	}
}

func TestLinesEmptyGivesNoLines(t *testing.T) {
	t.Parallel()
	if got := Lines(nil, time.Unix(0, 0)); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}
