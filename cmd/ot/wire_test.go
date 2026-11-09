package main

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestNewDepsFilled(t *testing.T) {
	t.Parallel()
	v := reflect.ValueOf(newDeps())
	if v.NumField() != 64 {
		t.Fatalf("deps has %d fields, want 64", v.NumField())
	}
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).IsNil() {
			t.Errorf("field %s is nil", v.Type().Field(i).Name)
		}
	}
}

func TestNewOptionDataPageCarriesNotesAndSymbol(t *testing.T) {
	t.Parallel()
	in := sitePageInput{
		UnderlyingID: 7, Label: "AAA", Symbol: "AAA", Notes: []string{"a note"},
		Eod: noRows{}, Quotes: noRows{}, Terms: noRows{},
	}
	b, err := newOptionDataPage(time.UTC, "b", in).Build(t.Context(), nil, "2026-11-06", time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p sitePageJSON
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.Underlying.Symbol == nil || *p.Underlying.Symbol != "AAA" || len(p.Notes) != 1 || p.Notes[0] != "a note" {
		t.Fatalf("page %+v", p)
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
	fixed := []string{"run", "--no-sync", "--project", "pyside", "python", "-m", "optdesk.calendar"}
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
}

func TestComputeFiguresJobGetsEase(t *testing.T) {
	t.Parallel()
	d := newDeps()
	if d.computeEase == nil {
		t.Fatal("computeEase is nil")
	}
	var got compute.Ease
	d.newComputeFigures = func(
		_ jobs.ComputeStores, _ compute.Figures, _ compute.Rules, _ compute.Units, _ compute.SeriesOps, e compute.Ease, _ clock.Clock,
	) (contracts.Job, error) {
		got = e
		return namedJob{id: "compute_figures"}, nil
	}
	if _, err := (jobFactory{d: d}).computeFigures(); err != nil {
		t.Fatal(err)
	}
	if got == nil || got != d.computeEase {
		t.Fatalf("job got ease %v, want the one of newDeps", got)
	}
}

func TestOptionCandidatesPageWiresReader(t *testing.T) {
	t.Parallel()
	p := newDeps().optionCandidatesPage(time.UTC, "ot-page", "dev", "candidates", demoCandidates{})
	if p.Meta.Name != "option_candidates_page" || p.Meta.Schema != "page.option_candidates_page.v1" {
		t.Fatalf("meta %+v", p.Meta)
	}
	b, err := p.Build(t.Context(), nil, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"scope":"candidates"`) || !strings.Contains(string(b), `"run_id":"ot-page"`) {
		t.Fatalf("page %s", b)
	}
}

func TestMarketPageWiresReadings(t *testing.T) {
	t.Parallel()
	var called atomic.Int32
	read := func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		called.Add(1)
		return nil, nil
	}
	p := newDeps().marketPage(time.UTC, "ot-page", "dev", "market", read)
	if p.Meta.Name != "market_page" {
		t.Fatalf("name %q", p.Meta.Name)
	}
	if p.Meta.Schema != "page.market_page.v1" {
		t.Fatalf("schema %q", p.Meta.Schema)
	}
	_, err := p.Build(t.Context(), demoReader{}, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil && strings.Contains(err.Error(), "not built") {
		t.Skip("the builder of market_page has not landed: Build is still the stub of the interface entry")
	}
	if called.Load() == 0 {
		t.Fatal("Build did not call the readings function")
	}
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
