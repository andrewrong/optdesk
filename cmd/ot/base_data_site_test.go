package main

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/store/basefact"
)

type fakeBaseFacts struct{}

func (fakeBaseFacts) Facts(context.Context) (basefact.Result, error) {
	return basefact.Result{Facts: []basefact.Fact{}, Failed: []string{}}, nil
}
func (fakeBaseFacts) Close() error { return nil }

func baseDataGet(t *testing.T, a *app, rd siteReaders, method string) (int, string) {
	t.Helper()
	h := siteHandler(a, fakeClock{dataHealthDay()}, "", rd, echoHandler, http.NotFoundHandler())
	rec := settingsGet(t, h, method, "/"+siteBaseDataRoute)
	return rec.Code, rec.Body.String()
}

func TestBaseDataRouteServesGetAndHead(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	var dsn string
	evals := 0
	a.deps.ext = map[string]any{extBaseData: baseDataDeps{
		openFacts: func(_ context.Context, d string) (basefact.Reader, error) { dsn = d; return fakeBaseFacts{}, nil },
		eval: func(basecheckapi.Input) basecheckapi.Output {
			evals++
			return basecheckapi.Output{Level: basecheckapi.Pass}
		},
	}}
	v, ok := siteOpeners[siteBaseDataFactsKey](t.Context(), a, func(io.Closer, error) bool { return true })
	if !ok || v == nil || dsn != "postgres://pages" {
		t.Fatalf("opener: %v %v dsn %q", v, ok, dsn)
	}
	rd := siteReaders{ext: map[string]any{siteBaseDataFactsKey: v}}
	code, body := baseDataGet(t, a, rd, http.MethodGet)
	if code != http.StatusOK || evals != 1 || !strings.Contains(body, `"schema":"base_data_page"`) {
		t.Fatalf("GET %d evals %d body %s", code, evals, body)
	}
	if code, body := baseDataGet(t, a, rd, http.MethodHead); code != http.StatusOK {
		t.Fatalf("HEAD %d body %q", code, body)
	}
	if code, _ := baseDataGet(t, a, rd, http.MethodPost); code != http.StatusMethodNotAllowed {
		t.Fatalf("POST %d", code)
	}
}

func TestBaseDataRouteWithoutDSNIsGap(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNRELATED=1\n")
	a.deps.ext = map[string]any{extBaseData: baseDataDeps{
		openFacts: func(context.Context, string) (basefact.Reader, error) { return nil, io.ErrUnexpectedEOF },
	}}
	v, ok := siteOpeners[siteBaseDataFactsKey](t.Context(), a, func(io.Closer, error) bool { return true })
	if !ok || v != nil {
		t.Fatalf("opener without the line: %v %v", v, ok)
	}
	code, body := baseDataGet(t, a, siteReaders{}, http.MethodGet)
	if code != http.StatusOK || !strings.Contains(body, `"reason":"facts_unavailable"`) {
		t.Fatalf("%d %s", code, body)
	}
}
