package basecheckapi_test

import (
	"slices"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
)

func TestRuleIDsUnique(t *testing.T) {
	t.Parallel()
	seen := map[[2]string]bool{}
	for _, r := range basecheckapi.Rules {
		k := [2]string{r.Row, r.Rule}
		if seen[k] {
			t.Errorf("rule %s of row %s listed twice", r.Rule, r.Row)
		}
		seen[k] = true
	}
}

func TestEveryRuleHasSource(t *testing.T) {
	t.Parallel()
	ok := []string{"sql", "ledger", "derived", "note"}
	rows := map[string]bool{}
	for _, r := range basecheckapi.Rows {
		rows[r.ID] = true
	}
	for _, r := range basecheckapi.Rules {
		if !slices.Contains(ok, r.Source) {
			t.Errorf("rule %s/%s has source %q", r.Row, r.Rule, r.Source)
		}
		if r.Rule == "" || !rows[r.Row] {
			t.Errorf("rule %q names unknown row %q", r.Rule, r.Row)
		}
	}
}

// TestRowsAreTheTwelveOfTheDesign: ids and groups, in order, are the Page table of the design.
func TestRowsAreTheTwelveOfTheDesign(t *testing.T) {
	t.Parallel()
	want := [][2]string{
		{"bars", "Market data"}, {"quotes", "Market data"}, {"chains", "Market data"},
		{"open", "Market data"}, {"contracts", "Market data"},
		{"rates", "Rates and events"}, {"fx", "Rates and events"}, {"events", "Rates and events"},
		{"calendar", "Rates and events"},
		{"facts", "Company facts"},
		{"account", "Account facts"}, {"flex", "Account facts"},
	}
	var got [][2]string
	for _, r := range basecheckapi.Rows {
		got = append(got, [2]string{r.ID, r.Group})
	}
	if !slices.Equal(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// TestCheckedRulesAreNotTableConstraints: rules a table constraint enforces, and the rules the
// design dropped, are not in Rules.
func TestCheckedRulesAreNotTableConstraints(t *testing.T) {
	t.Parallel()
	dropped := []string{
		"bar_positive", "price_positive", "fx_positive", "rate_positive", "value_shape",
		"strike", "multiplier", "nonpositive", "bad_delta", "no_oi", "outside_band",
		"position_zero", "exec_bad", "series_null", "filed_early", "bar_gap",
	}
	for _, r := range basecheckapi.Rules {
		if slices.Contains(dropped, r.Rule) {
			t.Errorf("rule %s of row %s is dropped by the design", r.Rule, r.Row)
		}
	}
}

func TestEveryRowHasAViewGroup(t *testing.T) {
	t.Parallel()
	groups := map[string][]string{
		"bars": {"bars"}, "option": {"quotes", "chains", "open"},
		"other":   {"contracts", "rates", "fx", "events", "calendar", "facts"},
		"account": {"account", "flex"},
	}
	for _, r := range basecheckapi.Rows {
		if r.ViewGroup == "" {
			t.Errorf("row %s has no view group", r.ID)
			continue
		}
		if !slices.Contains(groups[r.ViewGroup], r.ID) {
			t.Errorf("row %s in view group %q, design says otherwise", r.ID, r.ViewGroup)
		}
		if len(r.LedgerDatasets) == 0 {
			t.Errorf("row %s has no ledger dataset", r.ID)
		}
	}
}

// TestRulesHistoryLostNotes: the note rule history_lost is on chains, contracts, calendar, flex and open, and on no other row.
func TestRulesHistoryLostNotes(t *testing.T) {
	t.Parallel()
	var got []string
	for _, r := range basecheckapi.Rules {
		if r.Rule != "history_lost" {
			continue
		}
		if r.Source != "note" {
			t.Errorf("%s.history_lost has source %q", r.Row, r.Source)
		}
		got = append(got, r.Row)
	}
	slices.Sort(got)
	want := []string{"calendar", "chains", "contracts", "flex", "open"}
	if !slices.Equal(got, want) {
		t.Fatalf("rows with history_lost = %v, want %v", got, want)
	}
}
