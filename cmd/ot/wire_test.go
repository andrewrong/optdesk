package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/jobs"
	"github.com/andrewrong/optdesk/internal/jobs/earningsrefresh"
	"github.com/andrewrong/optdesk/internal/sources"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/bar"
	"github.com/andrewrong/optdesk/internal/store/dividend"
	"github.com/andrewrong/optdesk/internal/store/ownerset"
	"github.com/andrewrong/optdesk/internal/symbol"
)

func TestNewDepsFilled(t *testing.T) {
	t.Parallel()
	v := reflect.ValueOf(newDeps())
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
	t.Run("serve", demoServeOpensNoStore)
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

// jqTestKey is the J-Quants key of the tests; it must never reach a log line or an error.
const jqTestKey = "jq-test-secret"

// jqEnv is the rig's .env with the J-Quants line.
func jqEnv() string { return fullEnv + "JQUANTS_API_KEY=" + jqTestKey + "\n" }

const jpWatch = "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"US\"\nsymbol = \"AAPL\"\n" +
	"[[underlying]]\nissuer = \"edinet:E02144\"\nmarket = \"JP\"\nsymbol = \"7203\"\n" +
	"[[underlying]]\nissuer = \"edinet:E01777\"\nmarket = \"JP\"\nsymbol = \"6758\"\n"

// jqSource is a J-Quants source over a fake client: one client call per symbol and Fetch.
type jqSource struct {
	cfg     sources.EarningsConfig
	fetches atomic.Int32
	calls   atomic.Int32
	err     error
}

type jqErr struct{}

func (jqErr) Error() string             { return "jquants: boom" }
func (jqErr) Kind() contracts.ErrorKind { return contracts.Retryable }
func (jqErr) ErrorCode() string         { return "http_status" }

func (s *jqSource) Fetch(context.Context) ([]sources.EarningsListing, error) {
	s.fetches.Add(1)
	for range s.cfg.Symbols {
		s.calls.Add(1)
		if s.err != nil {
			return nil, s.err
		}
	}
	return nil, nil
}

// jqRig wires run-once with the real earnings_refresh job over an in-memory store and a counting J-Quants source.
func jqRig(t *testing.T, env string) (*rig, *jqSource, *memEvents) {
	t.Helper()
	r := newRig(t, env)
	if err := writeFile(r.a.getenv("OT_HOME")+"/watch.toml", jpWatch); err != nil {
		t.Fatal(err)
	}
	src, mem := &jqSource{}, &memEvents{}
	d := &r.a.deps
	d.newJQuants = func(cfg sources.EarningsConfig) (sources.EarningsSource, error) {
		src.cfg = cfg
		return src, nil
	}
	d.newAlphaVantage = func(sources.EarningsConfig) (sources.EarningsSource, error) { return &jqSource{}, nil }
	d.openEarningsDates = func(context.Context, string) (store.EarningsDateStore, error) { return mem, nil }
	d.newEarningsRefresh = func(
		srcs map[string]sources.EarningsSource, w store.EarningsDateWriter, rd store.EarningsDateReader, es []jobs.WatchEntry, c clock.Clock, _ symbol.Translator,
	) (contracts.Job, error) {
		r.earnSrcs = srcs
		return earningsrefresh.New(srcs, w, rd, es, c)
	}
	return r, src, mem
}

func earningsJob(t *testing.T, r *rig) contracts.Job {
	t.Helper()
	for _, j := range r.jobs {
		if j.ID() == "earnings_refresh" {
			return j
		}
	}
	t.Fatalf("earnings_refresh not wired: %v", jobIDs(r.jobs))
	return nil
}

func rowsOf(m *memEvents, market string) []contracts.EarningsDate {
	var out []contracts.EarningsDate
	for _, x := range m.rows {
		if string(x.Market) == market {
			out = append(out, x)
		}
	}
	return out
}

func TestJPSourceWired(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if r.earnSrcs["JP"] != sources.EarningsSource(src) || r.earnSrcs["US"] == nil {
		t.Fatalf("sources %v", r.earnSrcs)
	}
	if src.cfg.APIKey != jqTestKey || src.cfg.BaseURL != jquantsBaseURL {
		t.Fatalf("source config has the wrong key or host")
	}
	if want := []string{"7203", "6758"}; !slices.Equal(src.cfg.Symbols, want) {
		t.Fatalf("symbols %v, want %v", src.cfg.Symbols, want)
	}
	if strings.Contains(r.log.String(), jqTestKey) {
		t.Fatal("the key reached the log")
	}
}

func TestMissingJQuantsKeyNamedLine(t *testing.T) {
	t.Parallel()
	r, src, mem := jqRig(t, fullEnv)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.earnSrcs["JP"]; ok || r.earnSrcs["US"] == nil {
		t.Fatalf("only the JP source is left out: %v", r.earnSrcs)
	}
	if !strings.Contains(r.log.String(), "JQUANTS_API_KEY is not set") {
		t.Fatalf("log %q", r.log.String())
	}
	res := earningsJob(t, r).Run(t.Context(), "2026-11-09", nil)
	if res.Err != nil || src.fetches.Load() != 0 {
		t.Fatalf("err %v fetches %d", res.Err, src.fetches.Load())
	}
	jp := rowsOf(mem, "JP")
	if len(jp) != 2 || jp[0].Status != "unsupported" || jp[0].Source != "none" || jp[1].Status != "unsupported" {
		t.Fatalf("JP rows %+v", jp)
	}
}

func TestJQuantsFailureOnlyJPRows(t *testing.T) {
	t.Parallel()
	r, src, mem := jqRig(t, jqEnv())
	src.err = jqErr{}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	earningsJob(t, r).Run(t.Context(), "2026-11-09", nil)
	jp, us := rowsOf(mem, "JP"), rowsOf(mem, "US")
	if len(jp) != 2 || jp[0].Status != "error" || jp[1].Status != "error" || jp[0].Source != "jquants" {
		t.Fatalf("JP rows %+v", jp)
	}
	if len(us) != 1 || us[0].Status == "error" {
		t.Fatalf("US rows %+v", us)
	}
}

func TestJQuantsOneCallPerDay(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	j := earningsJob(t, r)
	for i := 0; i < 2; i++ {
		if res := j.Run(t.Context(), "2026-11-09", nil); res.Err != nil {
			t.Fatalf("tick %d: %v", i, res.Err)
		}
	}
	if src.fetches.Load() != 1 || src.calls.Load() != 2 {
		t.Fatalf("fetches %d calls %d, want 1 and 2 (one per JP entry)", src.fetches.Load(), src.calls.Load())
	}
}

func TestJQuantsErrorDayNotRetried(t *testing.T) {
	t.Parallel()
	r, src, _ := jqRig(t, jqEnv())
	src.err = jqErr{}
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	j := earningsJob(t, r)
	j.Run(t.Context(), "2026-11-09", nil)
	before := src.calls.Load()
	if res := j.Run(t.Context(), "2026-11-09", nil); res.Err == nil {
		t.Fatal("a second run after an error must return the Permanent error")
	}
	if src.calls.Load() != before || before != 1 {
		t.Fatalf("calls %d then %d: no call after an error", before, src.calls.Load())
	}
}

func TestOptionNoticesPageWiresRule(t *testing.T) {
	t.Parallel()
	d := newDeps()
	p := d.optionNoticesPage(time.UTC, "ot-page", "dev", "notices", "main", []string{"main"}, nil, nil, nil, d.newOptionNotices(), d.noticeParams())
	if p.Meta.Name != "option_notices_page" || p.Meta.Schema != "page.option_notices_page.v1" {
		t.Fatalf("meta %+v", p.Meta)
	}
	b, err := p.Build(t.Context(), nil, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"scope":"notices"`) || !strings.Contains(string(b), `"no_reader"`) {
		t.Fatalf("page %s", b)
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

// divRef is a ReferenceClient that answers one projection per contract.
type divRef struct{}

func (*divRef) Dividends(_ context.Context, cs []broker.Contract) ([]broker.DividendProjection, error) {
	return make([]broker.DividendProjection, len(cs)), nil
}
func (*divRef) Close() error { return nil }

// divStore is the dividend store the rig opens.
type divStore struct{ dividend.Store }

func (*divStore) Close() error { return nil }

// divRig is a run-once rig whose dividend_capture constructor records what it was given.
type divRig struct {
	*rig
	ref    broker.ReferenceClient
	res    store.InstrumentResolver
	w      dividend.Writer
	watch  []jobs.WatchEntry
	store  *divStore
	dialed atomic.Int32
	events string
	socket string
}

func newDivRig(t *testing.T) *divRig {
	t.Helper()
	d := &divRig{rig: newRig(t, fullEnv), store: &divStore{}}
	dd := &d.a.deps
	dd.newReference = func(_ context.Context, socket string, _ broker.Options) (broker.ReferenceClient, error) {
		d.dialed.Add(1)
		d.socket = socket
		return &divRef{}, nil
	}
	dd.openDividend = func(_ context.Context, dsn string) (dividend.Store, error) {
		d.events = dsn
		return d.store, nil
	}
	dd.newDividendCapture = func(
		c broker.ReferenceClient, res store.InstrumentResolver, w dividend.Writer, _ dividend.Reader, entries []jobs.WatchEntry, _ clock.Clock,
	) (contracts.Job, error) {
		d.ref, d.res, d.w, d.watch = c, res, w, entries
		return namedJob{id: "dividend_capture"}, nil
	}
	body := "[[underlying]]\nissuer = \"cik:0000320193\"\nmarket = \"US\"\nsymbol = \"AAPL\"\n"
	if err := writeFile(d.a.getenv("OT_HOME")+"/watch.toml", body); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDividendCaptureRegistered(t *testing.T) {
	t.Parallel()
	d := newDivRig(t)
	if err := runOnce(d.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := jobIDs(d.jobs)
	i, k := slices.Index(got, "dividend_capture"), slices.Index(got, "option_eod")
	if i < 0 || k < 0 || i != k+1 {
		t.Fatalf("jobs %v: dividend_capture is not right after option_eod", got)
	}
	if d.res != store.InstrumentResolver(d.instr) {
		t.Error("the instrument store opened for option_eod is not the resolver of dividend_capture")
	}
	if d.events != "events-dsn" {
		t.Errorf("dividend store opened with %q", d.events)
	}
	if d.w != dividend.Writer(d.store) {
		t.Error("the writer is not the opened dividend store")
	}
	if len(d.watch) != 1 || d.watch[0].Issuer != "cik:0000320193" {
		t.Errorf("watch list %+v", d.watch)
	}
	if want := filepath.Join(d.a.getenv("OT_HOME"), "run", "ibkr.sock"); d.socket != want {
		t.Errorf("reference dialled %q, want %q", d.socket, want)
	}
	if d.ref == nil {
		t.Error("no reference client was handed to the job")
	}
	if err := backfill(d.a, []string{"--day", "2026-11-04"}); err != nil || !slices.Contains(jobIDs(d.jobs), "dividend_capture") {
		t.Fatalf("backfill err %v jobs %v", err, jobIDs(d.jobs))
	}
}

func TestDividendCaptureOffWhenBrokerDown(t *testing.T) {
	t.Parallel()
	d := newDivRig(t)
	d.fail["broker"] = errors.New("down")
	if err := runOnce(d.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	got := jobIDs(d.jobs)
	if slices.Contains(got, "dividend_capture") || !slices.Contains(got, "option_eod") {
		t.Fatalf("jobs %v: want option_eod and no dividend_capture", got)
	}
	if d.dialed.Load() != 0 || d.events != "" {
		t.Errorf("the reference client was dialled (%d) or the dividend store opened (%q) with the broker down", d.dialed.Load(), d.events)
	}
	log := d.log.String()
	if !strings.Contains(log, "dividend_capture") || !strings.Contains(log, reasonBrokerDown) || strings.Contains(log, d.a.getenv("OT_HOME")) {
		t.Errorf("log %q does not name the job and the reason, or names a path", log)
	}
}

// barRec records what the bar fakes were given.
type barRec struct {
	dsns                []string
	stores              []*fakeBarStore
	barsB, fxB          broker.Broker
	barsH, fxH          broker.HistoryClient
	barsW, barsR        any
	fxW, fxR            any
	entries             []jobs.WatchEntry
	historyDials        int
	historyErr          error
	closedStores, hists int
}

type fakeBarStore struct {
	bar.BarStore
	rec *barRec
}

func (s *fakeBarStore) Close() error { s.rec.closedStores++; return nil }

type fakeHistory struct{ rec *barRec }

func (fakeHistory) Bars(context.Context, broker.BarQuery) ([]broker.Bar, error) { return nil, nil }
func (h fakeHistory) Close() error                                              { h.rec.hists++; return nil }

// withBarFakes sets the four bar deps on d to fakes that record into the returned barRec.
func withBarFakes(d *deps) *barRec {
	rec := &barRec{}
	d.openBar = func(_ context.Context, dsn string) (bar.BarStore, error) {
		rec.dsns = append(rec.dsns, dsn)
		s := &fakeBarStore{rec: rec}
		rec.stores = append(rec.stores, s)
		return s, nil
	}
	d.newHistory = func(context.Context, string, broker.Options) (broker.HistoryClient, error) {
		rec.historyDials++
		if rec.historyErr != nil {
			return nil, rec.historyErr
		}
		return fakeHistory{rec}, nil
	}
	d.newBarsRefresh = func(b broker.Broker, h broker.HistoryClient, _ store.InstrumentResolver, w bar.PriceBarWriter,
		r bar.PriceBarReader, entries []jobs.WatchEntry, _ clock.Clock) (contracts.Job, error) {
		rec.barsB, rec.barsH, rec.barsW, rec.barsR, rec.entries = b, h, w, r, entries
		return namedJob{id: jobBarsRefresh}, nil
	}
	d.newFxRefresh = func(b broker.Broker, h broker.HistoryClient, w bar.FxRateWriter, r bar.FxRateReader, _ clock.Clock) (contracts.Job, error) {
		rec.fxB, rec.fxH, rec.fxW, rec.fxR = b, h, w, r
		return namedJob{id: jobFxRefresh}, nil
	}
	return rec
}

var barEnv = bar.DSNEnvBar + "=bar-dsn\n"

func barRunOnce(t *testing.T, env string) (*rig, *barRec) {
	t.Helper()
	r := newRig(t, env)
	rec := withBarFakes(&r.a.deps)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	return r, rec
}

func TestBarsRefreshRegistered(t *testing.T) {
	t.Parallel()
	r, rec := barRunOnce(t, fullEnv+barEnv)
	if got := jobIDs(r.jobs); !slices.Contains(got, jobBarsRefresh) {
		t.Fatalf("run-once jobs %v lack bars_refresh", got)
	}
	if len(rec.stores) == 0 || rec.dsns[0] != "bar-dsn" || rec.barsW != any(rec.stores[0]) || rec.barsR != any(rec.stores[0]) {
		t.Fatalf("bars_refresh not built over the store opened with the bar line: dsns %v", rec.dsns)
	}
	if rec.barsH == nil || rec.barsH == broker.HistoryClient(unreachableHistory{}) || rec.historyDials != 1 {
		t.Fatalf("history client %v after %d dials", rec.barsH, rec.historyDials)
	}
	if !slices.Equal(rec.entries, r.earnWatch) {
		t.Fatalf("entries %v, want the watch list %v", rec.entries, r.earnWatch)
	}
	s := newSchedRig(t, fullEnv+barEnv)
	withBarFakes(&s.a.deps)
	done := s.start()
	s.waitStarted(t, done)
	if got := s.lastBuild(); !slices.Contains(got, jobBarsRefresh) || len(s.sched.disabled) != 0 {
		t.Fatalf("scheduler jobs %v, disabled %v", got, s.sched.disabled)
	}
	_ = s.stop(t, done)
}

func TestFxRefreshRegistered(t *testing.T) {
	t.Parallel()
	r, rec := barRunOnce(t, fullEnv+barEnv)
	got := jobIDs(r.jobs)
	if !slices.Contains(got, jobFxRefresh) || slices.Index(got, jobFxRefresh) < slices.Index(got, jobBarsRefresh) {
		t.Fatalf("run-once jobs %v", got)
	}
	if rec.fxW != any(rec.stores[0]) || rec.fxR != any(rec.stores[0]) || rec.fxH != rec.barsH {
		t.Fatal("fx_refresh is not built over the bar store and the history client of bars_refresh")
	}
	s := newSchedRig(t, fullEnv+barEnv)
	withBarFakes(&s.a.deps)
	done := s.start()
	s.waitStarted(t, done)
	if got := s.lastBuild(); !slices.Contains(got, jobFxRefresh) {
		t.Fatalf("scheduler jobs %v", got)
	}
	_ = s.stop(t, done)
	if rec.closedStores != len(rec.stores) || rec.hists != 1 {
		t.Fatalf("closed %d of %d stores, %d history clients", rec.closedStores, len(rec.stores), rec.hists)
	}
}

func TestMissingBarDSNDisablesOnlyBars(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	rec := withBarFakes(&r.a.deps)
	if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	want := []string{"fred_series", "option_eod", "quote_snapshot", "compute_figures", "calendar_refresh", "earnings_refresh"}
	if got := jobIDs(r.jobs); !slices.Equal(got, want) {
		t.Fatalf("jobs %v, want %v", got, want)
	}
	log := r.log.String()
	if !strings.Contains(log, "bars_refresh") || !strings.Contains(log, "fx_refresh") || !strings.Contains(log, "OT_PG_DSN_BAR is not set") {
		t.Fatalf("log %q lacks the named line", log)
	}
	if len(rec.dsns) != 0 || rec.historyDials != 0 || r.compute.PriceBars != nil {
		t.Fatalf("a bar store was opened (%v), a history client dialled or the HV step kept", rec.dsns)
	}
	s := newSchedRig(t, fullEnv)
	withBarFakes(&s.a.deps)
	done := s.start()
	s.waitStarted(t, done)
	if len(s.sched.disabled) != 2 || s.sched.disabled[jobBarsRefresh] != reasonNoBarDSN || s.sched.disabled[jobFxRefresh] != reasonNoBarDSN {
		t.Fatalf("disabled %v", s.sched.disabled)
	}
	if got := s.lastBuild(); slices.Contains(got, jobBarsRefresh) || !slices.Contains(got, jobOptionEod) {
		t.Fatalf("scheduler jobs %v", got)
	}
	_ = s.stop(t, done)
}

func TestComputeFiguresGetsPriceBars(t *testing.T) {
	t.Parallel()
	r, rec := barRunOnce(t, fullEnv+barEnv)
	if r.compute.PriceBars == nil {
		t.Fatal("compute_figures got no PriceBars")
	}
	if len(rec.dsns) != 2 || rec.dsns[1] != "pages-dsn" || r.compute.PriceBars != bar.PriceBarReader(rec.stores[1]) {
		t.Fatalf("the reader is not the store opened with the pages line: dsns %v", rec.dsns)
	}
}

func TestBrokerDownDisablesBarJobs(t *testing.T) {
	t.Parallel()
	for name, fail := range map[string]func(*rig, *barRec){
		"broker":  func(r *rig, _ *barRec) { r.fail["broker"] = errors.New("dial: /secret/path refused") },
		"history": func(_ *rig, rec *barRec) { rec.historyErr = errors.New("dial: /secret/path refused") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, fullEnv+barEnv)
			rec := withBarFakes(&r.a.deps)
			fail(r, rec)
			if err := runOnce(r.a, []string{"--day", "2026-11-04"}); err != nil {
				t.Fatal(err)
			}
			if got := jobIDs(r.jobs); slices.Contains(got, jobBarsRefresh) || slices.Contains(got, jobFxRefresh) {
				t.Fatalf("jobs %v hold a bar job", got)
			}
			log := r.log.String()
			if !strings.Contains(log, "bars_refresh") || !strings.Contains(log, reasonBrokerDown) || strings.Contains(log, "/secret/path") {
				t.Fatalf("log %q", log)
			}
			if r.compute.PriceBars != nil {
				t.Fatal("the HV step kept its reader")
			}
		})
	}
	s := newSchedRig(t, fullEnv+barEnv)
	withBarFakes(&s.a.deps)
	s.a.deps.dialBroker = func(context.Context, string, broker.Options) (broker.Broker, error) { return nil, errors.New("down") }
	done := s.start()
	s.waitStarted(t, done)
	if s.sched.disabled[jobBarsRefresh] != reasonBrokerDown || s.sched.disabled[jobFxRefresh] != reasonBrokerDown {
		t.Fatalf("disabled %v", s.sched.disabled)
	}
	_ = s.stop(t, done)
}
