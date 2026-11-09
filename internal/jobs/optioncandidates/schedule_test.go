package optioncandidates

import (
	"testing"
	"time"
)

func TestScheduleIsCloseAfterEod(t *testing.T) {
	t.Parallel()
	_, sl, p, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if sl.calendar != "XNYS" || sl.anchor != "session_close" || sl.offset != 30*time.Minute || sl.restartSensitive {
		t.Fatalf("slot %+v", sl)
	}
	if p.MinDTE != 21 || p.MaxDTE != 45 || p.MaxSpread != "0.10" || p.MaxRateAgeDays != 7 || p.MinReportGapDays != 60 {
		t.Fatalf("params %+v", p)
	}
}

func TestBadManifestIsRefused(t *testing.T) {
	t.Parallel()
	if _, _, _, err := parseManifest([]byte("id = \"x\"\n")); err == nil {
		t.Fatal("a manifest with no schedule was accepted")
	}
}
