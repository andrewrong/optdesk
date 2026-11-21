package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const heldFamilyPageV1 = "page.held_family_page.v1.json"

func heldFamilyFixture(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.held_family_page.v1.ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func heldRows(m map[string]any) []any { return m["rows"].([]any) }

// TestHeldFamilyPageSchema: the example payload (a value row, a negative-value row, a gap row and
// every earnings reason) validates; a negative days_to, an unknown key and six columns are refused.
func TestHeldFamilyPageSchema(t *testing.T) {
	t.Parallel()
	if !pageV1Valid(t, heldFamilyPageV1, heldFamilyFixture(t)) {
		t.Fatal("the example payload is invalid")
	}
	cases := map[string]func(m map[string]any){
		"negative days_to": func(m map[string]any) {
			heldRows(m)[0].(map[string]any)["earnings"].(map[string]any)["days_to"] = -1
		},
		"fractional days_to": func(m map[string]any) {
			heldRows(m)[0].(map[string]any)["earnings"].(map[string]any)["days_to"] = 1.5
		},
		"unknown page key": func(m map[string]any) { m["extra"] = "x" },
		"unknown row key":  func(m map[string]any) { heldRows(m)[0].(map[string]any)["name"] = "x" },
		"quantity_text":    func(m map[string]any) { heldRows(m)[0].(map[string]any)["quantity_text"] = "x" },
		"rows_share":       func(m map[string]any) { m["rows_share"] = "x" },
		"six columns": func(m map[string]any) {
			c := m["columns"].([]any)
			m["columns"] = append(c, c[0])
		},
		"four columns": func(m map[string]any) { m["columns"] = m["columns"].([]any)[:4] },
		"201 rows": func(m map[string]any) {
			r := heldRows(m)
			for len(r) < 201 {
				r = append(r, r[0])
			}
			m["rows"] = r
		},
		"empty text": func(m map[string]any) { heldRows(m)[0].(map[string]any)["company"] = "" },
		"unknown earnings reason": func(m map[string]any) {
			heldRows(m)[1].(map[string]any)["earnings"].(map[string]any)["reason"] = "bogus"
		},
		"bad cell kind": func(m map[string]any) {
			heldRows(m)[0].(map[string]any)["value"].(map[string]any)["kind"] = "date"
		},
		"numeric share": func(m map[string]any) { heldRows(m)[0].(map[string]any)["share"] = 12.1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := heldFamilyFixture(t)
			mutate(m)
			if pageV1Valid(t, heldFamilyPageV1, m) {
				t.Fatal("the payload validates")
			}
		})
	}
}
