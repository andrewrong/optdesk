package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/optionresult"
)

func homeFixture(t *testing.T, demo bool) http.Handler {
	t.Helper()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{}
	if demo {
		a.profile.Name = "demo"
		a.deps.newOptionLedger, a.deps.newOptionResults = optionledger.New, optionresult.New
		rd, _, _ = openSiteReaders(context.Background(), a)
	}
	return siteHandler(a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
}

func TestHomeRouteServesFiveMembers(t *testing.T) {
	t.Parallel()
	for name, demo := range map[string]bool{"no reader": false, "demo": true} {
		h := homeFixture(t, demo)
		rec := companiesGet(t, h, http.MethodGet, "/"+siteHomeRoute)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %q", name, rec.Code, rec.Body.String())
		}
		var p map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var schema string
		_ = json.Unmarshal(p["schema"], &schema)
		if schema != "home_page" {
			t.Fatalf("%s: schema %q", name, schema)
		}
		for _, m := range []string{"household", "companies", "macro", "market", "options"} {
			if len(p[m]) == 0 {
				t.Errorf("%s: member %s missing", name, m)
			}
		}
		if string(p["gap_line"]) == `""` || len(p["gap_line"]) == 0 {
			t.Errorf("%s: no gap_line", name)
		}
	}
}

func TestHomeRouteNoReaderMembersAreGaps(t *testing.T) {
	t.Parallel()
	rec := companiesGet(t, homeFixture(t, false), http.MethodGet, "/home")
	var p struct {
		Overview struct {
			Gaps json.RawMessage `json:"gaps"`
		} `json:"overview"`
		GapLine string `json:"gap_line"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(p.Overview.Gaps), "[") {
		t.Fatalf("overview.gaps is not a JSON array: %q", p.Overview.Gaps)
	}
	if p.GapLine == "" || p.GapLine == "No gaps today" {
		t.Fatalf("gap_line %q: five members without a reader are gaps", p.GapLine)
	}
}

func TestRootRedirectsToHome(t *testing.T) {
	t.Parallel()
	rec := companiesGet(t, homeFixture(t, false), http.MethodGet, "/")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/home" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestHomePageNamed(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	p, closeFn, err := homePageNamed(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if p.Meta.Name != homeName || p.Meta.Schema != "page.home_page.v3" {
		t.Fatalf("meta %+v", p.Meta)
	}
}

func TestHomeCompaniesRowsFromStore(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	rd := siteReaders{underlying: companiesStore()}
	body, err := buildHome(t.Context(), a, fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"AAA"`) || !strings.Contains(string(body), `"BBB"`) {
		t.Fatalf("stored names missing: %s", body)
	}
}
