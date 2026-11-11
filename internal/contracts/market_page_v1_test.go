package contracts_test

import (
	"testing"
)

const marketPageV1 = "page.market_page.v1.json"

// pageV1Minimal builds the smallest instance of a page.<name>.v1.json whose strings follow the
// Id pattern; extra maps further patterns to a value.
func pageV1Minimal(t *testing.T, name string, extra map[string]string) map[string]any {
	t.Helper()
	overrides := map[string]string{"^[A-Za-z0-9_-]{1,64}$": "id_1"}
	for k, v := range extra {
		overrides[k] = v
	}
	raw := readRaw(t, schemasDir, name)
	inst, ok := minimal(t, schemasDir, name, raw, overrides).(map[string]any)
	if !ok {
		t.Fatalf("%s is not an object schema", name)
	}
	return inst
}

func pageV1Gap() map[string]any {
	return map[string]any{"kind": "gap", "reason": "no_data", "text": "no data yet"}
}

func pageV1Figure() map[string]any {
	return map[string]any{"kind": "value", "value": 1.5, "text": "1.5", "unit": "percent"}
}

// pageV1Valid reports whether the instance validates against the page schema.
func pageV1Valid(t *testing.T, name string, m map[string]any) bool {
	t.Helper()
	return compilePageV2(t, name).Validate(pageV2JSON(t, m)) == nil
}

func marketChartPoint() map[string]any {
	return map[string]any{"label": "d1", "close": 1.0, "ma200": 1.0}
}

func TestMarketPageMinimalValid(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, marketPageV1, pageV1Minimal(t, marketPageV1, nil)) {
		t.Fatal("minimal instance is invalid")
	}
}

func TestMarketPageVersion2Fails(t *testing.T) {
	t.Parallel()
	inst := pageV1Minimal(t, marketPageV1, nil)
	inst["schema_version"] = 2
	if pageV1Valid(t, marketPageV1, inst) {
		t.Fatal("version 2 validates")
	}
}

func TestMarketPageIndexRowGapValid(t *testing.T) {
	t.Parallel()
	inst := pageV1Minimal(t, marketPageV1, nil)
	inst["indices"].(map[string]any)["rows"] = []any{map[string]any{
		"name": "SPX", "iv": pageV1Gap(), "rv20": pageV1Figure(), "iv_minus_rv20": pageV1Figure(), "rank_3y": pageV1Figure(),
	}}
	if !pageV1Valid(t, marketPageV1, inst) {
		t.Fatal("IndexRow with a Gap iv is invalid")
	}
}

func TestMarketPageBlocksAsGapValid(t *testing.T) {
	t.Parallel()
	for _, block := range []string{"weather.chart", "term", "breadth"} {
		t.Run(block, func(t *testing.T) {
			t.Parallel()
			inst := pageV1Minimal(t, marketPageV1, nil)
			if block == "weather.chart" {
				inst["weather"].(map[string]any)["chart"] = pageV1Gap()
			} else {
				inst[block] = pageV1Gap()
			}
			if !pageV1Valid(t, marketPageV1, inst) {
				t.Fatalf("%s as a Gap is invalid", block)
			}
		})
	}
}

func TestMarketPageChartPointNeedsMA200(t *testing.T) {
	t.Parallel()
	inst := pageV1Minimal(t, marketPageV1, nil)
	chart := map[string]any{"aria_text": "a", "series_names": []any{"close"},
		"points": []any{marketChartPoint()}}
	inst["weather"].(map[string]any)["chart"] = chart
	if !pageV1Valid(t, marketPageV1, inst) {
		t.Fatal("chart with complete points is invalid")
	}
	delete(chart["points"].([]any)[0].(map[string]any), "ma200")
	if pageV1Valid(t, marketPageV1, inst) {
		t.Fatal("ChartPoint without ma200 validates")
	}
}
