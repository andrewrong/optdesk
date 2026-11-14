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
	sql, err := os.ReadFile("../../../migrations/0043_series_registry.sql")
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range m.Series {
		if slices.Contains(unregistered, id) {
			continue
		}
		if !strings.Contains(string(sql), "'"+id+"'") {
			t.Errorf("series %s is not in the registry seed", id)
		}
	}
}

func TestUnregisteredAreInManifest(t *testing.T) {
	t.Parallel()
	sql, err := os.ReadFile("../../../migrations/0043_series_registry.sql")
	if err != nil {
		t.Fatal(err)
	}
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range unregistered {
		if !slices.Contains(m.Series, id) {
			t.Errorf("unregistered id %s is not in the manifest series", id)
		}
		if strings.Contains(string(sql), "'"+id+"'") {
			t.Errorf("unregistered id %s is in the registry seed", id)
		}
	}
}
