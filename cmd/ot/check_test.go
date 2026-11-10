package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

const checkSecretDSN = "postgres://ot_ro_pages:s3cretpw@dbhost:5432/optdesk?sslmode=require&password=s3cretpw"

// checkApp is an app with a fixed environment and a recording gap-reader opener.
func checkApp(t *testing.T, env map[string]string, out *bytes.Buffer, read *[]string, opened *[]string) *app {
	t.Helper()
	a, _ := testApp(t, "")
	a.stdout = out
	a.getenv = func(k string) string {
		if read != nil {
			*read = append(*read, k)
		}
		return env[k]
	}
	a.deps.openGapReader = func(_ context.Context, dsn string) (store.GapReader, error) {
		if opened != nil {
			*opened = append(*opened, dsn)
		}
		return nil, nil
	}
	return a
}

func okService(c Result) checkService {
	return func(_ context.Context, e *checkEnv) { e.c.emit(c) }
}

func TestCheckHeaderNoSecret(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	a := checkApp(t, map[string]string{"OT_HOME": "/srv/ot", store.DSNEnvPages: checkSecretDSN}, &out, nil, nil)
	svcs := map[string]checkService{"scheduler": okService(Result{Status: statusPass, Check: "x", Day: "2026-11-06", Reason: "ok"})}
	if err := runCheck(a, []string{"scheduler"}, svcs); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(out.String(), "\n")
	if lines[0] != "OT_HOME=/srv/ot" || lines[1] != "pg=dbhost/optdesk" {
		t.Fatalf("header %q", lines[:2])
	}
	for _, bad := range []string{"ot_ro_pages", "s3cretpw", "sslmode", "5432", "postgres://"} {
		if strings.Contains(out.String(), bad) {
			t.Errorf("output holds %q: %s", bad, out.String())
		}
	}
	if lines[2] != "PASS x 2026-11-06 ok" {
		t.Errorf("result line %q", lines[2])
	}
}

func TestCheckExitOnFail(t *testing.T) {
	t.Parallel()
	env := map[string]string{"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN}
	for _, tc := range []struct {
		r    Result
		code int
	}{
		{Result{Status: statusPass, Check: "c", Reason: "ok"}, 0},
		{Result{Status: statusExplained, Check: "c", Reason: "offline: pass --net"}, 0},
		{Result{Status: statusFail, Check: "c", Reason: "bad"}, 1},
	} {
		var out bytes.Buffer
		a := checkApp(t, env, &out, nil, nil)
		err := runCheck(a, []string{"serve"}, map[string]checkService{"serve": okService(tc.r)})
		code := 0
		var ee *exitError
		if errors.As(err, &ee) {
			code = ee.code
		} else if err != nil {
			t.Fatal(err)
		}
		if code != tc.code {
			t.Errorf("%s: exit %d, want %d", tc.r.Status, code, tc.code)
		}
	}
	// A missing DSN is a FAIL line, not a skip.
	var out bytes.Buffer
	a := checkApp(t, map[string]string{"OT_HOME": "/h"}, &out, nil, nil)
	err := runCheck(a, []string{"serve"}, map[string]checkService{"serve": okService(Result{Status: statusPass, Check: "c"})})
	var ee *exitError
	if !errors.As(err, &ee) || ee.code != 1 || !strings.Contains(out.String(), "FAIL pg - "+store.DSNEnvPages+" is not set") {
		t.Errorf("missing DSN: err %v out %q", err, out.String())
	}
}

func TestCheckExplainedNamed(t *testing.T) {
	t.Parallel()
	for _, r := range []string{"window_open", "closed_day", "non_session_day", "no_view: no cursor view", "offline: pass --net"} {
		var b bytes.Buffer
		if err := report(&b, Result{Status: statusExplained, Check: "c", Day: "2026-11-06", Reason: r}); err != nil {
			t.Errorf("%q rejected: %v", r, err)
		}
	}
	for _, r := range []string{"", "because", "window_opened", "stale"} {
		var b bytes.Buffer
		if err := report(&b, Result{Status: statusExplained, Check: "c", Reason: r}); err == nil || b.Len() != 0 {
			t.Errorf("%q accepted: err %v out %q", r, err, b.String())
		}
	}
	// Through the command a rejected line is a FAIL.
	var out bytes.Buffer
	a := checkApp(t, map[string]string{"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN}, &out, nil, nil)
	err := runCheck(a, []string{"serve"}, map[string]checkService{"serve": okService(Result{Status: statusExplained, Check: "c", Reason: "meh"})})
	if err == nil || !strings.Contains(out.String(), "FAIL c") {
		t.Errorf("err %v out %q", err, out.String())
	}
}

func TestCheckPagesDSNOnly(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	var read, opened []string
	env := map[string]string{
		"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN,
		"OT_PG_DSN_ADMIN": "postgres://admin@x/y", "OT_PG_DSN_RUNNER": "postgres://runner@x/y",
	}
	a := checkApp(t, env, &out, &read, &opened)
	var gotErr error
	svc := func(ctx context.Context, e *checkEnv) {
		_, gotErr = e.openGaps(ctx)
		_ = fredKey(e.a)
	}
	if err := runCheck(a, []string{"scheduler", "--net"}, map[string]checkService{"scheduler": svc}); err != nil {
		t.Fatal(err)
	}
	if gotErr != nil {
		t.Fatal(gotErr)
	}
	if len(opened) != 1 || opened[0] != checkSecretDSN {
		t.Errorf("opened with %q", opened)
	}
	for _, k := range read {
		if k != "OT_HOME" && k != store.DSNEnvPages && k != "FRED_API_KEY" {
			t.Errorf("read the variable %s", k)
		}
	}
}

func TestCheckNetFlagAndFredKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		net  bool
		bad  bool
	}{
		{[]string{"scheduler"}, false, false},
		{[]string{"scheduler", "--net"}, true, false},
		{[]string{"scheduler", "--bogus"}, false, true},
		{nil, false, true},
		{[]string{"nope"}, false, true},
	} {
		var out bytes.Buffer
		a := checkApp(t, map[string]string{"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN, "FRED_API_KEY": "k1"}, &out, nil, nil)
		var got checkOpts
		var key string
		svc := func(_ context.Context, e *checkEnv) { got = e.opts; key = fredKey(e.a) }
		err := runCheck(a, tc.args, map[string]checkService{"scheduler": svc})
		if (err != nil) != tc.bad {
			t.Errorf("%v: err %v", tc.args, err)
		}
		if !tc.bad && (got.Net != tc.net || key != "k1") {
			t.Errorf("%v: net %v key %q", tc.args, got.Net, key)
		}
	}
}

func TestCheckPytestHook(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	var gotDir, gotJunit string
	var gotEnv []string
	want := errors.New("pytest failed")
	a.runPytest = func(dir string, env []string, junit string) error {
		gotDir, gotEnv, gotJunit = dir, env, junit
		return want
	}
	if err := a.pytest("/p", []string{"A=1"}, "/j.xml"); !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
	if gotDir != "/p" || gotJunit != "/j.xml" || len(gotEnv) != 1 {
		t.Errorf("got %q %q %q", gotDir, gotEnv, gotJunit)
	}
}
