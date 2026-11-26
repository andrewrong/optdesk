package datacheck

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/finding"
)

func dayOf(s string) contracts.Day { return contracts.Day(s) }

func fresh(rule, ds string, sev finding.Severity) finding.Finding {
	return finding.Finding{Rule: rule, Dataset: ds, Day: "2026-11-25", Severity: sev, IsNew: true,
		Evidence: []finding.Count{{Name: "keys", N: 3}}}
}

func TestAnnounceCapsTenLines(t *testing.T) {
	t.Parallel()
	var fs []finding.Finding
	for i := 0; i < 4; i++ {
		fs = append(fs, fresh(finding.RuleCaptureGap, fmt.Sprintf("w%02d", i), finding.SeverityUnknown))
	}
	for i := 0; i < 4; i++ {
		fs = append(fs, fresh(finding.RuleCaptureGap, fmt.Sprintf("v%02d", i), finding.SeverityWarn))
	}
	for i := 0; i < 5; i++ {
		fs = append(fs, fresh(finding.RuleReadingRepeat, fmt.Sprintf("e%02d", i), finding.SeverityError))
	}
	text, ok := announceText(fs)
	if !ok {
		t.Fatal("no text")
	}
	lines := strings.Split(text, "\n")
	// header, 10 findings, the "left out" line
	if len(lines) != 12 || lines[11] != "还有 3 条见页面" {
		t.Fatalf("lines %d: %q", len(lines), text)
	}
	body := lines[1:11]
	want := []string{"错误", "错误", "错误", "错误", "错误", "缺口", "缺口", "缺口", "缺口", "无法判断"}
	for i, l := range body {
		if !strings.HasPrefix(l, "["+want[i]+"]") {
			t.Fatalf("line %d %q", i, l)
		}
	}
	if !strings.Contains(body[0], "读数重复前一日 e00 2026-11-25 keys=3") {
		t.Fatalf("line %q", body[0])
	}
}

func TestAnnounceNoLeftOutLineWhenTenOrFewer(t *testing.T) {
	t.Parallel()
	var fs []finding.Finding
	for i := 0; i < 10; i++ {
		fs = append(fs, fresh(finding.RulePlanAbsent, fmt.Sprintf("d%d", i), finding.SeverityWarn))
	}
	text, _ := announceText(fs)
	if strings.Contains(text, "还有") {
		t.Fatalf("text %q", text)
	}
}

func TestNoNewNoMessage(t *testing.T) {
	t.Parallel()
	f := fresh(finding.RuleCaptureGap, "a", finding.SeverityWarn)
	f.IsNew = false
	if text, ok := announceText([]finding.Finding{f}); ok || text != "" {
		t.Fatalf("text %q", text)
	}
	if _, ok := announceText(nil); ok {
		t.Fatal("text for nothing")
	}
}

func TestTextHasNoIDs(t *testing.T) {
	t.Parallel()
	rules := []string{finding.RuleReadingRepeat, finding.RuleOpenLateQuote, finding.RuleCandidatesStale,
		finding.RuleCaptureGap, finding.RulePlanAbsent, finding.RuleGapUnclosed}
	var fs []finding.Finding
	for _, r := range rules {
		f := fresh(r, "obs.price_bar", finding.SeverityWarn)
		f.Subject = "subject-x" // a subject never reaches the text
		fs = append(fs, f)
	}
	text, _ := announceText(fs)
	if strings.Contains(text, "subject-x") {
		t.Fatalf("subject in text: %q", text)
	}
	// What is left once the fixed words, the dataset, the day and the evidence names are removed is digits and
	// punctuation only.
	rest := text
	for _, l := range ruleLabels {
		rest = strings.ReplaceAll(rest, l, "")
	}
	for _, w := range []string{"obs.price_bar", "2026-11-25", "keys", "数据检查", "条新发现", "错误", "缺口", "无法判断"} {
		rest = strings.ReplaceAll(rest, w, "")
	}
	for _, r := range rest {
		if unicode.IsLetter(r) {
			t.Fatalf("free word %q in %q", string(r), text)
		}
	}
}
