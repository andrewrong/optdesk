package quotesnapshot

import (
	"strings"
	"testing"
	"time"
)

func TestEmbeddedSchedule(t *testing.T) {
	t.Parallel()
	j, err := build(manifestBytes, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cal, anchor, zone, offset := j.Schedule()
	if cal != "XNYS" || anchor != "session_close" || zone != "" || offset != -30*time.Minute {
		t.Fatalf("Schedule() = (%q, %q, %q, %v)", cal, anchor, zone, offset)
	}
	if got := j.RunBound(); got != 16*time.Minute {
		t.Fatalf("RunBound() = %v", got)
	}
	if got := j.RestartSensitive(); got != true {
		t.Fatalf("RestartSensitive() = %v", got)
	}
}

func TestBadScheduleTable(t *testing.T) {
	t.Parallel()
	head, _, ok := strings.Cut(string(manifestBytes), "\n[schedule]")
	if !ok {
		t.Fatal("embedded manifest has no [schedule] table")
	}
	cases := []struct {
		name  string
		table string
		key   string
		value string
	}{
		{"unknown anchor", "calendar = \"XNYS\"\nanchor = \"sunrise\"\n", "schedule.anchor", "sunrise"},
		{"time without zone", "calendar = \"XNYS\"\nanchor = \"time\"\noffset = \"7h\"\n", "schedule.zone", ""},
		{"negative bound", "calendar = \"XNYS\"\nanchor = \"session_close\"\nrun_bound = \"-7m\"\n", "schedule.run_bound", "-7m"},
		{"no calendar", "anchor = \"session_close\"\n", "schedule.calendar", ""},
		{"bad offset", "calendar = \"XNYS\"\nanchor = \"session_close\"\noffset = \"soon\"\n", "schedule.offset", "soon"},
		{"zone on a session anchor", "calendar = \"XNYS\"\nanchor = \"session_open\"\nzone = \"UTC\"\n", "schedule.zone", "UTC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := build([]byte(head+"\n[schedule]\n"+tc.table), nil, nil, nil, nil)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error %q does not name %s", err, tc.key)
			}
			if tc.value != "" && strings.Contains(err.Error(), tc.value) {
				t.Fatalf("error %q carries the value", err)
			}
		})
	}
}
