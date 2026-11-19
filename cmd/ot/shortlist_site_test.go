package main

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionshortlistapi"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestShortlisterIsRealRule(t *testing.T) {
	t.Parallel()
	s := newShortlister()
	if s == nil {
		t.Fatal("nil shortlister")
	}
	row := func(key, delta string) optionshortlistapi.Row {
		return optionshortlistapi.Row{Key: key, Company: "ACME", Ticker: "ACM", Strike: "94", Spot: "100",
			Delta: delta, Bid: "1.10", Annualized: "0.12", DTE: "30"}
	}
	res, err := s.Select(optionshortlistapi.Input{Rows: []optionshortlistapi.Row{row("keep", "-0.22"), row("out", "-0.40")}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Shown != 1 || len(res.Groups) != 1 || len(res.Groups[0].Rows) != 1 || res.Groups[0].Rows[0].Key != "keep" {
		t.Fatalf("result %+v", res)
	}
	found := false
	for _, d := range res.Dropped {
		if d.Code == "delta_out" && d.N == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("no delta_out drop: %+v", res.Dropped)
	}
}

func TestCandidatesWiresShortlist(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, "")
	h := siteHandler(f.a, candidatesClock, "2026-11-05", siteReaders{cands: demoCandidates{}}, echoHandler, http.NotFoundHandler())
	rec := candidatesGet(t, h, http.MethodGet, "/"+siteCandidatesRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"schema":"option_candidates_page"`) {
		t.Fatalf("%d %q", rec.Code, rec.Body)
	}
	if f.gotAll.Shortlist == nil {
		t.Fatal("route passed no Shortlist")
	}
}

func TestCandidatesNamedNoShortlist(t *testing.T) {
	t.Parallel()
	f := newCandidatesFixture(t, store.DSNEnvPages+"=pages-dsn\n")
	f.a.profile.Name = "real"
	f.a.deps.openCandidate = func(context.Context, string) (candidateStore, error) {
		return cCandsReader{demoCandidates: demoCandidates{}, closeCounter: closeCounter{new(atomic.Int32)}}, nil
	}
	f.a.deps.openInstrumentName = func(context.Context, string) (store.InstrumentNameStore, error) {
		return cNames{closeCounter: closeCounter{new(atomic.Int32)}}, nil
	}
	_, closeFn, err := candidatesPageNamed(t.Context(), f.a)
	if err != nil {
		t.Fatal(err)
	}
	closeFn()
	if f.gotAll.Shortlist != nil {
		t.Fatal("named path passed a Shortlist")
	}
}
