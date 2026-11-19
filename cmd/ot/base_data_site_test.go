package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store/basefact"
	reconstore "github.com/andrewrong/optdesk/internal/store/recon"
)

type fakeBaseFacts struct{ facts []basefact.Fact }

func (f fakeBaseFacts) Facts(context.Context) (basefact.Result, error) {
	if f.facts == nil {
		f.facts = []basefact.Fact{}
	}
	return basefact.Result{Facts: f.facts, Failed: []string{}}, nil
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

var barsFacts = []basefact.Fact{{Dataset: "bars", Rule: "rows", N: 40}}

type fakeReconReader struct{ runs []reconstore.Run }

func (f fakeReconReader) Latest(context.Context) ([]reconstore.Run, error) { return f.runs, nil }
func (fakeReconReader) Close() error                                       { return nil }

// reconRunOfDay is a finished R6 run for the day of the page with half of its bars differing.
func reconRunOfDay() reconstore.Run {
	day := dataHealthDay()
	return reconstore.Run{
		PairID: "R6", SessionDay: contracts.Day(day.AddDate(0, 0, -1).Format(time.DateOnly)), Status: "ok", StartedAt: day, FinishedAt: day,
		N: 10, Matched: 5, Diff: 5, PairsAsked: 10, PairsCompared: 10, SourceID: "yfinance", ToleranceText: "t",
	}
}

func TestBaseDataPassesReconReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	var dsn string
	rr := fakeReconReader{runs: []reconstore.Run{reconRunOfDay()}}
	a.deps.ext = map[string]any{extBaseData: baseDataDeps{
		openRecon: func(_ context.Context, d string) (reconstore.Reader, error) { dsn = d; return rr, nil },
	}}
	v, ok := siteOpeners[siteBaseDataReconKey](t.Context(), a, func(io.Closer, error) bool { return true })
	if !ok || v == nil || dsn != "postgres://pages" {
		t.Fatalf("opener: %v %v dsn %q", v, ok, dsn)
	}
	rd := siteReaders{ext: map[string]any{siteBaseDataReconKey: v, siteBaseDataFactsKey: fakeBaseFacts{facts: barsFacts}}}
	if code, body := baseDataGet(t, a, rd, http.MethodGet); code != http.StatusOK || !strings.Contains(body, `"cross_check"`) {
		t.Fatalf("GET %d body %s", code, body)
	}
	// Without the pages line the opener gives no reader and the page has no cross_check.
	b, _ := testApp(t, "OT_UNRELATED=1\n")
	b.deps.ext = a.deps.ext
	if v, ok := siteOpeners[siteBaseDataReconKey](t.Context(), b, func(io.Closer, error) bool { return true }); !ok || v != nil {
		t.Fatalf("opener without the line: %v %v", v, ok)
	}
	if _, body := baseDataGet(t, b, siteReaders{}, http.MethodGet); strings.Contains(body, `"cross_check"`) {
		t.Fatalf("cross_check without a reader: %s", body)
	}
}

func TestBaseDataPassesVerdictFunc(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_PG_DSN_PAGES=postgres://pages\n")
	rd := siteReaders{ext: map[string]any{
		siteBaseDataReconKey: fakeReconReader{runs: []reconstore.Run{reconRunOfDay()}},
		siteBaseDataFactsKey: fakeBaseFacts{facts: barsFacts},
	}}
	// Half of 10 bars differ: recon.Verdict makes that a fail; the page without the function could not say it.
	_, body := baseDataGet(t, a, rd, http.MethodGet)
	var m struct {
		Groups []struct {
			Rows []struct {
				Dataset    string `json:"dataset"`
				CrossCheck *struct {
					Level string `json:"level"`
				} `json:"cross_check"`
			} `json:"rows"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	for _, g := range m.Groups {
		for _, r := range g.Rows {
			if r.Dataset == "bars" && r.CrossCheck != nil {
				if r.CrossCheck.Level != "fail" {
					t.Fatalf("pair level %q, want fail", r.CrossCheck.Level)
				}
				return
			}
		}
	}
	t.Fatalf("no cross_check on the bars row: %s", body)
}
