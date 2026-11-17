package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/data_health_page"
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

func TestEveryJobHasRowOrReason(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	if len(jobTable) != 33 || len(jobs) != len(jobTable) {
		t.Fatalf("%d jobs for %d rows", len(jobs), len(jobTable))
	}
	for _, r := range jobTable {
		j := dataHealthJobOf(t, jobs, r.ID)
		reason, noLedger := dataHealthNoLedger[r.ID]
		switch {
		case len(j.Datasets) == 0 && !noLedger:
			t.Errorf("%s stores no dataset and is not in the fixed list", r.ID)
		case noLedger && (reason == "" || len(j.Datasets) > 0):
			t.Errorf("%s: reason %q, datasets %v", r.ID, reason, j.Datasets)
		}
		if j.Kind != r.Kind || !dataHealthZoneCarried(j.Zone, r) || j.WhyText != r.Text {
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

func TestDueAtFromSlot(t *testing.T) {
	t.Parallel()
	jobs := dataHealthJobs()
	for id, want := range map[string]string{"option_eod": "11:00", "option_eod_index": "11:00", "option_eod_jp": "10:30", "fx_refresh": ""} {
		if got := dataHealthJobOf(t, jobs, id).DueAt; got != want {
			t.Errorf("%s DueAt %q, want %q", id, got, want)
		}
	}
	for _, j := range jobs {
		if j.DueAt != "" && !slices.Contains([]string{"option_eod", "option_eod_index", "option_eod_jp", "fred_series", "fundamentals", "macro_events"}, j.ID) {
			t.Errorf("%s carries DueAt %q", j.ID, j.DueAt)
		}
	}
	if dueAtOfSlot("") != "" || dueAtOfSlot("9999") != "" {
		t.Error("a bad slot gives a due time")
	}
}

func TestDataHealthTimeSlots(t *testing.T) {
	t.Parallel()
	want := map[string]string{"fred_series": "07:30", "fundamentals": "08:00", "macro_events": "06:00", "option_eod": "11:00"}
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
	if !strings.Contains(body, `"due_text":"due`) {
		t.Fatalf("no due text: %s", body)
	}
	if strings.Contains(body, `"repair":"button"`) {
		t.Fatalf("button before the slot: %s", body)
	}
}
