package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const structuresListSchema = "page.structures_list.v1.json"

// structuresListWithHref loads the ok fixture and sets the href of its first row.
func structuresListWithHref(t *testing.T, href string) any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fixturesDir, "structures-list", "ok.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	groups := doc["groups"].([]any)
	rows := groups[0].(map[string]any)["rows"].([]any)
	rows[0].(map[string]any)["href"] = href
	return doc
}

func TestStructuresListHref(t *testing.T) {
	t.Parallel()
	const root = "/"
	cases := []struct {
		name string
		href string
		ok   bool
	}{
		{"escaped id", root + "options/structures/a%3AT123%3A1", true},
		{"colon id", root + "options/structures/a:T123:1", true},
		{"old route", root + "structure/a1b2c3", true},
		{"empty id", root + "options/structures/", false},
		{"slash in id", root + "options/structures/a/b", false},
		{"colon in old route", root + "structure/a:b", false},
		{"scheme", "https://example.com/options/structures/a1", false},
	}
	s := compileCarried(t, structuresListSchema)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := s.Validate(structuresListWithHref(t, c.href))
			if c.ok {
				if err != nil {
					t.Fatalf("want valid: %v", err)
				}
				return
			}
			verr, isV := err.(*jsonschema.ValidationError)
			if !isV {
				t.Fatalf("want a validation error, got %v", err)
			}
			var ls []leaf
			leaves(verr, &ls)
			for _, l := range ls {
				if l.location == "/groups/0/rows/0/href" && l.keyword == "pattern" {
					return
				}
			}
			t.Fatalf("no pattern leaf at the href of the first row; leaves %v", ls)
		})
	}
}
