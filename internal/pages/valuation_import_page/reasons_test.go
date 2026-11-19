package valuation_import_page

import (
	"strings"
	"testing"
	"unicode"
)

func hasHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// importerTexts pins, as literals, the refusal texts of the importer and of calc.Check.
var importerTexts = []string{
	"R of the row differs from the R of its market in the file",
	"no R version or margin table to compute the lines",
	"the site cannot compute the lines: V or P is out of range",
	"the key picked is not valid or is already on the list",
	"the list row has a market the import does not know",
	"the site computes another buy-zone top",
	"HK and CN differ: the site has one HKCN column",
	"older than stored",
	"unmatched name",
	"review type is required",
	"version date is required",
	"V is required",
	"P is required",
	"V must be a number above 0",
	"P must be from 1 to 5 in steps of 0.5",
	"margin table has a value that is not a number",
	"margin table has no row for P 3.5",
	"margin row from P 2: buy zone upper line is not below the fair zone lower line (12 >= 10)",
}

func TestEveryImporterReasonHasHint(t *testing.T) {
	t.Parallel()
	for _, s := range importerTexts {
		got := reason(s)
		if !hasHan(got) {
			t.Errorf("reason(%q) = %q: no Chinese hint", s, got)
		}
		if !strings.HasPrefix(got, s) {
			t.Errorf("reason(%q) = %q: does not start with the English text", s, got)
		}
	}
}

func TestJoinedCheckProblemsKeepEveryHint(t *testing.T) {
	t.Parallel()
	got := reason("V is required; margin table has no row for P 3.5")
	parts := strings.Split(got, "; ")
	if len(parts) != 2 {
		t.Fatalf("parts = %q", parts)
	}
	for _, p := range parts {
		if !hasHan(p) {
			t.Errorf("part %q has no Chinese hint", p)
		}
	}
}
