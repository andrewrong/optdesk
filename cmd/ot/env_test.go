package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/store"
)

func envApp(t *testing.T, home string, rd store.ReadEnvFunc) *app {
	t.Helper()
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string {
		if k == "OT_HOME" {
			return home
		}
		if k == "OT_ZONE" {
			return "UTC"
		}
		return ""
	}, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	a.deps = deps{readEnv: rd}
	return a
}

func TestEnvFileHomeUnset(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	a := envApp(t, "", func(string) (map[string]string, error) { calls.Add(1); return nil, nil })
	_, err := a.envFile()
	if err == nil || !strings.Contains(err.Error(), "OT_HOME") || calls.Load() != 0 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestEnvFilePassesPath(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	var got string
	a := envApp(t, home, func(p string) (map[string]string, error) { got = p; return map[string]string{"K": "v"}, nil })
	m, err := a.envFile()
	if err != nil || got != home+string(os.PathSeparator)+".env" || m["K"] != "v" {
		t.Fatalf("path %q map %v err %v", got, m, err)
	}
}

func TestEnvFileReturnsReaderError(t *testing.T) {
	t.Parallel()
	want := errors.New("reader failed")
	a := envApp(t, t.TempDir(), func(string) (map[string]string, error) { return map[string]string{"K": "v"}, want })
	m, err := a.envFile()
	if !errors.Is(err, want) || err.Error() != want.Error() || len(m) != 0 {
		t.Fatalf("err %v map %v", err, m)
	}
}

func TestEnvFileNilReaderFallsBack(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".env"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := envApp(t, home, nil)
	m, err := a.envFile()
	if err != nil || m["A"] != "1" {
		t.Fatalf("map %v err %v", m, err)
	}
}

const (
	sentDSN      = "postgres://sentinel-user:sentinel-pw@127.0.0.1:0/db"
	sentPassword = "sentinel-pw"
	sentVerifier = "SCRAM-SHA-256$4096:sentinelsalt$sentinelstored:sentinelserver"
)

var sentinels = []string{sentDSN, sentPassword, sentVerifier}

func leakErr() error { return fmt.Errorf("dial %s pw %s verifier %s", sentDSN, sentPassword, sentVerifier) }

// leakApp builds an app whose env, .env and every opener hold or return the sentinels.
func leakApp(t *testing.T, name string) (*app, *bytes.Buffer) {
	t.Helper()
	env := map[string]string{"OT_HOME": "home", "OT_ZONE": "UTC", store.AdminDSNEnv: sentDSN, "OT_PG_PASSWORD": sentPassword}
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string { return env[k] }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	a.profile.Name = "real"
	a.deps = deps{
		newClock:    func() clock.Clock { return fakeClock{time.Date(2026, 11, 6, 2, 0, 0, 0, time.UTC)} },
		newCalendar: func(clock.SessionSource) clock.Calendar { return nil },
		readEnv: func(string) (map[string]string, error) {
			return map[string]string{store.DSNEnvLoader: sentDSN, store.DSNEnvPages: sentDSN, store.DSNEnvRunner: sentDSN,
				store.DSNEnvSeriesFred: sentDSN, "FRED_API_KEY": "k", "VERIFIER": sentVerifier}, nil
		},
		openSessionDay: func(context.Context, string) (store.SessionDayStore, error) { return nil, leakErr() },
		openLedger:     func(context.Context, string) (store.OpsLedger, error) { return nil, leakErr() },
		openSeries:     func(context.Context, string) (store.SeriesStore, error) { return nil, leakErr() },
		setup:          func(context.Context, store.SetupConfig) error { return leakErr() },
		migrateUp:      func(context.Context, string, fs.FS) error { return leakErr() },
	}
	return a, &errb
}

// leakRows is the table of commands that take a DSN, with the arguments that reach the failing step.
var leakRows = map[string][]string{
	"calendar": {"load", "testdata/calendar_synthetic.json"},
	"migrate":  {"--dir", "."},
	"setup":    {"pg"},
	"page":     {"--day", "2026-03-04"},
	"run-once": {"--day", "2026-03-04"},
	"backfill": {"--day", "2026-03-04"},
}

// leakExcluded names the commands that have no row, each with its reason.
var leakExcluded = map[string]string{
	"serve":   "listens on a port; its DSN path is seriesReader (the page row) and TestServeOnBuildError500 covers its HTTP error path",
	"version": "takes no DSN",
	"help":    "takes no DSN",
}

func TestNoCommandLeaksDSN(t *testing.T) {
	t.Parallel()
	for name, args := range leakRows {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, errb := leakApp(t, name)
			err := commands[name](a, args)
			if err == nil {
				t.Fatal("want an error")
			}
			a.log.Error("command failed", "component", "ot", "err", err.Error())
			for _, s := range sentinels {
				if strings.Contains(err.Error(), s) || strings.Contains(errb.String(), s) {
					t.Fatalf("sentinel %q leaked: %v / %q", s, err, errb.String())
				}
			}
		})
	}
}

func TestLeakTableCoversCommands(t *testing.T) {
	t.Parallel()
	for name := range commands {
		_, row := leakRows[name]
		_, excl := leakExcluded[name]
		if !row && !excl {
			t.Errorf("command %q is neither a row of the leak table nor excluded with a reason", name)
		}
	}
}
