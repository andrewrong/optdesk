package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/alertrules"
	"github.com/andrewrong/optdesk/internal/notify"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/jonboulle/clockwork"
)

const oneAlertRule = `version = 1
public_subjects = ["SPY"]
[[rule]]
id = "only_rule"
title = "Title"
figure = "market.ivr_own"
figure_version = 1
subject_kind = "underlying"
subject = "SPY"
unit = "decimal"
op = "ge"
threshold = "0.80"
level = "w"
max_data_age_days = 4
`

func TestAlertsOverrideFileReplacesDefaults(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	if err := os.WriteFile(filepath.Join(r.a.getenv("OT_HOME"), "alerts.toml"), []byte(oneAlertRule), 0o600); err != nil {
		t.Fatal(err)
	}
	done := r.start()
	r.waitStarted(t, done)
	_ = r.stop(t, done)
	var got []string
	for _, rule := range r.alertDeps.Rules.Rules {
		got = append(got, rule.ID)
	}
	if !slices.Equal(got, []string{"only_rule"}) {
		t.Fatalf("rules %v, want only the file's rule", got)
	}
}

func TestAlertsBadOverrideStopsStart(t *testing.T) {
	t.Parallel()
	r := newSchedRig(t, fullEnv)
	bad := strings.Replace(oneAlertRule, `op = "ge"`, `op = "sideways"`, 1)
	if err := os.WriteFile(filepath.Join(r.a.getenv("OT_HOME"), "alerts.toml"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	err := <-r.start()
	if err == nil || !strings.Contains(err.Error(), "op") || !strings.Contains(err.Error(), "only_rule") {
		t.Fatalf("err %v, want one naming the key and the rule id", err)
	}
	if strings.Contains(err.Error(), "sideways") || r.inst.count() != 0 || r.sentOpen.Load() != 0 {
		t.Fatalf("err %v, lock tries %d, sent opened %d", err, r.inst.count(), r.sentOpen.Load())
	}
}

func TestAlertsCheckOpensNoDatabase(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	var out bytes.Buffer
	a.stdout = &out
	a.deps.loadAlertRules = alertrules.Load
	a.deps.alertRules = alertrules.DefaultRules
	a.deps.openSent = func(context.Context, string) (store.SentStore, error) {
		t.Error("alerts check opened the sent store")
		return nil, nil
	}
	a.deps.openReadingView = func(context.Context, string) (store.ReadingViewReader, error) {
		t.Error("alerts check opened the reading view")
		return nil, nil
	}
	a.deps.newAlertNotifier = func(notify.AlertDeps, notify.Sender, clockwork.Clock, notify.Config) (notify.AlertNotifier, error) {
		t.Error("alerts check built the notifier")
		return nil, nil
	}
	path := filepath.Join(t.TempDir(), "mine.toml")
	if err := os.WriteFile(path, []byte(oneAlertRule), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := alertsCmd(a, []string{"check", "--file", path}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "rules: 1\nonly_rule\n" {
		t.Fatalf("output %q", got)
	}
	out.Reset()
	if err := alertsCmd(a, []string{"check"}); err != nil || !strings.HasPrefix(out.String(), "rules: ") {
		t.Fatalf("default check: err %v, output %q", err, out.String())
	}
	if err := alertsCmd(a, []string{"check", "--file", path + ".missing"}); err == nil {
		t.Fatal("a named file that is absent must be an error")
	}
	if err := alertsCmd(a, []string{"bogus"}); err == nil {
		t.Fatal("usage error expected")
	}
}

func TestAlertsErrorsHoldNoSecret(t *testing.T) {
	t.Parallel()
	const secret = "postgres://sentinel-user:sentinel-pw@h/db"
	steps := map[string]func(r *schedRig){
		"open sent": func(r *schedRig) {
			r.a.deps.openSent = func(_ context.Context, dsn string) (store.SentStore, error) {
				return nil, os.ErrInvalid
			}
		},
		"open view": func(r *schedRig) {
			r.a.deps.openReadingView = func(_ context.Context, dsn string) (store.ReadingViewReader, error) {
				return nil, &os.PathError{Op: "dial", Path: dsn, Err: os.ErrInvalid}
			}
		},
	}
	env := strings.ReplaceAll(fullEnv, "readings-dsn", secret)
	for name, set := range steps {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newSchedRig(t, env)
			if err := writeFile(r.a.getenv("OT_HOME")+"/.env", env+telegramEnv+"OT_PG_DSN_ALERTS="+secret+"\n"); err != nil {
				t.Fatal(err)
			}
			set(r)
			err := <-r.start()
			if err == nil {
				t.Fatal("no error")
			}
			if strings.Contains(err.Error(), "sentinel") || strings.Contains(r.errb.String(), "sentinel") {
				t.Fatalf("leak: %v / %q", err, r.errb.String())
			}
		})
	}
}

func init() {
	leakExcluded["alerts"] = "check opens no database and takes no DSN; TestAlertsCheckOpensNoDatabase covers it"
}
