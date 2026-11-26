package datacheck

import (
	"strings"
	"testing"
	"time"
)

func TestManifestLoads(t *testing.T) {
	t.Parallel()
	m, err := DefaultManifest()
	if err != nil {
		t.Fatal(err)
	}
	s := m.Schedule
	if m.ID != "data_check" || s.Calendar != "XNYS" || s.Anchor != "session_close" || s.Zone != "America/New_York" ||
		s.Offset != 120*time.Minute || s.RunBound != 10*time.Minute || s.RestartSensitive {
		t.Fatalf("manifest %+v", m)
	}
}

func TestManifestRefusalNamesKeyNotValue(t *testing.T) {
	t.Parallel()
	_, err := LoadManifest([]byte("id = \"x\"\n[schedule]\ncalendar = \"XNYS\"\nanchor = \"session_close\"\noffset = \"secret-value\"\nrun_bound = \"10m\"\n"))
	if err == nil || !strings.Contains(err.Error(), "schedule.offset") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("err %v", err)
	}
	if _, err := LoadManifest([]byte("[schedule]\n")); err == nil || !strings.Contains(err.Error(), "id") {
		t.Fatalf("err %v", err)
	}
}
