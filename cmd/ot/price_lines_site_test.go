package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type priceLinesVM struct {
	Saved *string `json:"saved"`
	Rows  []struct {
		Symbol string  `json:"symbol"`
		Price  *string `json:"price"`
	} `json:"rows"`
	Unset []struct{ Symbol string } `json:"unset"`
	Gaps  []struct{ Reason string } `json:"gaps"`
}

func priceLinesGet(t *testing.T, h http.Handler, query string) (*httptest.ResponseRecorder, priceLinesVM) {
	t.Helper()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/plan/price-lines"+query, nil)
	rec := serve(h, r)
	var vm priceLinesVM
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &vm); err != nil {
			t.Fatalf("%v %.300q", err, rec.Body)
		}
	}
	return rec, vm
}

func TestPriceLinesPageRows(t *testing.T) {
	t.Parallel()
	h, _ := priceLineSite(t)
	rec, vm := priceLinesGet(t, h, "")
	if rec.Code != http.StatusOK || len(vm.Rows) != 2 || vm.Rows[0].Symbol != "BBB" || *vm.Rows[0].Price != "50" || vm.Rows[1].Symbol != "CCC" {
		t.Fatalf("%d %+v", rec.Code, vm)
	}
	// AAA is held with no price; CCC is priced; no other name is held.
	if len(vm.Unset) != 1 || vm.Unset[0].Symbol != "AAA" {
		t.Fatalf("unset %+v", vm.Unset)
	}
}

func TestPriceLinesReadOnly(t *testing.T) {
	t.Parallel()
	h, st, _ := underlyingsSite(t, false)
	rec, _ := priceLinesGet(t, h, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %d", rec.Code)
	}
	post := priceLinePost(t, url.Values{"action": {"delete"}, "symbol": {"BBB"}, "market": {"US"}})
	if rec := serve(h, post); rec.Code != http.StatusMethodNotAllowed || st.calls() != 0 {
		t.Fatalf("POST %d %d", rec.Code, st.calls())
	}
}

func TestPriceLinesGetWithOwnerEdit(t *testing.T) {
	t.Parallel()
	h, _ := priceLineSite(t)
	if rec, _ := priceLinesGet(t, h, ""); rec.Code != http.StatusOK {
		t.Fatalf("%d", rec.Code)
	}
}

func TestPriceLinesNoReader(t *testing.T) {
	t.Parallel()
	a, _ := testApp(t, underlyingsEnv)
	h := siteHandler(a, fakeClock{postNow}, "", siteReaders{}, echoHandler, http.NotFoundHandler())
	rec, vm := priceLinesGet(t, h, "")
	if rec.Code != http.StatusOK || len(vm.Gaps) != 1 || vm.Gaps[0].Reason != "no_reader" || len(vm.Rows) != 0 {
		t.Fatalf("%d %+v", rec.Code, vm)
	}
}

func TestPriceLinesSavedText(t *testing.T) {
	t.Parallel()
	h, _ := priceLineSite(t)
	for q, want := range map[string]string{"?saved=1": "Saved 1 row(s).", "?saved=0": "Nothing changed.", "?saved=x": ""} {
		_, vm := priceLinesGet(t, h, q)
		got := ""
		if vm.Saved != nil {
			got = *vm.Saved
		}
		if got != want || (want == "" && strings.Contains(q, "x") && vm.Saved != nil) {
			t.Fatalf("%s: %q want %q", q, got, want)
		}
	}
}
