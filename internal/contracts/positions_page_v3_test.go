package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// positionsOK reads web/fixtures/positions-page/ok.json (read only) as a generic tree.
func positionsOK(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixturesDir, "positions-page", "ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// withViews returns a copy of ok.json whose views are n copies of the first
// view, keyed by alias[i].
func withViews(t *testing.T, n int, key func(i int) string) map[string]any {
	t.Helper()
	m := positionsOK(t)
	first := m["views"].([]any)[0].(map[string]any)
	out := make([]any, 0, n)
	for i := 0; i < n; i++ {
		v := map[string]any{}
		for k, x := range first {
			v[k] = x
		}
		v["key"] = key(i)
		out = append(out, v)
	}
	m["views"] = out
	return m
}

func positionsValid(t *testing.T, m map[string]any) bool {
	t.Helper()
	s := compileCarried(t, "page.positions_page.v3.json")
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var inst any
	if err := json.Unmarshal(b, &inst); err != nil {
		t.Fatal(err)
	}
	return s.Validate(inst) == nil
}

func TestPositionsPageV3Views(t *testing.T) {
	t.Parallel()
	aliases := []string{"main", "a2", "a3", "a4", "a5", "a6", "a7", "a8"}
	alias := func(i int) string { return aliases[i] }
	cases := []struct {
		name string
		m    map[string]any
		ok   bool
	}{
		{"ok fixture", positionsOK(t), true},
		{"one view keyed by alias", withViews(t, 1, alias), true},
		{"eight views", withViews(t, 8, alias), true},
		{"zero views", withViews(t, 0, alias), false},
		{"nine views", withViews(t, 9, func(i int) string { return "k" + string(rune('a'+i)) }), false},
		{"empty key", withViews(t, 1, func(int) string { return "" }), false},
		{"key over 64", withViews(t, 1, func(int) string { return strings.Repeat("x", 65) }), false},
	}
	for _, c := range cases {
		if got := positionsValid(t, c.m); got != c.ok {
			t.Errorf("%s: valid = %v, want %v", c.name, got, c.ok)
		}
	}
}

func TestPositionsPageV3RoleCell(t *testing.T) {
	t.Parallel()
	setRole := func(text string) map[string]any {
		m := positionsOK(t)
		for _, v := range m["views"].([]any) {
			for _, r := range v.(map[string]any)["options"].(map[string]any)["rows"].([]any) {
				r.(map[string]any)["role"] = map[string]any{"text": text, "value": text}
				return m
			}
		}
		t.Skip("ok.json has no option row")
		return nil
	}
	if !positionsValid(t, setRole("Account (main)")) {
		t.Error("role Account (main) rejected")
	}
	if positionsValid(t, setRole("")) {
		t.Error("empty role text accepted")
	}
}

// firstRow returns the first row of the stocks (table "stocks") or options table of the first view.
func firstRow(t *testing.T, m map[string]any, table string) map[string]any {
	t.Helper()
	v := m["views"].([]any)[0].(map[string]any)
	rows := v[table].(map[string]any)["rows"].([]any)
	if len(rows) == 0 {
		t.Fatalf("ok.json has no %s row", table)
	}
	return rows[0].(map[string]any)
}

func TestPositionsPageV3MoneyUnit(t *testing.T) {
	t.Parallel()
	withCost := func(unit string) map[string]any {
		m := positionsOK(t)
		firstRow(t, m, "stocks")["cost"] = map[string]any{
			"value": 12300.0, "text": "¥12,300.00", "unit": unit, "currency": "JPY", "tone": "none",
		}
		return m
	}
	if !positionsValid(t, withCost("amount_per_share")) {
		t.Error("JPY cost with unit amount_per_share rejected")
	}
	for _, unit := range []string{"usd_per_share", "usd", "usd_signed"} {
		if positionsValid(t, withCost(unit)) {
			t.Errorf("JPY cost with unit %s accepted", unit)
		}
	}
}

func TestPositionsPageV3Quantity(t *testing.T) {
	t.Parallel()
	set := func(table string, value float64, unit string) map[string]any {
		m := positionsOK(t)
		firstRow(t, m, table)["quantity"] = map[string]any{"value": value, "text": "0.5", "unit": unit}
		return m
	}
	cases := []struct {
		name  string
		m     map[string]any
		valid bool
	}{
		{"stock 0.5 shares", set("stocks", 0.5, "shares"), true},
		{"option 0.5 contracts", set("options", 0.5, "contracts"), false},
		{"option 2 contracts", set("options", 2, "contracts"), true},
		{"stock with unit contracts", set("stocks", 1, "contracts"), false},
		{"option with unit shares", set("options", 1, "shares"), false},
	}
	for _, c := range cases {
		if got := positionsValid(t, c.m); got != c.valid {
			t.Errorf("%s: valid = %v, want %v", c.name, got, c.valid)
		}
	}
}
