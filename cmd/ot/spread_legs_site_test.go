package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute/optionledger"
	"github.com/andrewrong/optdesk/internal/compute/optionstruct"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

func validateSpreadLegs(t *testing.T, b []byte) map[string]any {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(schemaFileLoader{})
	sch, err := c.Compile(dataHealthSchemaHost + "page.spread_legs_page.v1.json")
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
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// spreadLedger holds no trades and notes the aliases asked for.
type spreadLedger struct {
	store.OptionLedgerReader
	mu    *sync.Mutex
	asked *[]string
}

func (l spreadLedger) FlexTradeRows(_ context.Context, alias string) ([]store.FlexTradeRow, error) {
	l.mu.Lock()
	*l.asked = append(*l.asked, alias)
	l.mu.Unlock()
	return []store.FlexTradeRow{}, nil
}

func spreadLegsReaders(asked *[]string) siteReaders {
	return siteReaders{
		ledger:     spreadLedger{mu: &sync.Mutex{}, asked: asked},
		sessions:   householdSessions{},
		ledgers:    optionledger.New,
		structures: optionstruct.New,
	}
}

func TestSpreadLegsSitePageAndRoute(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	page, closeFn, err := pageNamed(a, spreadLegsName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	validateSpreadLegs(t, got)
	if page.Meta.Name != "spread_legs_page" || page.Meta.Schema != "page.spread_legs_page.v1" {
		t.Fatalf("meta %+v", page.Meta)
	}
	var asked []string
	h := siteHandler(a, householdClock, "2026-11-05", spreadLegsReaders(&asked), echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteSpreadLegsRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"spread_legs_page"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	m := validateSpreadLegs(t, rec.Body.Bytes())
	if m["run_status"] != "ok" || len(asked) != 1 || asked[0] != "main" {
		t.Fatalf("status %v asked %v", m["run_status"], asked)
	}
}

func TestSpreadLegsSiteAccountQuery(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "")
	writeBook(t, a, "acct-a", "acct-b")
	var asked []string
	rd := spreadLegsReaders(&asked)
	rd.aliases = siteAliasList(a)
	h := siteHandler(a, householdClock, "2026-11-05", rd, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteSpreadLegsRoute+"?account=acct-b")
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	m := validateSpreadLegs(t, rec.Body.Bytes())
	if m["scope"] != "acct-b" || len(asked) != 1 || asked[0] != "acct-b" {
		t.Fatalf("scope %v asked %v", m["scope"], asked)
	}
	asked = nil
	rec = companiesGet(t, h, http.MethodGet, "/"+siteSpreadLegsRoute+"?account=nobody")
	m = validateSpreadLegs(t, rec.Body.Bytes())
	if rec.Code != 200 || m["run_status"] != "failed" || len(asked) != 0 {
		t.Fatalf("unknown alias: %d %v asked %v", rec.Code, m["run_status"], asked)
	}
}

func TestSpreadLegsSiteDemo(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, "OT_UNUSED=1\n")
	a.profile.Name = "demo"
	page, closeFn, err := pageNamed(a, spreadLegsName)
	if err != nil {
		t.Fatal(err)
	}
	defer closeFn()
	got, err := page.Build(t.Context(), nil, contracts.Day("2026-11-05"), householdClock.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	validateSpreadLegs(t, got)
	if !strings.Contains(string(got), "DEMOA") || strings.Contains(string(got), `"no_reader"`) {
		t.Fatalf("demo page %s", got)
	}
	h := siteHandler(a, householdClock, "2026-11-05", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec := companiesGet(t, h, http.MethodGet, "/"+siteSpreadLegsRoute)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "DEMOA") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	validateSpreadLegs(t, rec.Body.Bytes())
}
