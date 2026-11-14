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
	fx.inputs.Bars = nil
	if _, ok := pepbPageOf(t, fx)["pepb"]; ok {
		t.Fatal("page with a nil bar reader has a pepb property")
	}
}
