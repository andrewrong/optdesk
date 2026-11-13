package fredseries

import (
	"os"
	"strings"
	"testing"
)

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
		if !strings.Contains(string(sql), "'"+id+"'") {
			t.Errorf("series %s is not in the registry seed", id)
		}
	}
}
