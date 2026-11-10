package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestRiskRouteServesFromReaders(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	h := siteHandler(a, householdClock, "2026-11-05", householdReaders(new([]string)), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteRiskRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, riskName) || !strings.Contains(body, `"scope":"main"`) {
		t.Fatalf("%d %q", rec.Code, body)
	}
}

func TestRiskRouteWithoutReaderIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteRiskRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"no_reader"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
}

func TestRiskPageNamedDemoIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	page, closeFn, err := pageNamed(a, riskName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
	if err != nil || !strings.Contains(string(got), `"no_reader"`) {
		t.Fatalf("%v %s", err, got)
	}
}
