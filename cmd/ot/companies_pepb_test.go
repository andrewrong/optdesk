package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/pages/companies_page"
)

func pepbPageOf(t *testing.T, in companiesInputsFixture) map[string]json.RawMessage {
	t.Helper()
	cfg := withCompaniesPepb(newCompaniesPage(fixtureBase(in.in, in.view), in.inputs, fixtureFuncs()), in.inputs)
	got, err := companies_page.Build(t.Context(), cfg, time.Date(2026, 11, 10, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var page map[string]json.RawMessage
	if err := json.Unmarshal(got, &page); err != nil {
		t.Fatal(err)
	}
	return page
}

type companiesInputsFixture struct {
	in     fixtureInput
	view   fxFakeView
	inputs companiesInputs
}

func TestCompaniesPepbWiring(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	view, inputs := fixtureStores(t, in)
	fx := companiesInputsFixture{in: in, view: view, inputs: inputs}
	if _, ok := pepbPageOf(t, fx)["pepb"]; !ok {
		t.Fatal("page with a bar reader has no pepb property")
	}
	// Without the catalog and the name reader no symbol can be resolved: the block is off.
	fx.inputs.Catalog, fx.inputs.Names = nil, nil
	if _, ok := pepbPageOf(t, fx)["pepb"]; ok {
		t.Fatal("page with no resolver has a pepb property")
	}
}

// wantNoBarsGap fails unless a row of the pepb block has the Gap no_data "no stored bars" for PE and PB.
func wantNoBarsGap(t *testing.T, page map[string]json.RawMessage) {
	t.Helper()
	raw, ok := page["pepb"]
	if !ok {
		t.Fatal("no pepb property")
	}
	var blk struct {
		Rows []struct {
			PE, PB struct{ Reason, Text string }
		}
	}
	if err := json.Unmarshal(raw, &blk); err != nil {
		t.Fatal(err)
	}
	for _, r := range blk.Rows {
		if r.PE.Reason == "no_data" && r.PE.Text == "no stored bars" && r.PB == r.PE {
			return
		}
	}
	t.Fatalf("no row with the Gap no_data \"no stored bars\": %s", raw)
}

func TestNilBarReaderGivesNoDataGap(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	view, inputs := fixtureStores(t, in)
	inputs.Bars = nil
	wantNoBarsGap(t, pepbPageOf(t, companiesInputsFixture{in: in, view: view, inputs: inputs}))
}

func TestSymbolWithNoInstrumentGivesNoDataGap(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	view, inputs := fixtureStores(t, in)
	inputs.Catalog, inputs.Names = fxFakeCatalog{}, fxFakeNames{}
	wantNoBarsGap(t, pepbPageOf(t, companiesInputsFixture{in: in, view: view, inputs: inputs}))
}
