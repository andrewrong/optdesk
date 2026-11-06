package contracts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// The v2 files of market_summary and options_summary pass the three outcomes of
// checkPageV2.
func TestPageV2SummariesB(t *testing.T) {
	t.Parallel()
	overrides := map[string]string{
		"^[A-Za-z0-9_-]{1,64}$": "id_1",
	}
	for _, name := range []string{"page.market_summary.v2.json", "page.options_summary.v2.json"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			checkPageV2(t, name, overrides)
		})
	}
}

// options_summary keeps its 3-reason Gap: a gap with the reason "stale" fails,
// one with each of the three reasons passes.
func TestPageV2OptionsSummaryGapReasons(t *testing.T) {
	t.Parallel()
	const name = "page.options_summary.v2.json"
	s := compileURL(t, schemasDir, schemaHost+name)
	raw := readRaw(t, schemasDir, name)
	overrides := map[string]string{
		"^[A-Za-z0-9_-]{1,64}$": "id_1",
	}
	for reason, valid := range map[string]bool{
		"missing_quote": true, "no_data": true, "unsupported_currency": true,
		"stale": false, "not_built": false,
	} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			inst, ok := minimal(t, schemasDir, name, raw, overrides).(map[string]any)
			if !ok {
				t.Fatal("not an object schema")
			}
			card, ok := inst["card"].(map[string]any)
			if !ok {
				t.Fatal("card is not an object")
			}
			card["gaps"] = []any{map[string]any{"reason": reason, "text": "gap"}}
			b, err := json.Marshal(inst)
			if err != nil {
				t.Fatal(err)
			}
			v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
			if err != nil {
				t.Fatal(err)
			}
			err = s.Validate(v)
			if valid && err != nil {
				t.Errorf("reason %q: want valid, got %v", reason, err)
			}
			if !valid && err == nil {
				t.Errorf("reason %q: want invalid, got valid", reason)
			}
		})
	}
}
