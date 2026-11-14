package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type jobConsoleVM struct {
	Form *struct {
		PostPath string `json:"post_path"`
		Expires  string `json:"expires"`
		MAC      string `json:"mac"`
	} `json:"form"`
	Gaps []struct {
		Reason string `json:"reason"`
	} `json:"gaps"`
	Jobs []struct {
		ID        string  `json:"id"`
		Kind      string  `json:"kind"`
		Today     string  `json:"today"`
		Note      *string `json:"note"`
		Force     bool    `json:"force"`
		ForceWhy  string  `json:"force_why"`
		WhyText   string  `json:"why_text"`
		NeedsText string  `json:"needs_text"`
	} `json:"jobs"`
}

func decodeJobConsole(t *testing.T, rec *httptest.ResponseRecorder) jobConsoleVM {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %.200q", rec.Code, rec.Body)
	}
	var vm jobConsoleVM
	if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
		t.Fatalf("%v %.300q", err, rec.Body)
	}
	return vm
}

func TestRouteOffGivesNullForm(t *testing.T) {
	t.Parallel()
	h, _, _ := jobConsoleSite(t, false, postNow)
	vm := decodeJobConsole(t, settingsGet(t, h, http.MethodGet, "/job-console"))
	if vm.Form != nil {
		t.Fatalf("form %+v", vm.Form)
	}
	h, _, _ = jobConsoleSite(t, true, postNow)
	vm = decodeJobConsole(t, settingsGet(t, h, http.MethodGet, "/job-console"))
	if vm.Form == nil || vm.Form.PostPath != "/owner-job-console" || vm.Form.Expires != "0" || vm.Form.MAC != strings.Repeat("0", 64) {
		t.Fatalf("form %+v", vm.Form)
	}
}

func TestSiteBuildsPageFromTable(t *testing.T) {
	t.Parallel()
	h, _, _ := jobConsoleSite(t, true, postNow)
	vm := decodeJobConsole(t, settingsGet(t, h, http.MethodGet, "/job-console"))
	if len(vm.Jobs) != len(jobTable) {
		t.Fatalf("%d jobs, table has %d", len(vm.Jobs), len(jobTable))
	}
	by := map[string]int{}
	for i, j := range vm.Jobs {
		by[j.ID] = i
	}
	fred := vm.Jobs[by["fred_series"]]
	if fred.Kind != "any_day" || !fred.Force || fred.ForceWhy != "" || fred.Today != "2026-11-06" {
		t.Errorf("fred_series %+v", fred)
	}
	if cf := vm.Jobs[by["compute_figures"]]; cf.Kind != "any_day" || !cf.Force || cf.ForceWhy != "" {
		t.Errorf("compute_figures %+v", cf)
	}
	if cal := vm.Jobs[by["calendar_refresh"]]; cal.Kind != "today_only" || !cal.Force {
		t.Errorf("calendar_refresh %+v", cal)
	}
	open := vm.Jobs[by["option_open"]]
	if open.Kind != "listed_only" || open.WhyText != textNoLowerBound || open.NeedsText != needsLowerBound {
		t.Errorf("option_open %+v", open)
	}
	if q := vm.Jobs[by["quote_snapshot"]]; q.Note == nil || *q.Note != noteQuoteSnapshot {
		t.Errorf("quote_snapshot %+v", q)
	}
	// the Tokyo job reads today in its own zone: 12:00 UTC is 21:00 on the same date.
	if jp := vm.Jobs[by["option_eod_jp"]]; jp.Today != "2026-11-06" {
		t.Errorf("option_eod_jp %+v", jp)
	}
}

func TestSiteOffersForceOnEventAndFundamentalRows(t *testing.T) {
	t.Parallel()
	h, _, _ := jobConsoleSite(t, false, postNow)
	vm := decodeJobConsole(t, settingsGet(t, h, http.MethodGet, "/job-console"))
	by := map[string]int{}
	for i, j := range vm.Jobs {
		by[j.ID] = i
	}
	for _, id := range []string{"earnings_refresh", "dividend_capture", "fundamentals"} {
		idx, ok := by[id]
		if !ok {
			t.Errorf("%s is not on the page", id)
			continue
		}
		if j := vm.Jobs[idx]; j.Kind != "any_day" || !j.Force || j.ForceWhy != "" {
			t.Errorf("%s %+v", id, j)
		}
	}
}

func TestReaderErrorGivesUnavailableGap(t *testing.T) {
	t.Parallel()
	h, fp, _ := jobConsoleSite(t, false, postNow)
	fp.getErr = errors.New("permission denied for table job_request")
	vm := decodeJobConsole(t, settingsGet(t, h, http.MethodGet, "/job-console"))
	if len(vm.Gaps) != 1 || vm.Gaps[0].Reason != "job_requests_unavailable" || len(vm.Jobs) != len(jobTable) {
		t.Fatalf("%+v", vm)
	}
	// no reader at all gives the same gap.
	a, _ := testApp(t, "")
	h = siteHandler(a, fakeClock{postNow}, "", siteReaders{}, echoHandler, http.NotFoundHandler())
	if vm = decodeJobConsole(t, settingsGet(t, h, http.MethodGet, "/job-console")); len(vm.Gaps) != 1 || vm.Gaps[0].Reason != "job_requests_unavailable" {
		t.Fatalf("%+v", vm)
	}
}
