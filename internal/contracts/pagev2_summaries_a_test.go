package contracts_test

import "testing"

// The v2 files of held_companies_summary, household_summary and macro_summary
// pass the three outcomes of checkPageV2.
func TestPageV2SummariesA(t *testing.T) {
	t.Parallel()
	overrides := map[string]string{
		"^[A-Za-z0-9_-]{1,64}$":                               "id_1",
		"^[0-9]{4}-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$": "2026-01-02",
	}
	for _, name := range []string{
		"page.held_companies_summary.v2.json",
		"page.household_summary.v2.json",
		"page.macro_summary.v2.json",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkPageV2(t, name, overrides)
		})
	}
}
