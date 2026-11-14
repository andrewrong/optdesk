package main

import (
	"encoding/json"
	"strings"
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

func TestNilBarReaderGivesNoDataGap(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	view, inputs := fixtureStores(t, in)
	inputs.Bars = nil
	page := pepbPageOf(t, companiesInputsFixture{in: in, view: view, inputs: inputs})
	raw, ok := page["pepb"]
	if !ok || !strings.Contains(string(raw), `"no_data"`) {
		t.Fatal("nil bar reader: want a Gap no_data (acceptance 3)")
	}
}

func TestSymbolWithNoInstrumentGivesNoDataGap(t *testing.T) {
	t.Parallel()
	in := loadFixtureInput(t)
	view, inputs := fixtureStores(t, in)
	inputs.Catalog, inputs.Names = fxFakeCatalog{}, fxFakeNames{}
	page := pepbPageOf(t, companiesInputsFixture{in: in, view: view, inputs: inputs})
	raw, ok := page["pepb"]
	if !ok || !strings.Contains(string(raw), `"no_data"`) {
		t.Fatal("symbol with no instrument id: want a Gap no_data")
	}
}
