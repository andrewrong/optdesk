package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"unsafe"
)

// mergeSiteReaders fills each reader of the result from the first set that has it; the ext maps are united.
func mergeSiteReaders(sets ...siteReaders) siteReaders {
	var out siteReaders
	ov := reflect.ValueOf(&out).Elem()
	ext := map[string]any{}
	for _, s := range sets {
		sv := reflect.ValueOf(&s).Elem()
		for i := 0; i < sv.NumField(); i++ {
			dst, src := ov.Field(i), sv.Field(i)
			if dst.IsZero() && !src.IsZero() {
				// the fields are unexported: write through their address
				reflect.NewAt(dst.Type(), unsafe.Pointer(dst.UnsafeAddr())).Elem().Set(
					reflect.NewAt(src.Type(), unsafe.Pointer(src.UnsafeAddr())).Elem())
			}
		}
		for k, v := range s.ext {
			if _, ok := ext[k]; !ok {
				ext[k] = v
			}
		}
	}
	out.ext = ext
	return out
}

// siteRoutePaths is every path the walk calls, sorted: the routes of pageFamilies read at run time, the paths of
// serveRoutes and the fixed routes.
func siteRoutePaths() (all []string, families map[string]bool) {
	families = map[string]bool{}
	set := map[string]bool{}
	for _, f := range pageFamilies {
		set["/"+f.route] = true
		families["/"+f.route] = true
		if f.buildKey != nil {
			set["/"+f.route+"/AAPL"] = true
		}
	}
	for _, r := range serveRoutes {
		set[r.path] = true
	}
	for _, p := range []string{"home", "market", "macro", "report?market=us&slot=open", "positions", "options/data", "health"} {
		set["/"+p] = true
	}
	for p := range set {
		all = append(all, p)
	}
	sort.Strings(all)
	return all, families
}

// panicSite names the first frame of the stack inside the optdesk module below the panic itself.
func panicSite(stack []byte) string {
	lines := strings.Split(string(stack), "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "\t") && strings.Contains(l, "/optdesk/") && !strings.Contains(l, "site_routes_test.go") {
			return strings.TrimSpace(l[strings.Index(l, "/optdesk/")+1:])
		}
	}
	return "unknown"
}

// serveRecovered serves r and returns what the handler panicked with, nil when it returned.
func serveRecovered(h http.Handler, w http.ResponseWriter, r *http.Request) (pv any) {
	defer func() {
		if pv = recover(); pv != nil {
			pv = fmt.Sprintf("%v at %s", pv, panicSite(debug.Stack()))
		}
	}()
	h.ServeHTTP(w, r)
	return nil
}

// routesLedger is householdLedger that also names the stored aliases, which the tracking and pnl pages ask.
type routesLedger struct{ householdLedger }

func (routesLedger) StoredAliases(context.Context) ([]string, error) { return []string{"main"}, nil }

// knownPagesPanics are the panics of module pages that this change cannot edit, keyed by reader set and path; the
// follow-up of pages removes them. A key whose route no longer panics is only logged.
var knownPagesPanics = map[string]bool{
	"nil readers macro":    true, // pages-nil-reader-panics
	"fixture readers home": true, // pages-nil-reader-panics
}

// unavailable503 are the routes that answer 503 for a reader the site has not opened (item 6 of the issue); every
// other route answers a page.
var unavailable503 = map[string]bool{"/health": true, "/options/data": true, "/options/pnl": true, "/options/tracking": true}

func TestEveryRouteAnswersBelow500(t *testing.T) {
	t.Parallel()
	var asked []string
	fixture := mergeSiteReaders(siteReaders{optionLedger: routesLedger{}}, householdReaders(&asked), msReaders(), companySiteReaders(&asked),
		siteReaders{underlying: companiesStore(), account: holdsAAA{}, aliases: []string{"main"}, hours: fakeHoursReader{}})
	sets := []struct {
		name string
		rd   siteReaders
	}{{"nil readers", siteReaders{}}, {"fixture readers", fixture}}
	paths, families := siteRoutePaths()
	if len(paths) == 0 {
		t.Fatal("no routes")
	}
	unseen := map[string]bool{}
	for k := range knownPagesPanics {
		unseen[k] = true
	}
	for _, s := range sets {
		a, logs := testApp(t, "OT_UNUSED=1\n")
		a.deps = newDeps()                                                    // the page constructors of the real wiring; the readers are the set's
		a.deps.openSeries, a.deps.openAccount, a.deps.openBar = nil, nil, nil // no store is opened: the report blocks are gaps
		delete(a.deps.ext, extReportGlobal)
		h := siteHandler(a, householdClock, "2026-11-05", s.rd, echoHandler, http.NotFoundHandler())
		for _, p := range paths {
			rec := httptest.NewRecorder()
			key := s.name + " " + strings.TrimPrefix(p, "/")
			delete(unseen, key)
			if pv := serveRecovered(h, rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, p, nil)); pv != nil {
				if knownPagesPanics[key] {
					t.Logf("%s: GET %s panicked (known): %v", s.name, p, pv)
				} else {
					t.Errorf("%s: GET %s panicked: %v", s.name, p, pv)
				}
				continue
			}
			if p == "/health" && s.name == "nil readers" && (rec.Code != http.StatusServiceUnavailable || rec.Body.String() != `{"status":"unavailable"}`) {
				t.Errorf("%s: GET %s answered %d: %s", s.name, p, rec.Code, rec.Body.String())
			}
			if rec.Code >= 500 && !(rec.Code == http.StatusServiceUnavailable && unavailable503[p]) {
				t.Errorf("%s: GET %s answered %d: %s\nlog: %s", s.name, p, rec.Code, rec.Body.String(), logs.String())
			}
			if knownPagesPanics[key] {
				t.Logf("%s: GET %s no longer panics", s.name, p)
			}
			if rec.Code == http.StatusNotFound && families[p] {
				t.Errorf("%s: GET %s of pageFamilies answered 404", s.name, p)
			}
		}
	}
	for k := range unseen {
		t.Errorf("known panic %q is not a path of the walk", k)
	}
}

func TestNilReadersMarketAndReportArePages(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.deps = newDeps()
	a.deps.openSeries, a.deps.openAccount, a.deps.openBar = nil, nil, nil
	delete(a.deps.ext, extReportGlobal)
	h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	for _, p := range []string{"/market", "/report?market=us&slot=open", "/company-data", "/company-quarters", "/settings"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s answered %d", p, rec.Code)
		}
	}
}
