package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

var stockClock = fakeClock{time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)}

// stockGet serves target through the site handler with the readers rd and returns the status and the page's
// schema and top-bar symbol.
func stockGet(t *testing.T, rd siteReaders, target string) (code int, schema, symbol, shared string) {
	t.Helper()
	a, _ := testApp(t, "")
	h := siteHandler(a, stockClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, target)
	var p struct {
		Schema    string  `json:"schema"`
		SharedGap *string `json:"shared_gap"`
		Top       struct {
			Symbol string `json:"symbol"`
		} `json:"top"`
	}
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("body %q: %v", rec.Body, err)
		}
	}
	if p.SharedGap != nil {
		shared = *p.SharedGap
	}
	return rec.Code, p.Schema, p.Top.Symbol, shared
}

func TestStockRouteKey(t *testing.T) {
	t.Parallel()
	rd := siteReaders{cands: demoCandidates{}}
	code, schema, sym, shared := stockGet(t, rd, "/options/stock/DEMO")
	if code != 200 || schema != "stock_option_page" || !strings.Contains(sym, "DEMO") {
		t.Fatalf("%d %q %q", code, schema, sym)
	}
	if shared != "" && strings.Contains(shared, "No candidate rows for this underlying") {
		t.Fatalf("known key shows no_card: %q", shared)
	}
	// A key with no stored row is an empty page, not an error.
	code, schema, _, shared = stockGet(t, rd, "/options/stock/NOSUCH")
	if code != 200 || schema != "stock_option_page" || shared == "" {
		t.Fatalf("unknown key: %d %q %q", code, schema, shared)
	}
}

func TestStockRouteBareKey(t *testing.T) {
	t.Parallel()
	rd := siteReaders{cands: demoCandidates{}}
	for _, target := range []string{"/options/stock", "/options/stock/DEMO"} {
		code, schema, sym, _ := stockGet(t, rd, target)
		if code != 200 || schema != "stock_option_page" || !strings.Contains(sym, "DEMO") {
			t.Fatalf("%s: %d %q %q", target, code, schema, sym)
		}
	}
}

func TestStockRouteBadKey404(t *testing.T) {
	t.Parallel()
	rd := siteReaders{cands: demoCandidates{}}
	for _, target := range []string{
		"/options/stock/", "/options/stock/a/b", "/options/stock/a%20b", "/options/stock/a.b",
		"/options/stock/" + strings.Repeat("x", 65), "/options/stockx", "/options/stock/DEMO/",
	} {
		if code, _, _, _ := stockGet(t, rd, target); code != 404 {
			t.Errorf("%s: code %d, want 404", target, code)
		}
	}
	if code, _, _, _ := stockGet(t, rd, "/options/stock/"+strings.Repeat("x", 64)); code != 200 {
		t.Errorf("a 64-character key: code %d", code)
	}
}

func TestPageKeyRouteClash(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("a route clash with a keyed family does not panic")
		}
	}()
	registerPage(pageFamily{route: "options/stock", buildKey: stockBuild})
}

func TestStockRouteNoReaderGap(t *testing.T) {
	t.Parallel()
	code, schema, _, shared := stockGet(t, siteReaders{}, "/options/stock/DEMO")
	if code != 200 || schema != "stock_option_page" || shared == "" {
		t.Fatalf("%d %q gap %q", code, schema, shared)
	}
}

func TestStockFamilyHasNoName(t *testing.T) {
	t.Parallel()
	f, ok := pageFamilyOfRoute("options/stock")
	if !ok || f.name != "" || f.named != nil || f.buildKey == nil {
		t.Fatalf("family %+v ok %v", f, ok)
	}
	if _, ok := pageFamilyOfName("stock_option_page"); ok {
		t.Error("stock_option_page is a known name")
	}
}

func TestStockRouteCarriesTheRefreshState(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	for live, want := range map[bool]string{false: "live quotes are off", true: "no symbols on this page"} {
		rd := siteReaders{cands: demoCandidates{}, names: demoOption{}}
		if live {
			rd.live = newLiveRig(t, true, nil, liveNoon).l
		}
		h := siteHandler(a, stockClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
		body := candidatesGet(t, h, http.MethodGet, "/"+siteStockRoute).Body.String()
		if !strings.Contains(body, `"refresh":{"enabled":false`) || !strings.Contains(body, want) {
			t.Errorf("live=%v: %s", live, body)
		}
	}
}

// okView is a fake ReadingViewReader answering one ok reading for the name market.rv20 and the subject DEMO only.
type okView struct{ store.ReadingViewReader }

func (okView) WinnerReadings(_ context.Context, name, _, subject string, _, to contracts.Day) ([]contracts.ReadingRecord, error) {
	if name != "market.rv20" || subject != "DEMO" {
		return nil, nil
	}
	d, err := time.Parse("2006-01-02", string(to))
	if err != nil {
		return nil, err
	}
	v := "0.31"
	var rec contracts.ReadingRecord
	rec.Name, rec.State, rec.Value = name, "ok", &v
	rec.Day.Time = d
	return []contracts.ReadingRecord{rec}, nil
}

type volRows struct {
	VolView *struct {
		Rows []struct {
			Label string `json:"label"`
			Text  string `json:"text"`
			Gap   string `json:"gap"`
		} `json:"rows"`
	} `json:"vol_view"`
}

func stockVolRows(t *testing.T, rd siteReaders) volRows {
	t.Helper()
	a, _ := testApp(t, "")
	h := siteHandler(a, stockClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteStockRoute+"/DEMO")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	var p volRows
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.VolView == nil || len(p.VolView.Rows) != 8 {
		t.Fatalf("vol_view %+v", p.VolView)
	}
	return p
}

func TestStockPageVolView(t *testing.T) {
	t.Parallel()
	p := stockVolRows(t, siteReaders{cands: demoCandidates{}, view: okView{}})
	for _, r := range p.VolView.Rows {
		if r.Label == "rv20" && r.Text == "0.31" {
			return
		}
	}
	t.Fatalf("no row with the fake's text: %+v", p.VolView.Rows)
}

func TestStockPageNoViewReader(t *testing.T) {
	t.Parallel()
	p := stockVolRows(t, siteReaders{cands: demoCandidates{}})
	for _, r := range p.VolView.Rows {
		if r.Gap == "" || r.Text != "" {
			t.Fatalf("row %+v: want a gap and no text", r)
		}
	}
}

type demoOnlyView struct{ store.ReadingViewReader }

func (demoOnlyView) WinnerReadings(_ context.Context, name, _, subject string, _, to contracts.Day) ([]contracts.ReadingRecord, error) {
	if name != "market.rv20" || subject != "DEMO" {
		return nil, nil
	}
	d, _ := time.Parse("2006-01-02", string(to))
	v := "0.31"
	var rec contracts.ReadingRecord
	rec.Name, rec.State, rec.Value = name, "ok", &v
	rec.Day.Time = d
	return []contracts.ReadingRecord{rec}, nil
}

func TestStockPageVolViewBySubject(t *testing.T) {
	t.Parallel()
	p := stockVolRows(t, siteReaders{cands: demoCandidates{}, view: demoOnlyView{}})
	for _, r := range p.VolView.Rows {
		if r.Label == "rv20" && r.Text == "0.31" {
			return
		}
	}
	t.Fatalf("readings of DEMO's subject did not reach the page: %+v", p.VolView.Rows)
}
