package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/runner"
	"github.com/andrewrong/optdesk/internal/store"
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
