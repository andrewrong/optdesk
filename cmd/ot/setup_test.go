package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

func setupApp(t *testing.T, env map[string]string, fn store.SetupFunc) (*app, *bytes.Buffer) {
	t.Helper()
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string { return env[k] }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	a.deps = deps{setup: fn}
	return a, &errb
}

func TestSetupPassesConfig(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	var got store.SetupConfig
	a, _ := setupApp(t, map[string]string{"OT_HOME": home, "OT_ZONE": "UTC", store.AdminDSNEnv: "dsn-x"},
		func(_ context.Context, c store.SetupConfig) error { got = c; return nil })
	if err := runSetup(a, []string{"pg"}); err != nil {
		t.Fatal(err)
	}
	if got != (store.SetupConfig{AdminDSN: "dsn-x", Home: home}) {
		t.Fatalf("config %+v", got)
	}
}

func TestSetupUnsetStopsBeforeCall(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	for name, env := range map[string]map[string]string{
		store.AdminDSNEnv: {"OT_HOME": home, "OT_ZONE": "UTC"},
		"OT_HOME":         {store.AdminDSNEnv: "d"},
	} {
		called := false
		a, _ := setupApp(t, env, func(context.Context, store.SetupConfig) error { called = true; return nil })
		err := runSetup(a, []string{"pg"})
		if err == nil || !strings.Contains(err.Error(), name) || called {
			t.Errorf("%s: err %v called %v", name, err, called)
		}
	}
}

func TestSetupArgumentMustBePg(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{nil, {"mysql"}, {"pg", "x"}} {
		called := false
		a, _ := setupApp(t, map[string]string{"OT_HOME": t.TempDir(), "OT_ZONE": "UTC", store.AdminDSNEnv: "d"},
			func(context.Context, store.SetupConfig) error { called = true; return nil })
		err := runSetup(a, args)
		if err == nil || !strings.Contains(err.Error(), "usage") || called {
			t.Errorf("args %v: err %v called %v", args, err, called)
		}
	}
}

func TestSetupSentinelDSN(t *testing.T) {
	t.Parallel()
	const sentinel = "postgres://sentinel-user:sentinel-pw@h/db"
	a, errb := setupApp(t, map[string]string{"OT_HOME": t.TempDir(), "OT_ZONE": "UTC", store.AdminDSNEnv: sentinel},
		func(context.Context, store.SetupConfig) error { return errors.New("dial " + sentinel) })
	err := runSetup(a, []string{"pg"})
	if err == nil || strings.Contains(err.Error(), "sentinel") || strings.Contains(errb.String(), "sentinel") {
		t.Fatalf("err %v log %q", err, errb.String())
	}
}
