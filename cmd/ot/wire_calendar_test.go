package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestCalendarJobGetsLoaderStore(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	if err := runOnce(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if r.dsn["sd-loader"] != "loader-dsn" || r.dsn["sd"] != "run-dsn" {
		t.Fatalf("loader store opened with %q, window store with %q", r.dsn["sd-loader"], r.dsn["sd"])
	}
	if r.calStore == nil || r.calStore != r.loader {
		t.Fatal("the store handed to calendar_refresh is not the one opened with the loader line")
	}
	if r.compute.SessionDays == nil || r.compute.SessionDays == r.loader {
		t.Fatal("the compute job reads the loader store, not the runner store")
	}
}

func TestCalendarChangeIsLogged(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	if err := runOnce(r.a, nil); err != nil {
		t.Fatal(err)
	}
	if r.observer == nil {
		t.Fatal("no observer")
	}
	after := contracts.SessionDay{CalendarID: "XNYS", IsSession: true, EarlyClose: true}
	r.observer(clock.SessionDayChange{Day: "2026-11-27", After: &after})
	r.observer(clock.SessionDayChange{Day: "2026-11-28"})
	log := r.log.String()
	if n := strings.Count(log, "calendar change"); n != 2 || !strings.Contains(log, "2026-11-27") || !strings.Contains(log, "2026-11-28") {
		t.Fatalf("log %q", log)
	}
}

func TestCalendarUvPathSetting(t *testing.T) {
	t.Parallel()
	fixed := []string{"run", "--locked", "--project", "pyside", "python", "-m", "optdesk.calendar"}
	for name, tc := range map[string]struct{ env, program string }{
		"absent": {fullEnv, "uv"},
		"set":    {fullEnv + "OT_CALENDAR_UV=/opt/tools/uv\n", "/opt/tools/uv"},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, tc.env)
			if err := runOnce(r.a, nil); err != nil {
				t.Fatal(err)
			}
			if len(r.argv) == 0 || r.argv[0] != tc.program || !slices.Equal(r.argv[1:], fixed) {
				t.Fatalf("argv %v", r.argv)
			}
		})
	}
}
