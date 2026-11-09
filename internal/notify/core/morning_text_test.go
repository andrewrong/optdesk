package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/gaphistory"
)

func healed(i int) gaphistory.Gap {
	return gaphistory.Gap{DatasetID: fmt.Sprintf("ds%02d", i), Day: "2026-11-09", Reason: "expired",
		ClearedAt: time.Date(2026, 11, 10, 1, i, 0, 0, time.UTC)}
}

func TestSelfHealedListed(t *testing.T) {
	t.Parallel()
	zone := time.FixedZone("KL", 8*3600)
	text := morningText(morningInput{day: "2026-11-10", zone: zone, selfHealed: []gaphistory.Gap{healed(5)}})
	want := "optdesk morning report 2026-11-10\nself-healed:\ndataset ds05 day 2026-11-09 expired cleared 11-10 09:05\ndetails are on the local pages"
	if text != want {
		t.Fatalf("text =\n%s\nwant\n%s", text, want)
	}
}

func TestMoreThan20LinesCut(t *testing.T) {
	t.Parallel()
	in := morningInput{day: "2026-11-10", zone: time.UTC, open: 2, clearedSent: 1, late: 3, sessionDay: "2026-11-09"}
	for i := 0; i < 40; i++ {
		in.selfHealed = append(in.selfHealed, healed(i))
	}
	text := morningText(in)
	lines := strings.Split(text, "\n")
	if len(lines) != 20 {
		t.Fatalf("lines = %d, want 20:\n%s", len(lines), text)
	}
	if !strings.Contains(text, "\nand 27 more\n") {
		t.Fatalf("no cut line:\n%s", text)
	}
	if !strings.Contains(text, "open gaps: 2") || !strings.HasSuffix(text, "details are on the local pages") {
		t.Fatalf("counts or details line lost:\n%s", text)
	}
}
