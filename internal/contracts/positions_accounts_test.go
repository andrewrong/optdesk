package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPositionsAccountsOptional(t *testing.T) {
	t.Parallel()
	const schema = "page.positions_page.v4.json"
	load := func() map[string]any {
		b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.positions_page.v4.ok.json"))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	link := func(key, href string, current bool) map[string]any {
		return map[string]any{"key": key, "label": key, "href": href, "current": current}
	}
	two := func() []any {
		return []any{link("main", "/positions", true), link("wife", "/positions?account=wife", false)}
	}
	m := load()
	delete(m, "accounts")
	if !pageV1Valid(t, schema, m) {
		t.Error("the ok fixture without accounts is invalid")
	}
	m = load()
	m["accounts"] = two()
	if !pageV1Valid(t, schema, m) {
		t.Error("two account links are invalid")
	}
	m = load()
	m["accounts"] = two()[:1]
	if pageV1Valid(t, schema, m) {
		t.Error("one account link is accepted")
	}
	m = load()
	m["accounts"] = []any{link("main", "/positions", true), link("wife", "/other", false)}
	if pageV1Valid(t, schema, m) {
		t.Error("an href outside /positions is accepted")
	}
}
