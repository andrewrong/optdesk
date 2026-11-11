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

// serviceEnvApp builds a setup app whose readEnv serves files by path and counts its calls; it returns stdout.
func serviceEnvApp(t *testing.T, env map[string]string, files map[string]map[string]string, setupErr error) (*app, *bytes.Buffer, *int) {
	t.Helper()
	var out, errb bytes.Buffer
	a, err := newApp(func(k string) string { return env[k] }, &out, &errb)
	if err != nil {
		t.Fatal(err)
	}
	a.deps = deps{setup: func(context.Context, store.SetupConfig) error { return setupErr }}
	calls := 0
	a.deps.readEnv = func(p string) (map[string]string, error) {
		calls++
		m, ok := files[p]
		if !ok {
			return nil, errors.New("no such file " + p)
		}
		return m, nil
	}
	return a, &out, &calls
}

func serviceEnvSetup(t *testing.T) (env map[string]string, homeEnv, svcEnv string) {
	t.Helper()
	home, secrets := t.TempDir(), t.TempDir()
	env = map[string]string{"OT_HOME": home, "OT_ZONE": "UTC", store.AdminDSNEnv: "d", "CREW_SECRETS": secrets, "CREW_SERVICE": "svc"}
	return env, home + "/.env", secrets + "/svc.env"
}

func TestSetupNamesKeyAbsentFromServiceEnv(t *testing.T) {
	t.Parallel()
	env, homeEnv, svcEnv := serviceEnvSetup(t)
	a, out, _ := serviceEnvApp(t, env, map[string]map[string]string{
		homeEnv: {"OT_PG_DSN_B": "vb", "OT_PG_DSN_C": "vc", "OT_PG_DSN_A": "va", "OTHER": "x"},
		svcEnv:  {"OT_PG_DSN_A": "va"},
	}, nil)
	if err := runSetup(a, []string{"pg"}); err != nil {
		t.Fatal(err)
	}
	want := "setup pg: the service env file " + svcEnv + " lacks or holds differently: OT_PG_DSN_B, OT_PG_DSN_C\n"
	if out.String() != want {
		t.Fatalf("out %q want %q", out.String(), want)
	}
}

func TestSetupNamesKeyWithOtherValueNoValueLogged(t *testing.T) {
	t.Parallel()
	env, homeEnv, svcEnv := serviceEnvSetup(t)
	a, out, _ := serviceEnvApp(t, env, map[string]map[string]string{
		homeEnv: {"OT_PG_DSN_B": "secret-new", "OT_PG_DSN_A": "secret-a"},
		svcEnv:  {"OT_PG_DSN_B": "secret-old", "OT_PG_DSN_A": "secret-a"},
	}, nil)
	if err := runSetup(a, []string{"pg"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "OT_PG_DSN_B") || strings.Contains(out.String(), "OT_PG_DSN_A") || strings.Contains(out.String(), "secret") {
		t.Fatalf("out %q", out.String())
	}
}

func TestSetupEqualServiceEnvWritesNothing(t *testing.T) {
	t.Parallel()
	env, homeEnv, svcEnv := serviceEnvSetup(t)
	a, out, _ := serviceEnvApp(t, env, map[string]map[string]string{
		homeEnv: {"OT_PG_DSN_A": "va"},
		svcEnv:  {"OT_PG_DSN_A": "va", "EXTRA": "y"},
	}, nil)
	if err := runSetup(a, []string{"pg"}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("out %q", out.String())
	}
}

func TestSetupSecretsUnsetReadsNothing(t *testing.T) {
	t.Parallel()
	env, homeEnv, svcEnv := serviceEnvSetup(t)
	delete(env, "CREW_SECRETS")
	a, out, calls := serviceEnvApp(t, env, map[string]map[string]string{
		homeEnv: {"OT_PG_DSN_A": "va"}, svcEnv: {},
	}, nil)
	if err := runSetup(a, []string{"pg"}); err != nil {
		t.Fatal(err)
	}
	if *calls != 0 || out.Len() != 0 {
		t.Fatalf("calls %d out %q", *calls, out.String())
	}
}

func TestSetupServiceEnvReadErrorIsWarning(t *testing.T) {
	t.Parallel()
	env, homeEnv, svcEnv := serviceEnvSetup(t)
	a, out, _ := serviceEnvApp(t, env, map[string]map[string]string{homeEnv: {"OT_PG_DSN_A": "va"}}, nil)
	if err := runSetup(a, []string{"pg"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), svcEnv) || !strings.Contains(out.String(), "no such file") {
		t.Fatalf("out %q", out.String())
	}
}

func TestSetupFailureWritesNoServiceEnvLine(t *testing.T) {
	t.Parallel()
	env, homeEnv, svcEnv := serviceEnvSetup(t)
	a, out, calls := serviceEnvApp(t, env, map[string]map[string]string{
		homeEnv: {"OT_PG_DSN_A": "va"}, svcEnv: {},
	}, errors.New("boom"))
	if err := runSetup(a, []string{"pg"}); err == nil {
		t.Fatal("want error")
	}
	if *calls != 0 || out.Len() != 0 {
		t.Fatalf("calls %d out %q", *calls, out.String())
	}
}
