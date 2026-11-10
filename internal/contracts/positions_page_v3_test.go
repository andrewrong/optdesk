package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	inst, err := jsonUnmarshalInstance(b)
	if err != nil {
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
		{"key over 64", withViews(t, 1, func(int) string { return repeat("x", 65) }), false},
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
	if !positionsValid(t, setRole("账户（main）")) {
		t.Error("role 账户（main） rejected")
	}
	if positionsValid(t, setRole("")) {
		t.Error("empty role text accepted")
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func jsonUnmarshalInstance(b []byte) (any, error) {
	var v any
	err := json.Unmarshal(b, &v)
	return v, err
}
