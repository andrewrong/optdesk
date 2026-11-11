package main

import (
	"os"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
)

func TestNewDepsFilled(t *testing.T) {
	t.Parallel()
	v := reflect.ValueOf(newDeps())
	for i := 0; i < v.NumField(); i++ {
		// openIndexIDs is nil by design until the index-ids-bind issue sets its default.
		if v.Type().Field(i).Name == "openIndexIDs" {
			continue
		}
		if v.Field(i).IsNil() {
			t.Errorf("field %s is nil", v.Type().Field(i).Name)
		}
	}
}

func jobIDs(js []contracts.Job) []string {
	var ids []string
	for _, j := range js {
		ids = append(ids, j.ID())
	}
	return ids
}

func TestRunOnceWiresSixJobs(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}
	if got := jobIDs(r.jobs); !slices.Equal(got, want) {
		t.Fatalf("jobs %v, want %v", got, want)
	}
}

func TestRunOnceNeedsLoaderDSN(t *testing.T) {
	t.Parallel()
	const sentinel = "sentinel-loader-dsn"
	for name, env := range map[string]string{
		"absent": strings.Replace(fullEnv, "OT_PG_DSN_LOADER=loader-dsn\n", "", 1),
		"empty":  strings.Replace(fullEnv, "OT_PG_DSN_LOADER=loader-dsn\n", "OT_PG_DSN_LOADER=\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			r := newRig(t, env)
			err := runOnce(r.a, nil)
			if err == nil || !strings.Contains(err.Error(), store.DSNEnvLoader) || len(r.ev.list()) != 0 {
				t.Fatalf("err %v events %v", err, r.ev.list())
			}
		})
	}
	// The value of the line never reaches the error when its open fails.
	r := newRig(t, strings.Replace(fullEnv, "loader-dsn", sentinel, 1))
	r.fail["sd-loader"] = errorString("dial " + sentinel)
	err := runOnce(r.a, nil)
	if err == nil || !strings.Contains(err.Error(), store.DSNEnvLoader) || strings.Contains(err.Error(), sentinel) {
		t.Fatalf("err %v", err)
	}
}

type errorString string

func (e errorString) Error() string { return string(e) }

func TestDemoProfileOpensNoStore(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]func(*app) error{
		"run-once": func(a *app) error { return runOnce(a, nil) },
		"backfill": func(a *app) error { return backfill(a, []string{"--day", "2026-03-04"}) },
	} {
		t.Run(name, func(t *testing.T) {
			a, _ := testApp(t, fullEnv)
			a.profile.Name = "demo"
			var opened, closed atomic.Int32
			var got contracts.Day
			runOnceFakes(a, &opened, &closed, &got)
			if err := run(a); err == nil || opened.Load() != 0 {
				t.Fatalf("err %v opened %d", err, opened.Load())
			}
		})
	}
	t.Run("serve", demoServeOpensNoStore)
}

func TestEarningsRefreshRegistered(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	body := "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"US\"\nsymbol = \"AAPL\"\nbuy_price = \"150\"\n"
	if err := writeFile(r.a.getenv("OT_HOME")+"/watch.toml", body); err != nil {
		t.Fatal(err)
	}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := jobIDs(r.jobs)
	if len(got) == 0 || got[len(got)-1] != "earnings_refresh" {
		t.Fatalf("jobs %v", got)
	}
	if r.dsn["events"] != "events-dsn" {
		t.Fatalf("events store opened with %q", r.dsn["events"])
	}
	if len(r.earnWatch) != 1 || r.earnWatch[0].Issuer != "cik:0000320193" || r.earnWatch[0].BuyPrice != "150" {
		t.Fatalf("watch list %+v", r.earnWatch)
	}
	if r.avCfg.APIKey != alphaTestKey || r.avCfg.BaseURL != alphaVantageBaseURL {
		t.Fatalf("source config %+v", r.avCfg)
	}
	if len(r.earnSrcs) != 1 {
		t.Fatalf("sources %v", r.earnSrcs)
	}
	if _, ok := r.earnSrcs["JP"]; ok {
		t.Fatal("JP has no source yet: its entries get unsupported rows")
	}

	// backfill takes the same six jobs.
	if err := backfill(r.a, []string{"--day", "2026-11-04"}); err != nil || len(r.jobs) != 6 {
		t.Fatalf("backfill err %v jobs %d", err, len(r.jobs))
	}
}

func TestRunOnceRefusesBadWatchList(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	body := "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"XX\"\n"
	if err := writeFile(r.a.getenv("OT_HOME")+"/watch.toml", body); err != nil {
		t.Fatal(err)
	}
	err := runOnce(r.a, nil)
	if err == nil || !strings.Contains(err.Error(), "cik:0000320193") || strings.Contains(err.Error(), r.a.getenv("OT_HOME")) || len(r.ev.list()) != 0 {
		t.Fatalf("err %v events %v", err, r.ev.list())
	}
}

func TestSetupLineEqualsOwnersetConst(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("../../internal/store/setup/setup.go")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`dsnEnvSettings\s*=\s*"([^"]+)"`).FindSubmatch(src)
	if m == nil || string(m[1]) != ownerset.DSNEnvSettings {
		t.Fatalf("setup literal %q, ownerset %q", m, ownerset.DSNEnvSettings)
	}
}
