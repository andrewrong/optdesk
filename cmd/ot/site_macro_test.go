package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/pages"
)

func TestMacroRouteServesMacroPage(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Zone = time.UTC
	a.deps.macroPage = newMacroPage
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	h := siteHandler(a, clk, "2026-11-06", siteReaders{series: noSeries{}}, echoHandler, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/macro", nil))
	if rec.Code != 200 {
		t.Fatalf("code %d %q", rec.Code, rec.Body)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if string(body["schema"]) != `"macro_page"` {
		t.Fatalf("schema %s", body["schema"])
	}
	if _, ok := body["lead"]; !ok {
		t.Fatalf("no lead block: %.300q", rec.Body)
	}
}

func TestMacroRouteWithEventsUsesOwnerPage(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.profile.Zone = time.UTC
	a.deps.macroPage = func(*time.Location, string, compute.ReadingFunc) pages.Page {
		t.Error("the page without events was used")
		return pages.Page{}
	}
	clk := fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}
	rd := siteReaders{series: noSeries{}, macroEvents: fakeHomeEvents{}}
	h := siteHandler(a, clk, "2026-11-06", rd, echoHandler, http.NotFoundHandler())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/macro", nil))
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"events"`) || !strings.Contains(rec.Body.String(), `"schema":"macro_page"`) {
		t.Fatalf("%d %.300q", rec.Code, rec.Body)
	}
}
