package main

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/store"
)

// optionPageApp is an app with the demo zone of testApp and the real builder (newOptionPage is never replaced:
// tests run in parallel).
func optionPageApp(t *testing.T, env string) (*app, *strings.Builder) {
	t.Helper()
	a, _ := testApp(t, env)
	out := &strings.Builder{}
	a.stdout = out
	return a, out
}

func TestOptionPageRoute(t *testing.T) {
	t.Parallel()
	a, _ := optionPageApp(t, "")
	rd := siteReaders{cands: demoCandidates{}, eod: demoOption{}}
	h := siteHandler(a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteOptionRoute)
	body := rec.Body.String()
	if rec.Code != 200 || !strings.Contains(body, `"schema":"option_page"`) || !strings.Contains(body, `"run_id":"ot-serve"`) {
		t.Fatalf("%d %q", rec.Code, body)
	}
	if rec := candidatesGet(t, h, http.MethodHead, "/"+siteOptionRoute); rec.Code != 200 {
		t.Errorf("HEAD: %d", rec.Code)
	}
	if rec := candidatesGet(t, h, http.MethodPost, "/"+siteOptionRoute); rec.Code != 405 {
		t.Errorf("POST: %d", rec.Code)
	}
	if rec := candidatesGet(t, h, http.MethodGet, "/"+siteOptionRoute+"/x"); rec.Code == 200 && strings.Contains(rec.Body.String(), `"schema":"option_page"`) {
		t.Errorf("sub path served the page")
	}
}

// buildNamed builds the page ot page --name option_page selects, for 2026-11-05, and returns the bytes.
func buildNamed(t *testing.T, a *app) (string, func()) {
	t.Helper()
	p, closeFn, err := pageNamed(a, optionPageName)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.Build(t.Context(), nil, "2026-11-05", candidatesClock.Now())
	if err != nil {
		t.Fatal(err)
	}
	return string(b), closeFn
}

func TestOptionPageNamed(t *testing.T) {
	t.Parallel()
	const env = store.DSNEnvPages + "=pages-dsn\n"
	a, _ := optionPageApp(t, env)
	a.profile.Name = "real"
	var closed atomic.Int32
	cc := closeCounter{&closed}
	var dsns []string
	a.deps.openCandidate = func(_ context.Context, d string) (candidateStore, error) {
		dsns = append(dsns, d)
		return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: cc}, nil
	}
	a.deps.openOptionEod = func(_ context.Context, d string) (store.OptionEodStore, error) {
		dsns = append(dsns, d)
		return cEodDemo{demoOption: demoOption{}, closeCounter: cc}, nil
	}
	body, closeFn := buildNamed(t, a)
	if !strings.Contains(body, `"schema":"option_page"`) || !strings.Contains(body, `"run_id":"ot-page"`) || strings.Contains(body, "no_reader") {
		t.Fatalf("page %s", body)
	}
	if len(dsns) != 2 || dsns[0] != "pages-dsn" || dsns[1] != "pages-dsn" || closed.Load() != 0 {
		t.Fatalf("dsns %v closed %d", dsns, closed.Load())
	}
	closeFn()
	if closed.Load() != 2 {
		t.Fatalf("closed %d", closed.Load())
	}
}

func TestOptionPageDemoProfile(t *testing.T) {
	t.Parallel()
	a, _ := optionPageApp(t, "")
	a.profile.Name = "demo"
	a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		t.Error("candidate store opened")
		return nil, context.Canceled
	}
	a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
		t.Error("option_eod store opened")
		return nil, context.Canceled
	}
	body, closeFn := buildNamed(t, a)
	defer closeFn()
	if !strings.Contains(body, `"schema":"option_page"`) || strings.Contains(body, "no_reader") {
		t.Fatalf("page %s", body)
	}
}

func TestOptionPageNoReaderGap(t *testing.T) {
	t.Parallel()
	a, _ := optionPageApp(t, store.DSNEnvCandidate+"=x\n")
	a.profile.Name = "real"
	a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		t.Error("candidate store opened")
		return nil, context.Canceled
	}
	body, closeFn := buildNamed(t, a)
	defer closeFn()
	if !strings.Contains(body, `"schema":"option_page"`) || !strings.Contains(body, "no_reader") {
		t.Fatalf("page %s", body)
	}
}

type cCandsReader struct {
	demoCandidates
	closeCounter
}

func (c cCandsReader) Close() error { return c.closeCounter.Close() }

type cEodDemo struct {
	demoOption
	store.OptionEodWriter
	closeCounter
}

func (c cEodDemo) Close() error { return c.closeCounter.Close() }
