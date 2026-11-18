package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelines"
	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/valuation"
)

func sampleLinesInput() valuelinesapi.Input {
	return valuelinesapi.Input{
		V: "100", P: "3", Market: valuelinesapi.MarketUS,
		R:      valuelinesapi.RValues{US: "1.0", JP: "1.0", HKCN: "1.0"},
		Margin: []valuelinesapi.MarginRow{{PFrom: "1", M: "0.1", W: "0.05", H: "0.2", S: "0.1"}},
	}
}

func TestValueLinesIsRealCalculator(t *testing.T) {
	t.Parallel()
	c := newValueLines()
	if c == nil {
		t.Fatal("nil calculator")
	}
	in := sampleLinesInput()
	got, err := c.Lines(in)
	want, werr := valuelines.New().Lines(in)
	if err != nil || werr != nil || got.BuyHigh == "" || got.BuyHigh != want.BuyHigh {
		t.Fatalf("%q %v / %q %v", got.BuyHigh, err, want.BuyHigh, werr)
	}
}

func TestCandidatesWiresValuation(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	f.a.profile.Name = "real"
	fv := newFakeValuation()
	rd := siteReaders{cands: demoCandidates{}, ext: map[string]any{siteValuationKey: valuation.Reader(fv)}}
	h := siteHandler(f.a, candidatesClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_candidates_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if f.gotAll.Valuation == nil {
		t.Fatal("Valuation not passed")
	}
}

func TestCandidatesConfigHasValueLines(t *testing.T) {
	t.Parallel()
	fv := valuation.Reader(newFakeValuation())
	cfg := candidatesConfig(time.UTC, "r", "b", "candidates", candidateReaders{Valuation: fv})
	if cfg.ValueLines == nil || cfg.Valuation != fv {
		t.Fatalf("config %+v", cfg)
	}
	if bare := candidatesConfig(time.UTC, "r", "b", "candidates", candidateReaders{}); bare.ValueLines == nil || bare.Valuation != nil {
		t.Fatalf("bare config %+v", bare)
	}
}

func TestCandidatesNamedNoValuation(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, store.DSNEnvPages+"=pages-dsn\n")
	f.a.profile.Name = "real"
	f.a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: closeCounter{new(atomic.Int32)}}, nil
	}
	f.a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		return cNames{closeCounter: closeCounter{new(atomic.Int32)}}, nil
	}
	f.a.deps.openValuation = func(string) (valuation.Reader, valuation.Writer, io.Closer, error) {
		t.Error("valuation store opened")
		return nil, nil, nil, errors.New("opened")
	}
	if _, closeFn, err := candidatesPageNamed(t.Context(), f.a); err != nil {
		t.Fatal(err)
	} else {
		closeFn()
	}
	if f.gotAll.Valuation != nil {
		t.Fatalf("named path passed a Valuation")
	}
	if candidatesConfig(time.UTC, "r", "b", "candidates", f.gotAll).ValueLines == nil {
		t.Fatal("no ValueLines")
	}
}

func TestDemoProfileBuildsNoValuation(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	f.a.profile.Name = "demo"
	fv := valuation.Reader(newFakeValuation())
	rd := siteReaders{cands: demoCandidates{}, ext: map[string]any{siteValuationKey: fv}}
	if _, err := buildCandidates(t.Context(), f.a, candidatesClock, "2026-11-05", rd); err != nil {
		t.Fatal(err)
	}
	if f.gotAll.Valuation != nil {
		t.Fatal("demo profile passed a Valuation")
	}
	if candidatesConfig(time.UTC, "r", "b", "candidates", f.gotAll).ValueLines == nil {
		t.Fatal("no ValueLines")
	}
}
