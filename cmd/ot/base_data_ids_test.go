package main

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/pages/base_data_page"
)

var sqlRuleLine = regexp.MustCompile(`(?m)^\s*--\s*rule\s+(\S+)\s+(\S+)\s*$`)

func TestSqlRuleCommentsEqualRulesTable(t *testing.T) {
	t.Parallel()
	got := map[string]bool{}
	for _, f := range []string{"0056_base_fact_bars.sql", "0057_base_fact_option.sql", "0058_base_fact_other.sql", "0059_base_fact_account.sql"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "migrations", f))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range sqlRuleLine.FindAllStringSubmatch(string(b), -1) {
			got[m[1]+"."+m[2]] = true
		}
	}
	want := map[string]bool{}
	for _, r := range basecheckapi.Rules {
		if r.Source == "sql" {
			want[r.Row+"."+r.Rule] = true
		}
	}
	for id := range want {
		if !got[id] {
			t.Errorf("rule %s is marked sql and has no `-- rule` line", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("`-- rule` line %s is not a rule marked sql", id)
		}
	}
}

func TestEveryRuleAndRowHasZhText(t *testing.T) {
	t.Parallel()
	for _, r := range basecheckapi.Rules {
		if h, ok := base_data_page.Hint(r.Row + "." + r.Rule); !ok || h == "" {
			t.Errorf("rule %s.%s has no hint", r.Row, r.Rule)
		}
	}
	for _, r := range basecheckapi.Rows {
		if h, ok := base_data_page.Hint(r.ID); !ok || h == "" {
			t.Errorf("row %s has no hint", r.ID)
		}
	}
}

// baseDataNamedOut is the fixed list of derived and owner-input datasets that are in no row of the page
// (docs/design/base-data-page.md, Page section).
var baseDataNamedOut = []string{
	"settings.underlying", "readings.reading", "candidates.option_candidate", "strategy.result",
	"plan.option_open_item", "plan.option_open_item.jp",
}

func TestEveryLedgerDatasetIsInARowOrNamedOut(t *testing.T) {
	t.Parallel()
	owner := map[string][]string{}
	for _, r := range basecheckapi.Rows {
		for _, d := range r.LedgerDatasets {
			owner[d] = append(owner[d], r.ID)
		}
	}
	ids := map[string]bool{}
	for _, m := range dataHealthManifests {
		for _, d := range m.datasets {
			ids[d] = true
		}
	}
	var sorted []string
	for d := range ids {
		sorted = append(sorted, d)
	}
	sort.Strings(sorted)
	for _, d := range sorted {
		named := slices.Contains(baseDataNamedOut, d)
		switch {
		case len(owner[d]) > 1:
			t.Errorf("dataset %s is in several rows: %v", d, owner[d])
		case len(owner[d]) == 0 && !named:
			t.Errorf("dataset %s is in no row and not named out", d)
		case len(owner[d]) == 1 && named:
			t.Errorf("dataset %s is in row %s and named out", d, owner[d][0])
		}
	}
}
