package contracts_test

import (
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func loadFlexImportV1(t *testing.T, name string) map[string]any {
	t.Helper()
	return readRaw(t, optionPageFixturesDir, "page.flex_import.v1."+name+".json")
}

// TestFlexImportV1Fixtures validates golden, result and disabled, shows bad-version fails at
// /schema_version, and rejects inline mutations.
func TestFlexImportV1Fixtures(t *testing.T) {
	t.Parallel()
	s := compilePageV2(t, "page.flex_import.v1.json")
	for _, name := range []string{"golden", "result", "disabled"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := s.Validate(loadFlexImportV1(t, name)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
	t.Run("bad-version", func(t *testing.T) {
		t.Parallel()
		err := s.Validate(loadFlexImportV1(t, "bad-version"))
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			t.Fatalf("err = %v, want a validation error", err)
		}
		var got []leaf
		leaves(ve, &got)
		if len(got) != 1 || got[0] != (leaf{"/schema_version", "const"}) {
			t.Errorf("failures = %v, want one /schema_version const", got)
		}
	})
	first := func(m map[string]any, key string) map[string]any {
		return m[key].([]any)[0].(map[string]any)
	}
	rejected := map[string]func(m map[string]any){
		"unknown-file-status":   func(m map[string]any) { first(m, "files")["status"] = "done" },
		"unknown-recent-status": func(m map[string]any) { first(m, "recent")["status"] = "done" },
		"unknown-reason":        func(m map[string]any) { first(m, "files")["reason"] = "disk_full" },
		"negative-count":        func(m map[string]any) { first(m, "files")["unmapped_statements"] = -1 },
		"negative-alias-count": func(m map[string]any) {
			first(first(m, "files"), "aliases")["rows_new"] = -1
		},
		"extra-top-level": func(m map[string]any) { m["extra"] = true },
	}
	for name, mutate := range rejected {
		t.Run("rejects-"+name, func(t *testing.T) {
			t.Parallel()
			m := loadFlexImportV1(t, "result")
			mutate(m)
			if err := s.Validate(m); err == nil {
				t.Error("page is valid, want an error")
			}
		})
	}
	t.Run("accepts-empty-reason", func(t *testing.T) {
		t.Parallel()
		m := loadFlexImportV1(t, "result")
		first(m, "files")["reason"] = ""
		if err := s.Validate(m); err != nil {
			t.Errorf("page is invalid: %v", err)
		}
	})
}
