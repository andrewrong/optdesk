package job_console

import (
	"regexp"
	"testing"
	"unicode"
	"unicode/utf8"
)

// TestJobNoteConstantsAreChineseWithEnglish: each per-job note is Chinese first with the English in
// parentheses, and fits the schema limit counted in runes.
func TestJobNoteConstantsAreChineseWithEnglish(t *testing.T) {
	t.Parallel()
	english := regexp.MustCompile(`\([A-Za-z]`)
	cases := []struct {
		name  string
		text  string
		limit int
	}{
		{"ZhForceMomentMix", ZhForceMomentMix, 600},
		{"ZhForceAppendOnly", ZhForceAppendOnly, 600},
		{"ZhForceInsertOnly", ZhForceInsertOnly, 600},
		{"ZhForceAttemptRows", ZhForceAttemptRows, 600},
		{"ZhForceNotProved", ZhForceNotProved, 600},
		{"ZhWhyNoLowerBound", ZhWhyNoLowerBound, 600},
		{"ZhNeedsLowerBound", ZhNeedsLowerBound, 600},
		{"ZhWhyNoDayGuard", ZhWhyNoDayGuard, 600},
		{"ZhNeedsDayGuard", ZhNeedsDayGuard, 600},
		{"ZhWhyHeavyAndCap", ZhWhyHeavyAndCap, 600},
		{"ZhNeedsHeavyAndCap", ZhNeedsHeavyAndCap, 600},
		{"ZhWhyCachesFailures", ZhWhyCachesFailures, 600},
		{"ZhNeedsCachesFailure", ZhNeedsCachesFailure, 600},
		{"ZhNoteQuoteSnapshot", ZhNoteQuoteSnapshot, 1000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if !hasHan(c.text) {
				t.Errorf("no Chinese character in %q", c.text)
			}
			if !english.MatchString(c.text) {
				t.Errorf("no \"(\" followed by English letters in %q", c.text)
			}
			if n := utf8.RuneCountInString(c.text); n > c.limit {
				t.Errorf("%d runes, limit %d", n, c.limit)
			}
		})
	}
}

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
