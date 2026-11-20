package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMarketStateRangeOptional(t *testing.T) {
	t.Parallel()
	const schema = "page.market_state_page.v2.json"
	load := func() map[string]any {
		b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.market_state_page.v2.ok.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	withRange := func(state string) map[string]any {
		m := load()
		row := m["market"].(map[string]any)["rows"].([]any)[0].(map[string]any)
		row["range"] = map[string]any{
			"state":      state,
			"state_text": "Normal range (trial)",
			"cuts": []any{
				map[string]any{"level": "caution", "text": "Caution at 20", "basis": "published", "source": "Cboe"},
				map[string]any{"level": "alarm", "text": "Alarm at 30", "basis": "percentile", "source": "own stored history"},
			},
			"notice": "A notice only; nothing is blocked.",
		}
		row["hint"] = "VIX 是恐慌指数，试用读法，只是提示。"
		return m
	}
	if !pageV1Valid(t, schema, load()) {
		t.Error("the ok fixture without range and hint is invalid")
	}
	if !pageV1Valid(t, schema, withRange("normal")) {
		t.Error("a row with a range and a hint is invalid")
	}
	if pageV1Valid(t, schema, withRange("red")) {
		t.Error("a range state red is accepted")
	}
}
