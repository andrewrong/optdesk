package main

import (
	"context"
	"maps"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/clock"
	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPageRegistryRoutes(t *testing.T) {
	t.Parallel()
	want := []struct {
		route, name string
		order       int
	}{
		{"options/candidates", "option_candidates_page", 10},
		{"options/notices", "option_notices_page", 20},
		{"options/tracking", "option_tracking_page", 30},
		{"options", "option_page", 40},
		{"companies", "companies_page", 50},
		{"settings", "", 0},
	}
	for _, w := range want {
		f, ok := pageFamilyOfRoute(w.route)
		if !ok {
			t.Fatalf("route %q is not registered", w.route)
		}
		if f.name != w.name || f.order != w.order || f.build == nil {
			t.Errorf("route %q: name %q order %d build nil=%v", w.route, f.name, f.order, f.build == nil)
		}
		if (f.named == nil) != (w.name == "") {
			t.Errorf("route %q: named nil=%v with name %q", w.route, f.named == nil, w.name)
		}
	}
	if _, ok := pageFamilyOfRoute("options/nosuch"); ok {
		t.Error("an unregistered route is found")
	}
	got := registeredPageNames()
	wantNames := "option_candidates_page,option_notices_page,option_tracking_page,option_page,companies_page,household_page,advice_page," +
		"household_risk"
	if strings.Join(got, ",") != wantNames {
		t.Errorf("registered names %v", got)
	}
}

func TestPageDupPanics(t *testing.T) {
	t.Parallel()
	build := func(context.Context, *app, clock.Clock, string, siteReaders) ([]byte, error) { return nil, nil }
	cases := map[string]pageFamily{
		"route": {route: "settings", build: build},
		"name":  {route: "x/one", name: "option_notices_page", order: 99, build: build},
		"order": {route: "x/two", name: "x_page", order: 10, build: build},
	}
	for what, f := range cases {
		reg := maps.Clone(pageFamilies)
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("a clash of %s does not panic", what)
				}
			}()
			addPage(reg, f)
		}()
	}
	reg := maps.Clone(pageFamilies)
	addPage(reg, pageFamily{route: "x/new", name: "x_new_page", order: 99, build: build})
	if _, ok := reg["x/new"]; !ok {
		t.Error("a page with a new route, name and order is not added")
	}
	if _, ok := pageFamilies["x/new"]; ok {
		t.Error("the global registry was changed")
	}
}

func TestUnknownPageText(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	a.deps.pageList = []pages.Page{fakePage("alpha", "a")}
	_, _, err := pageNamed(a, "nosuch")
	want := `unknown page "nosuch"; known pages: advice_page, alpha, companies_page, household_page, household_risk, macro_page, market_page, ` +
		`option_candidates_page, option_notices_page, option_page, option_tracking_page, report_page`
	if err == nil || err.Error() != want {
		t.Fatalf("err %v, want %s", err, want)
	}
	if _, _, err := pageNamed(a, "settings_page"); err == nil || err.Error() == "" {
		t.Fatalf("settings_page must stay unknown, err %v", err)
	}
}

func TestSiteReaderOf(t *testing.T) {
	t.Parallel()
	if v, ok := siteReaderOf[string](siteReaders{}, "k"); ok || v != "" {
		t.Errorf("nil ext gave %q %v", v, ok)
	}
	rd := siteReaders{ext: map[string]any{"k": "v", "n": 3}}
	if v, ok := siteReaderOf[string](rd, "k"); !ok || v != "v" {
		t.Errorf("got %q %v", v, ok)
	}
	if _, ok := siteReaderOf[string](rd, "n"); ok {
		t.Error("a reader of another type is returned")
	}
}

func TestPageRouteClashPanics(t *testing.T) {
	t.Parallel()
	b := func(context.Context, *app, clock.Clock, string, siteReaders) ([]byte, error) { return nil, nil }
	defer func() {
		if recover() == nil {
			t.Fatal("route clash with a non-nil build does not panic")
		}
	}()
	registerPage(pageFamily{route: "settings", build: b})
}
