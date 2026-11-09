package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/optionresult"
	"github.com/andrewrong/optdesk/internal/compute/optionstruct"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
	"github.com/andrewrong/optdesk/internal/pages/structures_page"
	"github.com/andrewrong/optdesk/internal/store"
)

var positionsClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

// positionsCall is what a fake page constructor was given.
type positionsCall struct {
	page, alias, id string
	cfg             structures_page.Config
	res             option_result_page.Config
	day             contracts.Day
}

type positionsRec struct {
	mu    sync.Mutex
	calls []positionsCall
}

func (r *positionsRec) add(c positionsCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, c)
}

func (r *positionsRec) last(t *testing.T) positionsCall {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		t.Fatal("no page was built")
	}
	return r.calls[len(r.calls)-1]
}

func fakeBuild(rec *positionsRec, c positionsCall, vm string) pages.Page {
	return pages.Page{Build: func(_ context.Context, _ store.SeriesReader, day contracts.Day, _ time.Time) ([]byte, error) {
		c.day = day
		rec.add(c)
		return []byte(vm), nil
	}}
}

// positionsApp is an app whose three page constructors are fakes that record what they were given.
func positionsApp(t *testing.T) (*app, *positionsRec) {
	t.Helper()
	a, _ := testApp(t, "")
	rec := &positionsRec{}
	a.deps.structuresListPage = func(c structures_page.Config, alias string) pages.Page {
		return fakeBuild(rec, positionsCall{page: "list", alias: alias, cfg: c},
			`{"schema":"structures_list","scope":"`+alias+`","sentence":"s","note":"n"}`)
	}
	a.deps.structureDetailPage = func(c structures_page.Config, alias, id string) pages.Page {
		return fakeBuild(rec, positionsCall{page: "detail", alias: alias, id: id, cfg: c},
			`{"schema":"structure_detail","scope":"`+alias+`","sentence":"s"}`)
	}
	a.deps.optionResultPage = func(c option_result_page.Config, alias string) pages.Page {
		return fakeBuild(rec, positionsCall{page: "result", alias: alias, res: c},
			`{"schema":"option_result_page","scope":"`+alias+`","nothing":"n"}`)
	}
	return a, rec
}

func positionsGet(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, target, nil))
	return rec
}

type fakeLedgerReader struct{ store.OptionLedgerReader }

func positionsReaders() siteReaders {
	return siteReaders{ledger: fakeLedgerReader{}, ledgers: optionledger.New, structures: optionstruct.New, results: optionresult.New}
}

func TestStructuresRouteServesList(t *testing.T) {
	t.Parallel()
	a, rec := positionsApp(t)
	h := siteHandler(a, positionsClock, "2026-11-05", positionsReaders(), echoHandler, http.NotFoundHandler())
	r := positionsGet(t, h, http.MethodGet, "/"+siteStructuresRoute)
	c := rec.last(t)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"schema":"structures_list"`) {
		t.Fatalf("%d %q", r.Code, r.Body)
	}
	if c.page != "list" || c.alias != "main" || c.day != "2026-11-05" || c.cfg.Ledger == nil || c.cfg.Structures == nil ||
		len(c.cfg.Aliases) != 1 || c.cfg.Aliases[0] != "main" || c.cfg.Zone == nil {
		t.Errorf("%+v", c)
	}
	if h := positionsGet(t, h, http.MethodHead, "/"+siteStructuresRoute); h.Code != 200 {
		t.Errorf("HEAD %d", h.Code)
	}
	p := positionsGet(t, h, http.MethodPost, "/"+siteStructuresRoute)
	if p.Code != 405 || p.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST %d", p.Code)
	}
}

func TestStructureRouteServesDetail(t *testing.T) {
	t.Parallel()
	a, rec := positionsApp(t)
	h := siteHandler(a, positionsClock, "2026-11-05", positionsReaders(), echoHandler, http.NotFoundHandler())
	r := positionsGet(t, h, http.MethodGet, "/"+siteStructureRoutePrefix+"main:T123:1")
	c := rec.last(t)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"schema":"structure_detail"`) || c.page != "detail" ||
		c.id != "main:T123:1" || c.alias != "main" {
		t.Fatalf("%d %q %+v", r.Code, r.Body, c)
	}
	if e := positionsGet(t, h, http.MethodGet, "/"+siteStructureRoutePrefix); e.Code != 404 {
		t.Errorf("empty id: %d", e.Code)
	}
}

func TestResultRouteServesPage(t *testing.T) {
	t.Parallel()
	a, rec := positionsApp(t)
	h := siteHandler(a, positionsClock, "", positionsReaders(), echoHandler, http.NotFoundHandler())
	r := positionsGet(t, h, http.MethodGet, "/"+siteResultRoute)
	c := rec.last(t)
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"schema":"option_result_page"`) || c.page != "result" ||
		c.res.Ledger == nil || c.res.Results == nil || c.res.RunID != "ot-serve" || c.day != "2026-11-06" {
		t.Fatalf("%d %q %+v", r.Code, r.Body, c)
	}
}

func TestAccountQueryDefaultsToMain(t *testing.T) {
	t.Parallel()
	a, rec := positionsApp(t)
	h := siteHandler(a, positionsClock, "2026-11-05", positionsReaders(), echoHandler, http.NotFoundHandler())
	for _, target := range []string{"/" + siteStructuresRoute, "/" + siteStructuresRoute + "?account=main", "/" + siteResultRoute + "?account="} {
		positionsGet(t, h, http.MethodGet, target)
		if c := rec.last(t); c.alias != "main" || c.cfg.Ledger == nil && c.res.Ledger == nil {
			t.Errorf("%s: %+v", target, c)
		}
	}
}

func TestUnknownAccountIsGapPage(t *testing.T) {
	t.Parallel()
	a, rec := positionsApp(t)
	h := siteHandler(a, positionsClock, "2026-11-05", positionsReaders(), echoHandler, http.NotFoundHandler())
	for _, target := range []string{"/" + siteStructuresRoute, "/" + siteStructureRoutePrefix + "x", "/" + siteResultRoute} {
		r := positionsGet(t, h, http.MethodGet, target+"?account=zz")
		c := rec.last(t)
		if r.Code != 200 || !strings.Contains(r.Body.String(), unknownAccount) || c.alias != "zz" ||
			c.cfg.Ledger != nil || c.res.Ledger != nil {
			t.Errorf("%s: %d %q %+v", target, r.Code, r.Body, c)
		}
	}
}

// demoServeOpensNoStore checks that the demo profile opens no store for ot serve and that its positions
// pages are gap pages with the demo text; it is a case of TestDemoProfileOpensNoStore.
func demoServeOpensNoStore(t *testing.T) {
	t.Helper()
	a, rec := positionsApp(t)
	a.profile.Name = "demo"
	a.deps.openSeries = func(context.Context, string) (store.SeriesStore, error) {
		t.Error("series store opened")
		return nil, context.Canceled
	}
	rd, closeFn, err := openSiteReaders(t.Context(), a)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	if rd.ledger != nil || rd.account != nil {
		t.Errorf("demo readers %+v", rd)
	}
	h := siteHandler(a, positionsClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	r := positionsGet(t, h, http.MethodGet, "/"+siteStructuresRoute)
	if r.Code != 200 || !strings.Contains(r.Body.String(), demoNoFlexText) || rec.last(t).cfg.Ledger != nil {
		t.Fatalf("%d %q", r.Code, r.Body)
	}
}

func TestStructureIdEscaped(t *testing.T) {
	t.Parallel()
	a, rec := positionsApp(t)
	h := siteHandler(a, positionsClock, "2026-11-05", positionsReaders(), echoHandler, http.NotFoundHandler())
	r := positionsGet(t, h, http.MethodGet, "/"+siteStructureRoutePrefix+"main%3AT%201%3F%231?account=main")
	if c := rec.last(t); r.Code != 200 || c.id != "main:T 1?#1" || c.alias != "main" {
		t.Fatalf("%d %+v", r.Code, c)
	}
}

func TestNoReaderIsGapPage(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	d := newDeps()
	a.deps.structuresListPage, a.deps.structureDetailPage, a.deps.optionResultPage =
		d.structuresListPage, d.structureDetailPage, d.optionResultPage
	rd := withPositionsRules(a, siteReaders{})
	h := siteHandler(a, positionsClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	for target, want := range map[string]string{
		"/" + siteStructuresRoute:                  "no_reader",
		"/" + siteStructureRoutePrefix + "main:T1": "failed",
		"/" + siteResultRoute:                      "没有读取器",
	} {
		r := positionsGet(t, h, http.MethodGet, target)
		if r.Code != 200 || !strings.Contains(r.Body.String(), want) {
			t.Errorf("%s: %d %q", target, r.Code, r.Body)
		}
	}
}
