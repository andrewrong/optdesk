package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compilePriceBarR2(t *testing.T) *jsonschema.Schema {
	t.Helper()
	file := "obs.price_bar.r2.json"
	f, err := os.Open(filepath.Join("..", "..", "contracts", "datasets", file))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(file, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(file)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func priceBarRecord(source string) map[string]any {
	return map[string]any{
		"instrument_id": 1, "basis": "trades", "adjust_epoch": 1, "session_day": "2026-11-18",
		"open": "10.00", "high": "11.00", "low": "9.50", "close": "10.50", "volume": "1000",
		"currency": "USD", "source": source, "received_at": "2026-11-19T01:00:00Z",
	}
}

func TestPriceBarSourceEnum(t *testing.T) {
	t.Parallel()
	s := compilePriceBarR2(t)
	for _, src := range []string{"ibkr", "yfinance"} {
		if err := s.Validate(priceBarRecord(src)); err != nil {
			t.Errorf("source %q rejected: %v", src, err)
		}
	}
	if err := s.Validate(priceBarRecord("tiingo")); err == nil {
		t.Error("source tiingo accepted")
	}
	rec := priceBarRecord("ibkr")
	delete(rec, "adjust_epoch")
	if err := s.Validate(rec); err == nil {
		t.Error("a bar without adjust_epoch accepted")
	}
}

func TestBarsSourcePageFixturesValid(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"market_state_page.v2", "company_data_page.v2", "positions_page.v4"} {
		b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page."+name+".ok.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if !pageV1Valid(t, "page."+name+".json", m) {
			t.Errorf("%s ok fixture is invalid", name)
		}
		m["bars_source"] = "tiingo"
		if pageV1Valid(t, "page."+name+".json", m) {
			t.Errorf("%s accepts bars_source tiingo", name)
		}
		m["bars_source"] = "mixed"
		m["bars_source_note"] = "x"
		if !pageV1Valid(t, "page."+name+".json", m) {
			t.Errorf("%s rejects bars_source mixed with a note", name)
		}
	}
}
