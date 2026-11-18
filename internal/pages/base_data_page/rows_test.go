package base_data_page

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
)

// TestFailedSummaryNamesWarnAndFail: only warn and fail checks are named, with a percent or the days of a gap.
func TestFailedSummaryNamesWarnAndFail(t *testing.T) {
	t.Parallel()
	got := failedSummary([]basecheckapi.Check{
		{Rule: "rows", Level: basecheckapi.Pass},
		{Rule: "coverage_gap", Level: basecheckapi.Warn, Bad: 3},
		{Rule: "crossed", Level: basecheckapi.Fail, Share: "0.021"},
		{Rule: "age", Level: basecheckapi.Fail},
		{Rule: "empty", Level: basecheckapi.Unknown, Share: "0.5"},
	})
	if want := "coverage_gap 3 days; crossed 2.1%; age"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestAgeTextUnit: sessions for a row with a calendar, days for one without.
func TestAgeTextUnit(t *testing.T) {
	t.Parallel()
	r := basecheckapi.Row{AgeDays: 5, AgeSessions: 2}
	if got := ageText(basecheckapi.RowInfo{Calendars: map[string]string{"USD": "XNYS"}}, r); got != "2 s" {
		t.Errorf("with calendar: %s", got)
	}
	if got := ageText(basecheckapi.RowInfo{}, r); got != "5 d" {
		t.Errorf("without calendar: %s", got)
	}
}

// TestUnknownRuleGetsFallbackHint: a rule the table does not know still has a text.
func TestUnknownRuleGetsFallbackHint(t *testing.T) {
	t.Parallel()
	got := checkList("bars", []basecheckapi.Check{{Rule: "mystery", Level: basecheckapi.Unknown}})
	if got[0].HintZh == "" || got[0].Label != "mystery" {
		t.Fatalf("got %+v", got[0])
	}
}

// TestFailedSummaryPercentIsExact: the percent is the share text with the point moved, never float arithmetic.
func TestFailedSummaryPercentIsExact(t *testing.T) {
	t.Parallel()
	got := failedSummary([]basecheckapi.Check{{Rule: "crossed", Level: basecheckapi.Fail, Share: "0.07"}})
	if want := "crossed 7%"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestPercentText: shares of several shapes, and the ones that give no percent.
func TestPercentText(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{"0.021": "2.1", "0.0033": "0.33", "0.5": "50", "1": "100", "0.001": "0.1", "0.0005": "0.05"} {
		if got, ok := percentText(in); !ok || got != want {
			t.Errorf("percentText(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "0", "0.000", "x", "-0.1", "1e-3"} {
		if got, ok := percentText(in); ok {
			t.Errorf("percentText(%q) = %q, want none", in, got)
		}
	}
}
