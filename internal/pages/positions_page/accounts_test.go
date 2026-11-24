package positions_page_test

import "testing"

func TestAccountLinksListEveryAlias(t *testing.T) {
	t.Parallel()
	c := config(account())
	c.Aliases = []string{"main", "wife", "daughter"}
	m, b := build(t, c, "wife")
	validate(t, b)
	wantKeys := []string{"main", "wife", "daughter"}
	links, _ := m["accounts"].([]any)
	if len(links) != len(wantKeys) {
		t.Fatalf("accounts = %v", m["accounts"])
	}
	for i, k := range wantKeys {
		l := links[i].(map[string]any)
		if l["key"] != k || l["href"] != "/positions?account="+k || l["current"] != (k == "wife") {
			t.Fatalf("link %d = %v", i, l)
		}
		if l["label"] != at0(t, m, "views", 0, "label") && k == "wife" {
			t.Fatalf("label of wife = %v, view label = %v", l["label"], at0(t, m, "views", 0, "label"))
		}
	}

	// a gap page still carries the links
	g, gb := build(t, c, "nobody")
	validate(t, gb)
	if gl, _ := g["accounts"].([]any); len(gl) != 3 || gl[0].(map[string]any)["current"] != false {
		t.Fatalf("gap page accounts = %v", g["accounts"])
	}

	c.Aliases = []string{"main"}
	one, _ := build(t, c, "main")
	if _, ok := one["accounts"]; ok {
		t.Fatalf("one alias carries accounts: %v", one["accounts"])
	}
}
