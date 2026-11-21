package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMarketStateChartOptional(t *testing.T) {
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
	row0 := func(m map[string]any) map[string]any {
		return m["market"].(map[string]any)["rows"].([]any)[0].(map[string]any)
	}
	m := load()
	if _, ok := row0(m)["price_chart"]; !ok {
		t.Fatal("the ok fixture has no price_chart")
	}
	if _, ok := m["vol_gap"]; !ok {
		t.Fatal("the ok fixture has no vol_gap")
	}
	if !pageV1Valid(t, schema, m) {
		t.Error("the ok fixture is invalid")
	}
	bare := load()
	delete(row0(bare), "price_chart")
	delete(bare, "vol_gap")
	if !pageV1Valid(t, schema, bare) {
		t.Error("a payload without price_chart and vol_gap is invalid")
	}
	bad := load()
	pts := row0(bad)["price_chart"].(map[string]any)["points"].([]any)
	delete(pts[0].(map[string]any), "close_text")
	if pageV1Valid(t, schema, bad) {
		t.Error("a price_chart point without a text is accepted")
	}
	badMa := load()
	pts = row0(badMa)["price_chart"].(map[string]any)["points"].([]any)
	delete(pts[len(pts)-1].(map[string]any), "ma200_text")
	if pageV1Valid(t, schema, badMa) {
		t.Error("a point with ma200 and no ma200_text is accepted")
	}
}
