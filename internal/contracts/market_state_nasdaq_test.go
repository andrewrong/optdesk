package contracts_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"
)

const marketStatePageV2 = "page.market_state_page.v2.json"

func TestMarketStateNasdaqRow(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.market_state_page.v2.ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var inst map[string]any
	if err := json.Unmarshal(b, &inst); err != nil {
		t.Fatal(err)
	}
	if !pageV1Valid(t, marketStatePageV2, inst) {
		t.Fatal("the stored five-row fixture is invalid")
	}
	rows := msRows(inst)
	idx := -1
	for i, r := range rows {
		if r.(map[string]any)["id"] == "sp500_drawdown" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("fixture has no sp500_drawdown row")
	}
	src := rows[idx].(map[string]any)
	cp := make(map[string]any, len(src))
	maps.Copy(cp, src)
	cp["id"] = "nasdaq_drawdown"
	six := append([]any{}, rows[:idx+1]...)
	six = append(six, cp)
	six = append(six, rows[idx+1:]...)
	inst["market"].(map[string]any)["rows"] = six
	if !pageV1Valid(t, marketStatePageV2, inst) {
		t.Fatal("six-row payload with nasdaq_drawdown is invalid")
	}
	inst["market"].(map[string]any)["rows"] = append(six, cp)
	if pageV1Valid(t, marketStatePageV2, inst) {
		t.Fatal("seven rows validate")
	}
}
