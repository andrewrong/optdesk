package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const spreadLegsPageV1 = "page.spread_legs_page.v1.json"

func spreadLegsFixture(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.spread_legs_page.v1.ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func spreadStructure(m map[string]any, group, i int) map[string]any {
	g := m["groups"].([]any)[group].(map[string]any)
	return g["structures"].([]any)[i].(map[string]any)
}

// TestSpreadLegsPageSchema: the example payload (a put spread, an iron condor with two break-evens, a
// covered call with gap cells, an unidentified structure) validates; a third group, a ninth leg and an
// unknown key are refused.
func TestSpreadLegsPageSchema(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, spreadLegsPageV1, spreadLegsFixture(t)) {
		t.Fatal("the example payload is invalid")
	}
	cases := map[string]func(m map[string]any){
		"third group": func(m map[string]any) {
			g := m["groups"].([]any)
			m["groups"] = append(g, g[0])
		},
		"one group": func(m map[string]any) { m["groups"] = m["groups"].([]any)[:1] },
		"ninth leg": func(m map[string]any) {
			s := spreadStructure(m, 0, 1)
			l := s["legs"].([]any)
			for len(l) < 9 {
				l = append(l, l[0])
			}
			s["legs"] = l
		},
		"no leg":           func(m map[string]any) { spreadStructure(m, 0, 0)["legs"] = []any{} },
		"unknown page key": func(m map[string]any) { m["extra"] = "x" },
		"unknown structure key": func(m map[string]any) {
			spreadStructure(m, 0, 0)["name"] = "x"
		},
		"unknown leg key": func(m map[string]any) {
			spreadStructure(m, 0, 0)["legs"].([]any)[0].(map[string]any)["x"] = "x"
		},
		"201 structures": func(m map[string]any) {
			g := m["groups"].([]any)[0].(map[string]any)
			r := g["structures"].([]any)
			for len(r) < 201 {
				r = append(r, r[0])
			}
			g["structures"] = r
		},
		"201 character text": func(m map[string]any) {
			b := make([]byte, 201)
			for i := range b {
				b[i] = 'a'
			}
			spreadStructure(m, 0, 0)["kind_gloss"] = string(b)
		},
		"empty text": func(m map[string]any) { spreadStructure(m, 0, 0)["symbol"] = "" },
		"gap cell without reason": func(m map[string]any) {
			delete(spreadStructure(m, 0, 2)["max_loss"].(map[string]any), "reason")
		},
		"gap cell with value": func(m map[string]any) {
			spreadStructure(m, 0, 2)["max_loss"].(map[string]any)["value"] = 0
		},
		"value cell without unit": func(m map[string]any) {
			delete(spreadStructure(m, 0, 0)["net_credit"].(map[string]any), "unit")
		},
		"break-even values empty": func(m map[string]any) {
			spreadStructure(m, 0, 0)["break_even"].(map[string]any)["values"] = []any{}
		},
		"break-even gap with values": func(m map[string]any) {
			spreadStructure(m, 0, 2)["break_even"].(map[string]any)["values"] = []any{1.0}
		},
		"bad href": func(m map[string]any) { spreadStructure(m, 0, 0)["href"] = "/other" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := spreadLegsFixture(t)
			mutate(m)
			if pageV1Valid(t, spreadLegsPageV1, m) {
				t.Fatal("the payload was accepted")
			}
		})
	}
}
