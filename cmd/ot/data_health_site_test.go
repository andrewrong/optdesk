package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/broker"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
	"github.com/andrewrong/optdesk/internal/pages/job_console"
	"github.com/andrewrong/optdesk/internal/store/coverage"
)

// fakeCoverage answers fixed cells and sessions; a non-nil err fails Cells.
type fakeCoverage struct {
	cells []coverage.Cell
	err   error
}

func (f fakeCoverage) Cells(context.Context, contracts.Day, contracts.Day) ([]coverage.Cell, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]coverage.Cell{}, f.cells...), nil
}

func (fakeCoverage) Sessions(_ context.Context, cal string, _, to contracts.Day) ([]coverage.Session, error) {
	return []coverage.Session{{Calendar: cal, Day: to, IsSession: true}}, nil
}

func (fakeCoverage) NewestObservations(context.Context) ([]coverage.Newest, error) {
	return []coverage.Newest{}, nil
}

func (fakeCoverage) Close() error { return nil }

func dataHealthDay() time.Time { return time.Date(2026, 11, 13, 20, 0, 0, 0, time.UTC) }

const dataHealthSchemaHost = "https://optdesk.invalid/schemas/"

// schemaFileLoader serves the schema files of contracts/schemas for the invalid host the schemas name.
type schemaFileLoader struct{}

func (schemaFileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, dataHealthSchemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(filepath.Join("../../contracts/schemas", file))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validateDataHealth(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(schemaFileLoader{})
	sch, err := c.Compile(dataHealthSchemaHost + "page.data_health_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
}

func dataHealthGet(t *testing.T, a *app, rd siteReaders) (int, []byte) {
	t.Helper()
	h := siteHandler(a, fakeClock{dataHealthDay()}, "", rd, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, http.MethodGet, "/"+siteDataHealthRoute)
	return rec.Code, rec.Body.Bytes()
}

func TestDataHealthRoute(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	cov := fakeCoverage{cells: []coverage.Cell{{Day: "2026-11-13", Dataset: "obs.fx_rate", Keys: 2, OK: 2}}}
	var dsn string
	a.deps.ext = map[string]any{extDataHealth: dataHealthDeps{openCoverage: func(_ context.Context, d string) (coverage.Reader, error) {
		dsn = d
		return cov, nil
	}}}
	v, ok := siteOpeners[siteDataHealthKey](t.Context(), a, func(io.Closer, error) bool { return true })
	if !ok || v == nil || dsn != "postgres://pages" {
		t.Fatalf("opener: %v %v dsn %q", v, ok, dsn)
	}
	code, body := dataHealthGet(t, a, siteReaders{ext: map[string]any{siteDataHealthKey: v}})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	validateDataHealth(t, body)
	if !strings.Contains(string(body), `"dataset_id":"obs.fx_rate"`) || !strings.Contains(string(body), `"job_id":"fx_refresh"`) {
		t.Fatalf("body %s", body)
	}
	if !strings.Contains(string(body), `"form":null`) {
		t.Fatalf("a site without the owner edit carries a form: %s", body)
	}

	// with the owner edit the form posts to the job console and a missing day of an any-day job has a button.
	cov.cells[0].OK, cov.cells[0].Missing = 1, 1
	rd := siteReaders{ext: map[string]any{siteDataHealthKey: coverage.Reader(cov)}, edit: &ownerEdit{}}
	_, body = dataHealthGet(t, a, rd)
	validateDataHealth(t, body)
	if !strings.Contains(string(body), `"post_path":"/`+jobConsolePostRoute+`"`) || !strings.Contains(string(body), `"repair":"button"`) {
		t.Fatalf("owner edit body %s", body)
	}
}

func TestDataHealthRouteNoPagesDSN(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNRELATED=1\n")
	a.deps.ext = map[string]any{extDataHealth: dataHealthDeps{openCoverage: func(context.Context, string) (coverage.Reader, error) {
		return nil, errors.New("must not open")
	}}}
	v, ok := siteOpeners[siteDataHealthKey](t.Context(), a, func(io.Closer, error) bool { return true })
	if !ok || v != nil {
		t.Fatalf("opener without the line: %v %v", v, ok)
	}
	code, body := dataHealthGet(t, a, siteReaders{})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	validateDataHealth(t, body)
	if !strings.Contains(string(body), `"reason":"coverage_unavailable"`) {
		t.Fatalf("body %s", body)
	}
}

func TestPageListHasDataHealth(t *testing.T) {
	t.Parallel()
	names := registeredPageNames()
	if !slices.Contains(names, dataHealthName) {
		t.Fatalf("registered names %v", names)
	}
	f, ok := pageFamilyOfRoute(siteDataHealthRoute)
	if !ok || f.name != "data_health_page" || f.order != 130 || f.named == nil {
		t.Fatalf("family %+v", f)
	}
	a, _ := testApp(t, "OT_UNRELATED=1\n")
	p, closeFn, err := pageNamed(a, "data_health_page")
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if p.Meta.Name != "data_health_page" || p.Meta.Schema != "page.data_health_page.v1" {
		t.Fatalf("meta %+v", p.Meta)
	}
}

func dataHealthJobOf(t *testing.T, jobs []data_health_page.Job, id string) data_health_page.Job {
	t.Helper()
	for _, j := range jobs {
		if j.ID == id {
			return j
		}
	}
	t.Fatalf("job %s is not carried", id)
	return data_health_page.Job{}
}

// dataHealthZoneCarried: the zone of the row, or the zone of the time table when the row has none.
func dataHealthZoneCarried(zone string, r jobRow) bool {
	return zone == r.Zone || (r.Zone == "" && zone == dataHealthTimeSlots[r.ID].zone)
}

// pageKindOf is the kind the page gives the job of a jobTable row.
func pageKindOf(r jobRow) string {
	if slices.Contains(dataHealthAnyDayJobs, r.ID) {
		return data_health_page.KindAnyDay
	}
	return r.Kind
}

func TestEveryJobHasRowOrReason(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	if len(jobTable) != 35 || len(jobs) != len(jobTable) {
		t.Fatalf("%d jobs for %d rows", len(jobs), len(jobTable))
	}
	for _, r := range jobTable {
		j := dataHealthJobOf(t, jobs, r.ID)
		wantKind := pageKindOf(r)
		reason, noLedger := dataHealthNoLedger[r.ID]
		switch {
		case len(j.Datasets) == 0 && !noLedger:
			t.Errorf("%s stores no dataset and is not in the fixed list", r.ID)
		case noLedger && (reason == "" || len(j.Datasets) > 0):
			t.Errorf("%s: reason %q, datasets %v", r.ID, reason, j.Datasets)
		}
		if j.Kind != wantKind || !dataHealthZoneCarried(j.Zone, r) || j.WhyText != r.Text {
			t.Errorf("%s: row not carried: %+v", r.ID, j)
		}
	}
	for _, id := range []string{"account_capture", "statement_capture", "position_marks", "quote_snapshot"} {
		if len(dataHealthJobOf(t, jobs, id).Datasets) == 0 && dataHealthNoLedger[id] == "" {
			t.Errorf("account job %s has neither dataset nor reason", id)
		}
	}
	for id := range dataHealthNoLedger {
		if _, ok := jobRowOf(id); !ok {
			t.Errorf("fixed list names %s, which is not in jobTable", id)
		}
	}
	for _, id := range []string{"earnings_refresh", "dividend_capture"} {
		if !dataHealthJobOf(t, jobs, id).PresentOnly {
			t.Errorf("%s is not present-only", id)
		}
	}
	if dataHealthJobOf(t, jobs, "fx_refresh").PresentOnly {
		t.Error("fx_refresh is present-only")
	}
}

func TestDatasetsResolveToFirstJob(t *testing.T) {
	t.Parallel()
	r := newRig(t, fullEnv)
	if err := runWired(r.a, []string{"--day", "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	jobs := dataHealthJobs()
	checked := 0
	for _, j := range r.jobs {
		keys, _, err := j.Plan(t.Context(), "2026-11-04")
		if err != nil {
			continue
		}
		checked++
		for _, k := range keys {
			if !slices.ContainsFunc(jobs, func(d data_health_page.Job) bool { return slices.Contains(d.Datasets, k.DatasetID) }) {
				t.Errorf("job %s plans dataset %s that no job lists", j.ID(), k.DatasetID)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no plan was built")
	}
	var listing []string
	for _, d := range jobs {
		if slices.Contains(d.Datasets, "master.option_list") {
			listing = append(listing, d.ID)
		}
	}
	if !slices.Equal(listing, []string{"option_list_preopen", "option_list_zone"}) {
		t.Fatalf("master.option_list is listed by %v", listing)
	}
	// the dataset row takes the first job in jobTable order (Decision 17).
	cov := fakeCoverage{cells: []coverage.Cell{{Day: "2026-11-13", Dataset: "master.option_list", Keys: 1, Missing: 1}}}
	b, err := data_health_page.Build(t.Context(), data_health_page.Input{Now: dataHealthDay(), Today: "2026-11-13", Cov: cov, Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"job_id":"option_list_preopen"`) {
		t.Fatalf("dataset row: %s", b)
	}
}

// dueTextRE matches any non-empty due_text value, whatever language it starts in.
var dueTextRE = regexp.MustCompile(`"due_text":\s*"[^"]`)

func TestDueAtFromSlot(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	for id, want := range map[string]string{"option_eod": "14:00", "option_eod_index": "15:45", "option_eod_jp": "15:15", "fx_refresh": "",
		"option_open_resolve": "17:00", "option_open_resolve_jp": "16:30", "option_open_resolve_gap": "08:00",
		"option_open": "09:35", "option_open_jp": "09:05", "option_open_plan": "08:45", "option_open_plan_jp": "08:15", "option_ledger": "05:00"} {
		if got := dataHealthJobOf(t, jobs, id).DueAt; got != want {
			t.Errorf("%s DueAt %q, want %q", id, got, want)
		}
	}
	for _, j := range jobs {
		timed := []string{"option_eod", "option_eod_index", "option_eod_jp", "fred_series", "fundamentals", "macro_events",
			"bars_refresh", "iv_history", "account_capture", "position_marks",
			"option_open_resolve", "option_open_resolve_jp", "option_open_resolve_gap",
			"option_open", "option_open_jp", "option_open_plan", "option_open_plan_jp", "option_ledger"}
		if j.DueAt != "" && !slices.Contains(timed, j.ID) {
			t.Errorf("%s carries DueAt %q", j.ID, j.DueAt)
		}
	}
	if dueAtOfSlot("") != "" || dueAtOfSlot("9999") != "" {
		t.Error("a bad slot gives a due time")
	}
}

func TestDataHealthTimeSlots(t *testing.T) {
	t.Parallel()
	want := map[string]string{"fred_series": "07:30", "fundamentals": "08:00", "macro_events": "06:00", "option_eod": "14:00"}
	got := map[string]data_health_page.Job{}
	for _, j := range dataHealthJobs() {
		got[j.ID] = j
	}
	for id, due := range want {
		if got[id].DueAt != due {
			t.Errorf("%s DueAt %q, want %q", id, got[id].DueAt, due)
		}
		if id != "option_eod" && got[id].Zone != "America/New_York" {
			t.Errorf("%s Zone %q", id, got[id].Zone)
		}
	}

	// 06:00 New York on a session day, before the slot of fred_series: the missing cell shows its due time.
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	cov := fakeCoverage{cells: []coverage.Cell{{Day: "2026-11-13", Dataset: "obs.series_point", Keys: 1, Missing: 1}}}
	rd := siteReaders{ext: map[string]any{siteDataHealthKey: coverage.Reader(cov)}, edit: &ownerEdit{}}
	early := time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC)
	h := siteHandler(a, fakeClock{early}, "", rd, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, http.MethodGet, "/"+siteDataHealthRoute)
	body := rec.Body.String()
	validateDataHealth(t, rec.Body.Bytes())
	if !dueTextRE.MatchString(body) {
		t.Fatalf("no due text: %s", body)
	}
	if strings.Contains(body, `"repair":"button"`) {
		t.Fatalf("button before the slot: %s", body)
	}
}

func TestListedOnlyResolveCellIsDueBeforeSlot(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	cov := fakeCoverage{cells: []coverage.Cell{{Day: "2026-11-13", Dataset: "master.option_contract", Keys: 1, Missing: 1}}}
	rd := siteReaders{ext: map[string]any{siteDataHealthKey: coverage.Reader(cov)}, edit: &ownerEdit{}}
	get := func(now time.Time) string {
		h := siteHandler(a, fakeClock{now}, "", rd, echoHandler, http.NotFoundHandler())
		rec := settingsGet(t, h, http.MethodGet, "/"+siteDataHealthRoute)
		validateDataHealth(t, rec.Body.Bytes())
		return rec.Body.String()
	}
	early := get(time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC))
	if !dueTextRE.MatchString(early) {
		t.Fatalf("no due text before the slot: %s", early)
	}
	if regexp.MustCompile(`"why_text":"[^"]`).MatchString(early) || !strings.Contains(early, `"repair":"none"`) {
		t.Fatalf("why_text or repair other than none before the slot: %s", early)
	}
	if strings.Contains(early, `"repair":"button"`) || strings.Contains(early, job_console.ZhWhyHeavyAndCap) {
		t.Fatalf("reason or button before the slot: %s", early)
	}
	late := get(time.Date(2026, 11, 13, 23, 0, 0, 0, time.UTC))
	if !strings.Contains(late, `"repair":"not_refetchable"`) {
		t.Fatalf("late repair: %s", late)
	}
	if !strings.Contains(late, job_console.ZhWhyHeavyAndCap) {
		t.Fatalf("no always-cut reason after the slot: %s", late)
	}
}

func TestDataHealthGateways(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, gatewayEnv)
	main := &fakeHealth{h: broker.Health{State: "farm_broken", Farms: []broker.Farm{{Name: "usfarm", Up: true}, {Name: "ushmds"}}}}
	wife := &fakeHealth{err: errors.New("no answer")}
	a.deps.ext = map[string]any{extGateway: gatewayDeps{newHealth: fakeDial(map[string]*fakeHealth{"/s/m": main, "/s/w": wife})}}
	code, body := dataHealthGet(t, a, siteReaders{})
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	validateDataHealth(t, body)
	fx, err := os.ReadFile("../../contracts/fixtures/page.data_health_page.v1.gateways.json")
	if err != nil {
		t.Fatal(err)
	}
	// The note is the builder's text, written by the builder's own tests; the wire owns name, verdict and farms.
	var got, want struct {
		Gateways []map[string]any `json:"gateways"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fx, &want); err != nil {
		t.Fatal(err)
	}
	for _, g := range append(got.Gateways, want.Gateways...) {
		delete(g, "note")
	}
	if len(want.Gateways) != 2 || !reflect.DeepEqual(got.Gateways, want.Gateways) {
		t.Fatalf("gateways %v, want the fixture's %v", got.Gateways, want.Gateways)
	}
}

func TestDataHealthJobsCloseSlots(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	for id, due := range map[string]string{
		"bars_refresh": "16:10", "iv_history": "16:10", "account_capture": "16:30", "position_marks": "16:40",
	} {
		j := dataHealthJobOf(t, jobs, id)
		if j.DueAt != due || j.Zone != "America/New_York" {
			t.Errorf("%s: DueAt %q zone %q, want %q America/New_York", id, j.DueAt, j.Zone, due)
		}
	}
}

func TestDataHealthOptionEodSlots(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	for id, want := range map[string]struct{ dueAt, zone string }{
		"option_eod":       {"14:00", "America/New_York"},
		"option_eod_index": {"15:45", "America/New_York"},
		"option_eod_jp":    {"15:15", "Asia/Tokyo"},

		"option_open_resolve":     {"17:00", "America/New_York"},
		"option_open_resolve_jp":  {"16:30", "Asia/Tokyo"},
		"option_open_resolve_gap": {"08:00", "America/New_York"},

		"option_open":         {"09:35", "America/New_York"},
		"option_open_jp":      {"09:05", "Asia/Tokyo"},
		"option_open_plan":    {"08:45", "America/New_York"},
		"option_open_plan_jp": {"08:15", "Asia/Tokyo"},
		"option_ledger":       {"05:00", "America/New_York"},
	} {
		j := dataHealthJobOf(t, jobs, id)
		if j.DueAt != want.dueAt || j.Zone != want.zone {
			t.Errorf("%s DueAt %q zone %q, want %q in %q", id, j.DueAt, j.Zone, want.dueAt, want.zone)
		}
	}
}

func TestDataHealthOpenSlots(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	for _, id := range []string{"option_open", "option_open_jp", "option_open_plan", "option_open_plan_jp", "option_ledger"} {
		j := dataHealthJobOf(t, jobs, id)
		if j.DueAt == "" || j.Zone == "" {
			t.Errorf("%s: DueAt %q zone %q, want both set", id, j.DueAt, j.Zone)
		}
	}
}

func TestDataHealthJobsIvHistoryAnyDay(t *testing.T) {
	t.Parallel()
	var job *data_health_page.Job
	for _, j := range dataHealthJobs() {
		if j.ID == "iv_history" {
			job = &j
		}
	}
	if job == nil {
		t.Fatal("no iv_history job")
	}
	if job.Kind != data_health_page.KindAnyDay || job.PresentOnly {
		t.Fatalf("kind %q presentOnly %v", job.Kind, job.PresentOnly)
	}
	cells := []data_health_page.Cell{{Day: "2026-11-02", State: "missing"}}
	form := &data_health_page.Form{PostPath: "/x", Expires: "1", MAC: "m"}
	data_health_page.Apply(cells, job, form, time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC), "2026-11-06")
	if cells[0].Repair != data_health_page.RepairButton {
		t.Fatalf("repair %q, want button", cells[0].Repair)
	}
}
