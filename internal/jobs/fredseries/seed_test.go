package fredseries

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// unregistered are the six series of docs/design/liquidity-view.md §4 (liquidity-view-job)
// that are collected by this job but have no row in 0043_series_registry.sql.
var unregistered = []string{"IORB", "SOFR99", "NFCI", "STLFSI4", "TLAACBW027SBOG", "WLCFLPCL"}

func TestRegistrySeedCoversManifest(t *testing.T) {
	t.Parallel()
	sql := readRegistrySQL(t)
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range m.Series {
		if slices.Contains(unregistered, id) {
			continue
		}
		if !strings.Contains(sql, "'"+id+"'") {
			t.Errorf("series %s is not in the registry seed", id)
		}
	}
}

func TestUnregisteredAreInManifest(t *testing.T) {
	t.Parallel()
	sql := readRegistrySQL(t)
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range unregistered {
		if !slices.Contains(m.Series, id) {
			t.Errorf("unregistered id %s is not in the manifest series", id)
		}
		if strings.Contains(sql, "'"+id+"'") {
			t.Errorf("unregistered id %s is in the registry seed", id)
		}
	}
}

// readRegistrySQL returns the registry seed: 0043 and the macro-page series of 0066.
func readRegistrySQL(t *testing.T) string {
	t.Helper()
	var all strings.Builder
	for _, f := range []string{"0043_series_registry.sql", "0066_macro_series.sql"} {
		b, err := os.ReadFile("../../../migrations/" + f)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(b)
	}
	return all.String()
}
