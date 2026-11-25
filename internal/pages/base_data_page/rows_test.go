package base_data_page

import (
	"strings"
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
		{Rule: "coverage_latest", Level: basecheckapi.Warn, N: 75, Bad: 4, Share: "0.0533"},
		{Rule: "empty", Level: basecheckapi.Unknown, Share: "0.5"},
	})
	if want := "coverage_gap 3 天; crossed 2.1%; age; coverage_latest 缺失或过期 4 个，共 75 个键（5.33%）"; got != want {
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

// TestCoverageLatestSummaryAndLabel: the missing share is not labelled or summarised as coverage.
func TestCoverageLatestSummaryAndLabel(t *testing.T) {
	t.Parallel()
	checks := []basecheckapi.Check{{Rule: "coverage_latest", Level: basecheckapi.Warn, N: 75, Bad: 4, Share: "0.0533"}}
	list := checkList("bars", checks)
	if list[0].Label == "Latest coverage" || list[0].Label == "" {
		t.Fatalf("label %q", list[0].Label)
	}
	got := failedSummary(checks)
	for _, w := range []string{"缺失或过期 4 个，共 75 个键", "5.33%"} {
		if !strings.Contains(got, w) {
			t.Fatalf("summary %q lacks %q", got, w)
		}
	}
}

// TestFailedSummaryCoverageFormsAreChinese: both coverage forms start with the rule name, hold the numbers and are Chinese.
func TestFailedSummaryCoverageFormsAreChinese(t *testing.T) {
	t.Parallel()
	got := failedSummary([]basecheckapi.Check{
		{Rule: "coverage_gap", Level: basecheckapi.Warn, Bad: 3},
		{Rule: "coverage_latest", Level: basecheckapi.Warn, N: 75, Bad: 4, Share: "0.0533"},
	})
	parts := strings.Split(got, "; ")
	if len(parts) != 2 {
		t.Fatalf("got %q, want two parts", got)
	}
	cases := []struct {
		part, rule string
		nums       []string
	}{
		{parts[0], "coverage_gap", []string{"3"}},
		{parts[1], "coverage_latest", []string{"4", "75", "5.33"}},
	}
	for _, c := range cases {
		if !strings.HasPrefix(c.part, c.rule) {
			t.Errorf("%q does not start with %q", c.part, c.rule)
		}
		if strings.IndexFunc(c.part, func(r rune) bool { return r >= 0x4e00 && r <= 0x9fff }) < 0 {
			t.Errorf("%q holds no Chinese character", c.part)
		}
		for _, n := range c.nums {
			if !strings.Contains(c.part, n) {
				t.Errorf("%q lacks %q", c.part, n)
			}
		}
	}
}

// TestFailedSummaryLatestKeepsMissingBeforeKeyCount: the missing count comes before the key count.
func TestFailedSummaryLatestKeepsMissingBeforeKeyCount(t *testing.T) {
	t.Parallel()
	got := failedSummary([]basecheckapi.Check{
		{Rule: "coverage_latest", Level: basecheckapi.Warn, N: 75, Bad: 4, Share: "0.0533"},
	})
	if i, j := strings.Index(got, " 4 "), strings.Index(got, "75"); i < 0 || j < 0 || i > j {
		t.Fatalf("%q: missing count must come before key count", got)
	}
}
