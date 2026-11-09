package accountcapture

import (
	"strings"
	"testing"
	"time"
)

func TestEmbeddedSchedule(t *testing.T) {
	t.Parallel()
	m, err := parseManifest(manifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	j := &job{m: m}
	cal, anchor, zone, offset := j.Schedule()
	if cal != "XNYS" || anchor != "session_close" || zone != "" || offset != 30*time.Minute {
		t.Fatalf("Schedule() = (%q, %q, %q, %v)", cal, anchor, zone, offset)
	}
	if j.RunBound() != 10*time.Minute || !j.RestartSensitive() {
		t.Fatalf("RunBound %v RestartSensitive %v", j.RunBound(), j.RestartSensitive())
	}
}

func TestBadScheduleTable(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ table, key string }{
		"unknown anchor": {"calendar = \"XNYS\"\nanchor = \"sunrise\"\n", "schedule.anchor"},
		"no calendar":    {"anchor = \"session_close\"\n", "schedule.calendar"},
		"negative bound": {"calendar = \"XNYS\"\nanchor = \"session_close\"\nrun_bound = \"-7m\"\n", "schedule.run_bound"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := parseManifest([]byte("version = 1\nid = \"account_capture\"\n[schedule]\n" + tc.table))
			if err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
