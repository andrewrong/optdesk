package contracts_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compilePageSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "contracts", "schemas", "page.macro_rates.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("page.macro_rates.v1.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("page.macro_rates.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func loadGolden(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "contracts", "fixtures", "page.macro_rates.v1.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPageSchema(t *testing.T) {
	t.Parallel()
	s := compilePageSchema(t)

	t.Run("golden is valid", func(t *testing.T) {
		t.Parallel()
		if err := s.Validate(loadGolden(t)); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unknown top-level field fails", func(t *testing.T) {
		t.Parallel()
		m := loadGolden(t)
		m["extra"] = true
		if err := s.Validate(m); err == nil {
			t.Fatal("want a failure for an unknown field")
		}
	})
	t.Run("unknown nested field fails", func(t *testing.T) {
		t.Parallel()
		m := loadGolden(t)
		pt := m["series"].([]any)[0].(map[string]any)["points"].([]any)[0].(map[string]any)
		pt["extra"] = 1
		if err := s.Validate(m); err == nil {
			t.Fatal("want a failure for an unknown nested field")
		}
	})
	t.Run("bad generated_at fails", func(t *testing.T) {
		t.Parallel()
		m := loadGolden(t)
		m["generated_at"] = "yesterday"
		err := s.Validate(m)
		if err == nil || !strings.Contains(err.Error(), "generated_at") {
			t.Fatalf("want a generated_at failure, got %v", err)
		}
	})
	gapCase := func(name string, edit func(g map[string]any), wantOK bool) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m := loadGolden(t)
			edit(m["gaps"].([]any)[0].(map[string]any))
			err := s.Validate(m)
			if wantOK && err != nil {
				t.Fatal(err)
			}
			if !wantOK && err == nil {
				t.Fatal("want a failure")
			}
		})
	}
	gapCase("stale gap is valid", func(g map[string]any) { g["reason"] = "stale" }, true)
	gapCase("unknown gap reason fails", func(g map[string]any) { g["reason"] = "bogus" }, false)
	gapCase("gap without series_id fails", func(g map[string]any) { delete(g, "series_id") }, false)
	gapCase("gap without date fails", func(g map[string]any) { delete(g, "date") }, false)
}
