package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func validateHeldFamily(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(schemaFileLoader{})
	sch, err := c.Compile(dataHealthSchemaHost + "page.held_family_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
}

func TestHeldFamilySite(t *testing.T) {
	t.Parallel()
	t.Run("ot page and the route give a valid payload", func(t *testing.T) {
		t.Parallel()
		a, _ := testApp(t, "OT_UNUSED=1\n")
		page, closeFn, err := pageNamed(a, heldFamilyName)
		if err != nil {
			t.Fatal(err)
		}
		defer closeFn()
		got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		validateHeldFamily(t, got)
		var asked []string
		h := siteHandler(a, householdClock, "2026-11-05", householdReaders(&asked), echoHandler, http.NotFoundHandler())
		rec := companiesGet(t, h, http.MethodGet, "/"+siteHeldFamilyRoute)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"held_family_page"`) {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		validateHeldFamily(t, rec.Body.Bytes())
	})
	t.Run("demo profile gives fixed rows with no read", func(t *testing.T) {
		t.Parallel()
		a, _ := testApp(t, "OT_UNUSED=1\n")
		a.profile.Name = "demo"
		page, closeFn, err := pageNamed(a, heldFamilyName)
		if err != nil {
			t.Fatal(err)
		}
		defer closeFn()
		got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		validateHeldFamily(t, got)
		if !strings.Contains(string(got), "DEMOA") || strings.Contains(string(got), `"no_reader"`) {
			t.Fatalf("demo page %s", got)
		}
		h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
		rec := companiesGet(t, h, http.MethodGet, "/"+siteHeldFamilyRoute)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "DEMOB") {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		validateHeldFamily(t, rec.Body.Bytes())
	})
}
