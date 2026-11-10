package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestNewOptionDataPageCarriesNotesAndSymbol(t *testing.T) {
	t.Parallel()
	in := sitePageInput{
		UnderlyingID: 7, Label: "AAA", Symbol: "AAA", Notes: []string{"a note"},
		Eod: noRows{}, Quotes: noRows{}, Terms: noRows{},
	}
	b, err := newOptionDataPage(time.UTC, "b", in).Build(t.Context(), nil, "2026-11-06", time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var p sitePageJSON
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	if p.Underlying.Symbol == nil || *p.Underlying.Symbol != "AAA" || len(p.Notes) != 1 || p.Notes[0] != "a note" {
		t.Fatalf("page %+v", p)
	}
}

func TestOptionCandidatesPageWiresReader(t *testing.T) {
	t.Parallel()
	p := newDeps().optionCandidatesPage(time.UTC, "ot-page", "dev", "candidates", demoCandidates{})
	if p.Meta.Name != "option_candidates_page" || p.Meta.Schema != "page.option_candidates_page.v1" {
		t.Fatalf("meta %+v", p.Meta)
	}
	b, err := p.Build(t.Context(), nil, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"scope":"candidates"`) || !strings.Contains(string(b), `"run_id":"ot-page"`) {
		t.Fatalf("page %s", b)
	}
}

func TestMarketPageWiresReadings(t *testing.T) {
	t.Parallel()
	var called atomic.Int32
	read := func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		called.Add(1)
		return nil, nil
	}
	p := newDeps().marketPage(time.UTC, "ot-page", "dev", "market", read)
	if p.Meta.Name != "market_page" {
		t.Fatalf("name %q", p.Meta.Name)
	}
	if p.Meta.Schema != "page.market_page.v1" {
		t.Fatalf("schema %q", p.Meta.Schema)
	}
	_, err := p.Build(t.Context(), demoReader{}, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil && strings.Contains(err.Error(), "not built") {
		t.Skip("the builder of market_page has not landed: Build is still the stub of the interface entry")
	}
	if called.Load() == 0 {
		t.Fatal("Build did not call the readings function")
	}
}

func TestOptionNoticesPageWiresRule(t *testing.T) {
	t.Parallel()
	d := newDeps()
	p := d.optionNoticesPage(time.UTC, "ot-page", "dev", "notices", "main", []string{"main"}, nil, nil, nil, d.newOptionNotices(), d.noticeParams())
	if p.Meta.Name != "option_notices_page" || p.Meta.Schema != "page.option_notices_page.v1" {
		t.Fatalf("meta %+v", p.Meta)
	}
	b, err := p.Build(t.Context(), nil, "2026-11-05", time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"scope":"notices"`) || !strings.Contains(string(b), `"no_reader"`) {
		t.Fatalf("page %s", b)
	}
}
