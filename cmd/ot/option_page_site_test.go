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
	a.deps.openInstrumentName = func(_ context.Context, d string) (store.InstrumentNameStore, error) {
		dsns = append(dsns, d)
		return cZZZNames{closeCounter: cc}, nil
	}
	body, closeFn := buildNamed(t, a)
	if !strings.Contains(body, `"schema":"option_page"`) || !strings.Contains(body, `"run_id":"ot-page"`) || strings.Contains(body, "no_reader") {
		t.Fatalf("page %s", body)
	}
	if len(dsns) != 3 || dsns[0] != "pages-dsn" || dsns[1] != "pages-dsn" || dsns[2] != "pages-dsn" || closed.Load() != 0 {
		t.Fatalf("dsns %v closed %d", dsns, closed.Load())
	}
	closeFn()
	if closed.Load() != 3 {
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

func TestOptionPageScope(t *testing.T) {
	t.Parallel()
	a, _ := optionPageApp(t, "")
	rd := siteReaders{cands: demoCandidates{}, eod: demoOption{}}
	h := siteHandler(a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/options")
	if !strings.Contains(rec.Body.String(), `"scope":"options"`) {
		t.Fatalf("scope: %s", rec.Body.String())
	}
}

// zzzNames names every underlying id ZZZ, so a page that was given the reader prints the ticker.
type zzzNames struct{}

func (zzzNames) InstrumentNames(_ context.Context, ids []int64) ([]store.InstrumentName, error) {
	out := make([]store.InstrumentName, 0, len(ids))
	for _, id := range ids {
		out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: "ZZZ"})
	}
	return out, nil
}

func TestOptionPageNamesReaderPassed(t *testing.T) {
	t.Parallel()
	a, _ := optionPageApp(t, "")
	rd := siteReaders{cands: demoCandidates{}, eod: demoOption{}, names: zzzNames{}}
	b, err := buildOptionPage(t.Context(), a, candidatesClock, "2026-11-05", rd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "ZZZ") {
		t.Fatalf("ticker missing: %s", b)
	}
}

// TestNamedOpensNameStore: both named pages open the instrument-name store with the pages line and close it once.
func TestNamedOpensNameStore(t *testing.T) {
	t.Parallel()
	for _, name := range []string{candidatesName, optionPageName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := optionPageApp(t, store.DSNEnvPages+"=pages-dsn\n")
			a.profile.Name = "real"
			a.deps.optionCandidatesPage = newOptionCandidatesPage
			var closed, nameCloses atomic.Int32
			var dsn string
			a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
				return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: closeCounter{&closed}}, nil
			}
			a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
				return cEodDemo{demoOption: demoOption{}, closeCounter: closeCounter{&closed}}, nil
			}
			a.deps.openInstrumentName = func(_ context.Context, d string) (store.InstrumentNameStore, error) {
				dsn = d
				return cZZZNames{closeCounter: closeCounter{&nameCloses}}, nil
			}
			_, closeFn, err := pageNamed(a, name)
			if err != nil {
				t.Fatal(err)
			}
			if dsn != "pages-dsn" || nameCloses.Load() != 0 {
				t.Fatalf("dsn %q closes %d", dsn, nameCloses.Load())
			}
			closeFn()
			if nameCloses.Load() != 1 {
				t.Fatalf("name store closed %d times", nameCloses.Load())
			}
		})
	}
}

// TestNamedOpenFailureClosesStores: a failed name-store open closes the stores opened before it.
func TestNamedOpenFailureClosesStores(t *testing.T) {
	t.Parallel()
	for _, name := range []string{candidatesName, optionPageName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := optionPageApp(t, store.DSNEnvPages+"=pages-dsn\n")
			a.profile.Name = "real"
			a.deps.optionCandidatesPage = newOptionCandidatesPage
			var closed atomic.Int32
			a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
				return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: closeCounter{&closed}}, nil
			}
			a.deps.openOptionEod = func(context.Context, string) (store.OptionEodStore, error) {
				return cEodDemo{demoOption: demoOption{}, closeCounter: closeCounter{&closed}}, nil
			}
			a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
				return nil, context.Canceled
			}
			if _, _, err := pageNamed(a, name); err == nil {
				t.Fatal("no error")
			}
			want := int32(1)
			if name == optionPageName {
				want = 2
			}
			if closed.Load() != want {
				t.Fatalf("closed %d, want %d", closed.Load(), want)
			}
		})
	}
}

// TestNamedNoLineNilNames: with no pages line no store is opened and the page shows its no_reader gap.
func TestNamedNoLineNilNames(t *testing.T) {
	t.Parallel()
	for _, name := range []string{candidatesName, optionPageName} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a, _ := optionPageApp(t, store.DSNEnvCandidate+"=x\n")
			a.profile.Name = "real"
			a.deps.optionCandidatesPage = newOptionCandidatesPage
			a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
				t.Error("name store opened")
				return nil, context.Canceled
			}
			p, closeFn, err := pageNamed(a, name)
			if err != nil {
				t.Fatal(err)
			}
			defer closeFn()
			b, err := p.Build(t.Context(), nil, "2026-11-05", candidatesClock.Now())
			if err != nil || !strings.Contains(string(b), "no_reader") {
				t.Fatalf("%v %s", err, b)
			}
		})
	}
}

// cZZZNames is zzzNames with a counted Close.
type cZZZNames struct {
	zzzNames
	closeCounter
}

func (c cZZZNames) Close() error { return c.closeCounter.Close() }
