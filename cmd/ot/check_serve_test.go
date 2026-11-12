package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

// serveBody is a page body: its schema, its day and its blocks (under "blocks" for report_page, flat otherwise).
func serveBody(t *testing.T, page, day string, blocks ...string) []byte {
	t.Helper()
	body := map[string]any{"schema": page, "schema_version": 1}
	if k, ok := pageDayKeys[page]; ok {
		body[k] = day
	} else if page != "flex_import" { // the page declares no day
		body["data_times"] = []map[string]string{{"source": "x", "data_time": day + "T00:00:00Z"}}
	}
	inner := map[string]any{}
	for _, b := range blocks {
		inner[b] = []int{}
	}
	if page == "report_page" {
		body["blocks"] = inner
	} else {
		for k, v := range inner {
			body[k] = v
		}
	}
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// serveRepo is a checkout whose schemas accept any object that has "schema".
func serveRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sd := filepath.Join(dir, "contracts", "schemas")
	if err := os.MkdirAll(sd, 0o750); err != nil {
		t.Fatal(err)
	}
	for page := range checkBlockTable {
		doc := `{"$id":"https://optdesk.invalid/schemas/page.` + page + `.v1.json","type":"object","required":["schema"]}`
		if err := os.WriteFile(filepath.Join(sd, "page."+page+".v1.json"), []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// goodBodies answers every route of serveRoutes with a body that holds the blocks of the table on day.
func goodBodies(t *testing.T, day string) map[string][]byte {
	t.Helper()
	m := map[string][]byte{}
	for _, rt := range serveRoutes {
		m[rt.path] = serveBody(t, rt.page, day, checkBlockTable[rt.page]...)
	}
	return m
}

// fakeSite serves bodies by request URI through the handler the check hands over, as siteHandler does; a path
// in status answers that code instead. It records the URIs it was called with.
func fakeSite(bodies map[string][]byte, status map[string]int, calls *[]string) siteMaker {
	return func(nh pages.NewHandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*calls = append(*calls, r.URL.RequestURI())
			if code, ok := status[r.URL.RequestURI()]; ok {
				http.Error(w, "internal error", code)
				return
			}
			nh(bodies[r.URL.RequestURI()]).ServeHTTP(w, r)
		})
	}
}

func serveEnv(t *testing.T) (*checkEnv, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	a := checkApp(t, map[string]string{}, &out, nil, nil)
	return &checkEnv{a: a, c: &checker{w: &out}}, &out
}

func lines(out *bytes.Buffer) []string { return strings.Split(strings.TrimSpace(out.String()), "\n") }

func TestCheckServeRoutes(t *testing.T) {
	t.Parallel()
	e, out := serveEnv(t)
	var calls []string
	bodies := goodBodies(t, "2026-11-06")
	checkServeRoutes(context.Background(), e, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bodies, nil, &calls), nil)
	if e.c.failed {
		t.Fatalf("a good site failed:\n%s", out)
	}
	if len(calls) != len(serveRoutes) || len(serveRoutes) != 13 {
		t.Fatalf("calls %v", calls)
	}
	fresh := 0
	for _, l := range lines(out) {
		if strings.Contains(l, ".fresh ") {
			fresh++
			if strings.Contains(l, "serve.flex_import.fresh") { // a page with no data day is explained
				if !strings.HasPrefix(l, "EXPLAINED ") {
					t.Errorf("fresh line %q", l)
				}
				continue
			}
			if !strings.HasPrefix(l, "PASS ") || !strings.Contains(l, " 2026-11-06 ") {
				t.Errorf("fresh line %q", l)
			}
		}
	}
	if fresh != len(serveRoutes) {
		t.Errorf("%d freshness lines:\n%s", fresh, out)
	}

	// a status that is not 200 names the page; the other routes are still called.
	e2, out2 := serveEnv(t)
	var calls2 []string
	checkServeRoutes(context.Background(), e2, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bodies, map[string]int{"/market": 500}, &calls2),
		func(_ context.Context, rt serveRoute) error {
			if rt.page != "market_page" {
				t.Errorf("diagnosed %s", rt.page)
			}
			return errors.New("relation rd.x does not exist")
		})
	want500 := "FAIL serve.market_page.status - market_page answered 500: internal error: build error: relation rd.x does not exist"
	if !e2.c.failed || !strings.Contains(out2.String(), want500) {
		t.Fatalf("500 not reported:\n%s", out2)
	}
	if len(calls2) != len(serveRoutes) {
		t.Errorf("calls %v", calls2)
	}

	// a body that declares a schema version with no file is FAIL.
	bad := goodBodies(t, "2026-11-06")
	bad["/macro"] = []byte(`{"schema":"macro_rates","schema_version":7,"session_day":"2026-11-06","series":[]}`)
	e3, out3 := serveEnv(t)
	var calls3 []string
	checkServeRoutes(context.Background(), e3, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bad, nil, &calls3), nil)
	if !e3.c.failed || !strings.Contains(out3.String(), "FAIL serve.macro_rates.schema - ") {
		t.Fatalf("schema failure not reported:\n%s", out3)
	}
}

func TestCheckBlockMissing(t *testing.T) {
	t.Parallel()
	e, out := serveEnv(t)
	var calls []string
	bodies := goodBodies(t, "2026-11-06")
	bodies["/market"] = serveBody(t, "market_page", "2026-11-06", "weather", "indices", "breadth")
	checkServeRoutes(context.Background(), e, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bodies, nil, &calls), nil)
	if !e.c.failed || !strings.Contains(out.String(), "FAIL serve.market_page.blocks 2026-11-06 missing block term\n") {
		t.Fatalf("missing block not reported:\n%s", out)
	}
	for _, l := range lines(out) {
		if strings.HasPrefix(l, "FAIL") && !strings.Contains(l, "market_page") {
			t.Errorf("other page failed: %s", l)
		}
	}
}

func TestCheckBlockUnknown(t *testing.T) {
	t.Parallel()
	e, out := serveEnv(t)
	var calls []string
	bodies := goodBodies(t, "2026-11-06")
	blocks := append([]string{"candidates"}, checkBlockTable["report_page"]...)
	bodies["/report?market=us&slot=open"] = serveBody(t, "report_page", "2026-11-06", blocks...)
	checkServeRoutes(context.Background(), e, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bodies, nil, &calls), nil)
	if !strings.Contains(out.String(), "FAIL serve.report_page.us.open.blocks 2026-11-06 unknown block candidates\n") {
		t.Fatalf("unknown block not reported:\n%s", out)
	}
}

func TestCheckReportVariants(t *testing.T) {
	t.Parallel()
	e, out := serveEnv(t)
	var calls []string
	bodies := goodBodies(t, "2026-11-06")
	bodies["/report?market=jp&slot=close"] = serveBody(t, "report_page", "2026-11-06", "actions", "account_moves", "risks", "market_watch")
	checkServeRoutes(context.Background(), e, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bodies, nil, &calls), nil)
	got := map[string]bool{}
	for _, c := range calls {
		if strings.HasPrefix(c, "/report") {
			got[c] = true
		}
	}
	for _, want := range []string{"/report?market=us&slot=open", "/report?market=us&slot=close", "/report?market=jp&slot=open", "/report?market=jp&slot=close"} {
		if !got[want] {
			t.Errorf("variant %s not called", want)
		}
	}
	if len(got) != 4 {
		t.Errorf("report calls %v", got)
	}
	for _, v := range []string{"us.open", "us.close", "jp.open"} {
		if !strings.Contains(out.String(), "PASS serve.report_page."+v+".blocks") {
			t.Errorf("variant %s not PASS:\n%s", v, out)
		}
	}
	if !strings.Contains(out.String(), "FAIL serve.report_page.jp.close.blocks 2026-11-06 missing block global_assets\n") {
		t.Errorf("jp close not compared with the one list:\n%s", out)
	}
}

type serveGaps struct{ exps []store.Expectation }

func (f serveGaps) OpenGaps(context.Context) ([]store.Gap, error) { return nil, nil }
func (f serveGaps) Expectations(context.Context, contracts.Day, contracts.Day) ([]store.Expectation, error) {
	return f.exps, nil
}
func (f serveGaps) Close() error { return nil }

func TestCheckServeStaleDay(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	e, out := serveEnv(t)
	e.a.getenv = func(k string) string {
		if k == store.DSNEnvPages {
			return checkSecretDSN
		}
		return ""
	}
	e.a.deps.openGapReader = func(context.Context, string) (store.GapReader, error) {
		return serveGaps{exps: []store.Expectation{
			{DatasetID: "a", ExpectedDay: "2026-11-06", DueBy: now.Add(-48 * time.Hour)},
			{DatasetID: "b", ExpectedDay: "2026-11-09", DueBy: now.Add(-time.Hour)},
			{DatasetID: "c", ExpectedDay: "2026-11-10", DueBy: now.Add(time.Hour)}, // window open
			{DatasetID: "d", ExpectedDay: "2026-11-08", DueBy: now.Add(-time.Hour), Closed: true},
		}}, nil
	}
	latest, err := latestClosedExpected(context.Background(), e, now)
	if err != nil || latest != "2026-11-09" {
		t.Fatalf("latest %q err %v", latest, err)
	}
	var calls []string
	bodies := goodBodies(t, "2026-11-06")
	checkServeRoutes(context.Background(), e, serveRepo(t), freshness{latest: latest}, fakeSite(bodies, nil, &calls), nil)
	if !strings.Contains(out.String(), "FAIL serve.macro_rates.fresh 2026-11-06 older than the latest closed window 2026-11-09\n") {
		t.Fatalf("stale page not FAIL:\n%s", out)
	}
	// no closed window at all is an error, and every page's freshness line says so.
	e.a.deps.openGapReader = func(context.Context, string) (store.GapReader, error) { return serveGaps{}, nil }
	if _, err := latestClosedExpected(context.Background(), e, now); err == nil {
		t.Fatal("no window: want an error")
	}
}

func TestCheckBlockUnknownFlatPage(t *testing.T) {
	t.Parallel()
	body := map[string]json.RawMessage{
		"schema": json.RawMessage(`"market_page"`), "schema_version": json.RawMessage(`2`),
		"weather": json.RawMessage(`{}`), "indices": json.RawMessage(`{}`),
		"breadth": json.RawMessage(`{}`), "term": json.RawMessage(`{}`),
		"rogue_block": json.RawMessage(`{}`),
	}
	got := compareBlocks("market_page", body)
	if len(got) != 1 || got[0] != "unknown block rogue_block" {
		t.Fatalf("want [unknown block rogue_block], got %v", got)
	}
}

func TestCheckBlockUnknownFlatRoute(t *testing.T) {
	t.Parallel()
	e, out := serveEnv(t)
	var calls []string
	bodies := goodBodies(t, "2026-11-06")
	bodies["/settings"] = serveBody(t, "settings_page", "2026-11-06", "params", "rows", "history", "extra")
	checkServeRoutes(context.Background(), e, serveRepo(t), freshness{latest: "2026-11-06"}, fakeSite(bodies, nil, &calls), nil)
	if !strings.Contains(out.String(), "FAIL serve.settings_page.blocks 2026-11-06 unknown block extra\n") {
		t.Fatalf("flat unknown block not reported:\n%s", out)
	}
}

func TestCheckRepoDirFlag(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"serve"}, "."},
		{[]string{"serve", "--repo-dir", "/some/checkout"}, "/some/checkout"},
	} {
		var out bytes.Buffer
		a := checkApp(t, map[string]string{"OT_HOME": "/h", store.DSNEnvPages: checkSecretDSN}, &out, nil, nil)
		var got string
		svcs := map[string]checkService{"serve": func(_ context.Context, e *checkEnv) { got = e.opts.RepoDir }}
		if err := runCheck(a, tc.args, svcs); err != nil {
			t.Fatal(err)
		}
		if got != tc.want {
			t.Errorf("%v: repo dir %q, want %q", tc.args, got, tc.want)
		}
	}
}
