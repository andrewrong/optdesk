package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

// pysideRun runs `ot check pyside` with a fake pytest that writes junit and returns runErr.
func pysideRun(t *testing.T, junit string, runErr error, args ...string) (string, string, []string, error) {
	t.Helper()
	var out bytes.Buffer
	a := checkApp(t, map[string]string{"OT_HOME": "/h", "OT_IBKR_HEALTH_PORT": "4002",
		"OT_PG_PAGES_DSN": "postgres://u:p@h/db"}, &out, nil, nil)
	var gotDir string
	var gotEnv []string
	a.runPytest = func(dir string, env []string, path string) error {
		gotDir, gotEnv = dir, env
		if junit != "" {
			if err := os.WriteFile(path, []byte(junit), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return runErr
	}
	err := commands["check"](a, append([]string{"pyside"}, args...))
	return out.String(), gotDir, gotEnv, err
}

func TestCheckPysideJunit(t *testing.T) {
	t.Parallel()
	junit := `<testsuites><testsuite name="pytest">
<testcase classname="live.t" name="test_a"><properties><property name="as_of" value="2026-11-09"/></properties></testcase>
<testcase classname="live.t" name="test_b"><failure message="bad
quote"/></testcase>
<testcase classname="live.t" name="test_c"><error message="boom"/></testcase>
</testsuite></testsuites>`
	out, dir, env, err := pysideRun(t, junit, errors.New("exit 1"), "--pyside-dir", "py")
	if err == nil {
		t.Error("a FAIL line must give a non-nil error")
	}
	for _, want := range []string{"PASS pyside.test_a 2026-11-09 passed", "FAIL pyside.test_b - failed: bad quote", "FAIL pyside.test_c - error: boom"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
	if dir != "py" {
		t.Errorf("dir %q", dir)
	}
	joined := strings.Join(env, "\n")
	for _, want := range []string{"OT_LIVE_PYSIDE=1", "OT_HOME=/h", "OT_IBKR_HEALTH_PORT=4002"} {
		if !strings.Contains(joined, want) {
			t.Errorf("env lacks %s", want)
		}
	}
}

func TestCheckPysideSkipFails(t *testing.T) {
	t.Parallel()
	junit := `<testsuite><testcase classname="live.t" name="test_a"><skipped message="OT_HOME is not set"/></testcase></testsuite>`
	out, dir, _, err := pysideRun(t, junit, nil)
	if err == nil || !strings.Contains(out, "FAIL pyside.test_a - skipped") {
		t.Errorf("skipped must be FAIL: err %v out %q", err, out)
	}
	if dir != "pyside" {
		t.Errorf("default dir %q", dir)
	}
}

func TestCheckPysideNoSwitch(t *testing.T) {
	t.Parallel()
	// pytest wrote no junit file (it did not run): never a silent pass.
	out, _, env, err := pysideRun(t, "", errors.New("uv: not found"))
	if err == nil || !strings.Contains(out, "FAIL pyside - pytest did not run") {
		t.Errorf("err %v out %q", err, out)
	}
	if !strings.Contains(strings.Join(env, "\n"), "OT_LIVE_PYSIDE=1") {
		t.Error("the switch must be set for pytest")
	}
	// an empty junit file is FAIL as well
	out, _, _, err = pysideRun(t, `<testsuite/>`, nil)
	if err == nil || !strings.Contains(out, "holds no test") {
		t.Errorf("err %v out %q", err, out)
	}
}
