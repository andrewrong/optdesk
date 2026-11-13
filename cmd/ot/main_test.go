package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/runner/sched"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

// fakeClock is a clock fixed at one instant.
type fakeClock struct{ t time.Time }

func (c fakeClock) Now() time.Time { return c.t }

var _ clock.Clock = fakeClock{}

// fakeBackfiller records its calls and answers res and err.
type fakeBackfiller struct {
	mu       sync.Mutex
	days     []contracts.Day
	datasets [][]string
	res      runner.BackfillResult
	err      error
}

func (f *fakeBackfiller) Backfill(_ context.Context, day contracts.Day, datasets []string) (runner.BackfillResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.days = append(f.days, day)
	f.datasets = append(f.datasets, datasets)
	res := f.res
	res.Day = day
	return res, f.err
}

func fakeNewBackfiller(f *fakeBackfiller) runner.NewBackfillerWithRuleFunc {
	return func(store.OpsLedger, clock.Calendar, clock.Clock, []contracts.Job, runner.WindowRule) runner.Backfiller {
		return f
	}
}

// testApp builds an app over a real-profile OT_HOME in a temp dir holding envBody as .env.
// It returns the app and the log output.
// testAcctID is the invented account id of the book of testApp; built from pieces so that no id-shaped
// string is committed.
const testAcctID = "U" + "000" + "0001"

func testApp(t *testing.T, envBody string) (*app, *bytes.Buffer) {
	t.Helper()
	// A short directory: a Unix socket path under the home must stay below 104 bytes, and t.TempDir
	// names are long on macOS.
	home, err := os.MkdirTemp("", "ot")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	if envBody != "" {
		if err := writeFile(home+"/.env", envBody); err != nil {
			t.Fatal(err)
		}
	}
	// The book every real profile needs: alias a keyed to an invented id built at run time.
	if err := writeFile(home+"/"+accountsFileName, "a = \""+testAcctID+"\"\nmain = \"a\"\n"); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"OT_HOME": home, "OT_ZONE": "UTC"}
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string { return env[k] }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	a.deps = deps{
		newClock:      func() clock.Clock { return fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)} },
		newBackfiller: fakeNewBackfiller(&fakeBackfiller{}),
	}
	return a, &errb
}

func TestDemoProfile(t *testing.T) {
	t.Parallel()
	var read []string
	getenv := func(k string) string {
		read = append(read, k)
		return ""
	}
	var out, errb bytes.Buffer
	a, err := newApp(getenv, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	if a.profile.Name != "demo" {
		t.Fatalf("profile %q, want demo", a.profile.Name)
	}
	for _, k := range read {
		if k != "OT_HOME" && k != "OT_PROFILE" {
			t.Errorf("read environment variable %s", k)
		}
	}
	if len(read) == 0 {
		t.Fatal("the profile was not selected from the environment")
	}
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := run([]string{"version"}, func(string) string { return "" }, &out, &errb)
	if code != 0 || !strings.Contains(out.String(), "ot dev (profile demo)") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, out.String(), errb.String())
	}
}

func TestUnknownCommandIsUsageError(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := run([]string{"nope"}, func(string) string { return "" }, &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "version") {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
}

func TestBadProfileStopsStart(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	code := run([]string{"version"}, func(k string) string {
		if k == "OT_PROFILE" {
			return "real"
		}
		return ""
	}, &out, &errb)
	if code != 2 || out.Len() != 0 {
		t.Fatalf("exit %d, stdout %q", code, out.String())
	}
}

func writeFile(path, body string) error { return os.WriteFile(path, []byte(body), 0o600) }

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

// The two commands below exist only in the test binary: one fails with an exitError, one with a plain error.
func init() {
	register("test-exit-75", func(*app, []string) error {
		return fmt.Errorf("outer: %w", &exitError{code: 75, err: errors.New("replaced")})
	})
	register("test-fail", func(*app, []string) error { return errors.New("plain") })
	leakExcluded["test-exit-75"] = "test-only command"
	leakExcluded["test-fail"] = "test-only command"
}

func TestRunHonoursExitCode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		cmd  string
		want int
	}{{"test-exit-75", 75}, {"test-fail", 1}, {"version", 0}} {
		var out, errb bytes.Buffer
		if code := run([]string{tc.cmd}, func(string) string { return "" }, &out, &errb); code != tc.want {
			t.Errorf("%s: exit %d, want %d", tc.cmd, code, tc.want)
		}
	}
}

// healthSched is a Scheduler whose health surface a test sets.
type healthSched struct {
	held bool
	tick time.Time
}

func (h healthSched) Run(context.Context) error { return nil }
func (h healthSched) LastTick() time.Time       { return h.tick }
func (h healthSched) FenceHeld() bool           { return h.held }

func healthGet(t *testing.T, h http.Handler, method, path string) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	return rec.Code
}

func TestSchedulerHealth200WhenFenceHeld(t *testing.T) {
	t.Parallel()
	fw := clockwork.NewFakeClock()
	h := healthHandler(healthSched{held: true, tick: fw.Now()}, fw)
	fw.Advance(59 * time.Second)
	if c := healthGet(t, h, http.MethodGet, "/health"); c != 200 {
		t.Fatalf("status %d", c)
	}
}

func TestSchedulerHealth503WhenTickStale(t *testing.T) {
	t.Parallel()
	fw := clockwork.NewFakeClock()
	s := healthSched{held: true, tick: fw.Now()}
	h := healthHandler(s, fw)
	fw.Advance(60 * time.Second)
	if c := healthGet(t, h, http.MethodGet, "/health"); c != 503 {
		t.Fatalf("stale tick: status %d", c)
	}
	if c := healthGet(t, healthHandler(healthSched{held: false, tick: fw.Now()}, fw), http.MethodGet, "/health"); c != 503 {
		t.Fatalf("fence lost: status %d", c)
	}
}

func TestSchedulerHealthOtherPath404(t *testing.T) {
	t.Parallel()
	fw := clockwork.NewFakeClock()
	h := healthHandler(healthSched{held: true, tick: fw.Now()}, fw)
	if c := healthGet(t, h, http.MethodGet, "/"); c != 404 {
		t.Fatalf("path /: status %d", c)
	}
	if c := healthGet(t, h, http.MethodGet, "/health/x"); c != 404 {
		t.Fatalf("path /health/x: status %d", c)
	}
	for _, m := range []string{http.MethodPost, http.MethodHead, http.MethodDelete} {
		if c := healthGet(t, h, m, "/health"); c != 405 {
			t.Fatalf("%s: status %d", m, c)
		}
	}
}

func TestSchedulerHealth200JustAfterStart(t *testing.T) {
	t.Parallel()
	// A ticker built now reports the instant of construction as its last tick, before its first turn.
	fw := clockwork.NewFakeClock()
	s := sched.New(&fakeFence{lost: make(chan struct{})}, nil, nil, nil, fw, nil, nil, nil, nil, nil)
	if c := healthGet(t, healthHandler(s, fw), http.MethodGet, "/health"); c != 200 {
		t.Fatalf("status %d", c)
	}
}

func TestSchedulerHealthBindsLoopback(t *testing.T) {
	t.Parallel()
	addr, err := healthAddr(func(string) string { return "" })
	if err != nil || addr != "127.0.0.1:8765" {
		t.Fatalf("default address %q, err %v", addr, err)
	}
	ln, err := listenHealth(func(k string) string {
		if k == healthPortEnv {
			return "0"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	ta, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !ta.IP.Equal(net.IPv4(127, 0, 0, 1)) || ta.Port == 0 {
		t.Fatalf("listener %v", ln.Addr())
	}
}

func TestExecIdentityIsStable(t *testing.T) {
	t.Parallel()
	a, err := execIdentity()
	b, err2 := execIdentity()
	if err != nil || err2 != nil || a == "" || a != b {
		t.Fatalf("identity %q %q, errs %v %v", a, b, err, err2)
	}
}
