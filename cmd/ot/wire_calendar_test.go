package main

import (
	"os"
	"path/filepath"
	"regexp"
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
	abs, err := filepath.Abs("pyside")
	if err != nil {
		t.Fatal(err)
	}
	fixed := []string{"run", "--locked", "--project", abs, "python", "-m", "optdesk.calendar"}
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

// init is the one write of hasPyproject, before any test runs.
func init() { hasPyproject = testHasProject }

// testHasProject stands for the pyside directory of the checkout, which the working directory of a test
// (cmd/ot) does not hold; any other directory is looked at for real.
func testHasProject(dir string) bool {
	if filepath.Base(dir) == "pyside" {
		return true
	}
	_, err := os.Stat(filepath.Join(dir, "pyproject.toml"))
	return err == nil
}

func calendarArgv(t *testing.T, env string) ([]string, error) {
	t.Helper()
	r := newRig(t, env)
	err := runOnce(r.a, nil)
	return r.argv, err
}

func TestCalendarCommandIsAbsolute(t *testing.T) {
	t.Parallel()
	argv, err := calendarArgv(t, fullEnv)
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) != 8 || argv[1] != "run" || argv[2] != "--locked" || argv[3] != "--project" {
		t.Fatalf("argv %v", argv)
	}
	abs := argv[4]
	if !filepath.IsAbs(abs) {
		t.Fatalf("project %q is not absolute", abs)
	}
	tail := []string{"--calendar", "XNYS", "--from", "2026-11-09", "--to", "2026-11-11", "--out", "FILE"}
	full := append(slices.Clone(argv), tail...)
	want := append([]string{"uv", "run", "--locked", "--project", abs, "python", "-m", "optdesk.calendar"}, tail...)
	if !slices.Equal(full, want) {
		t.Fatalf("argv %v", full)
	}
	src, err := os.ReadFile(filepath.Join("..", "..", "pyside", "src", "optdesk", "calendar", "cli.py"))
	if err != nil {
		t.Fatal(err)
	}
	declared := map[string]bool{}
	for _, m := range regexp.MustCompile(`add_argument\("(--[a-z]+)"`).FindAllSubmatch(src, -1) {
		declared[string(m[1])] = true
	}
	for _, a := range full[slices.Index(full, "optdesk.calendar")+1:] {
		if strings.HasPrefix(a, "--") && !declared[a] {
			t.Fatalf("flag %s is not declared by cli.py (%v)", a, declared)
		}
	}
	for f := range declared {
		if !slices.Contains(full, f) {
			t.Fatalf("cli.py declares %s, the command does not pass it", f)
		}
	}
}

func TestCalendarProjectLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	argv, err := calendarArgv(t, fullEnv+"OT_CALENDAR_PROJECT="+dir+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if argv[4] != dir {
		t.Fatalf("project %q, want %q", argv[4], dir)
	}
	argv, err = calendarArgv(t, fullEnv)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(argv[4]) || !strings.HasSuffix(argv[4], "/pyside") {
		t.Fatalf("default project %q", argv[4])
	}
}

func TestCalendarProjectMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	r := newRig(t, fullEnv+"OT_CALENDAR_PROJECT="+dir+"\n")
	err := runOnce(r.a, nil)
	if err == nil || !strings.Contains(err.Error(), "OT_CALENDAR_PROJECT") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(err.Error(), dir) {
		t.Fatalf("error holds the path: %v", err)
	}
	if r.argv != nil {
		t.Fatalf("the job was built: %v", r.argv)
	}
}
