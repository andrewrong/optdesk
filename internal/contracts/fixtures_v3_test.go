package contracts_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const fixturesV3Dir = "../../web/fixtures/v3"

// fixtureSchemaV3 maps a directory of web/fixtures/v3 to its schema file.
var fixtureSchemaV3 = map[string]string{
	"market-summary":  "page.market_summary.v3.json",
	"options-summary": "page.options_summary.v3.json",
}

// anyObject reports whether pred holds for some object inside v.
func anyObject(v any, pred func(map[string]any) bool) bool {
	switch x := v.(type) {
	case map[string]any:
		if pred(x) {
			return true
		}
		for _, c := range x {
			if anyObject(c, pred) {
				return true
			}
		}
	case []any:
		for _, c := range x {
			if anyObject(c, pred) {
				return true
			}
		}
	}
	return false
}

func hasNull(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case map[string]any:
		for _, c := range x {
			if hasNull(c) {
				return true
			}
		}
	case []any:
		for _, c := range x {
			if hasNull(c) {
				return true
			}
		}
	}
	return false
}

func isGap(o map[string]any) bool       { return o["kind"] == "gap" }
func isStaleTrue(o map[string]any) bool { return o["is_stale"] == true }

// fixturesV3Problems applies the fixture rule to the directory root (the place of web/fixtures/v3), with the
// schema files of schemas. It returns one line per broken rule.
func fixturesV3Problems(t *testing.T, root, schemas string) []string {
	t.Helper()
	var problems []string
	bad := func(format string, a ...any) { problems = append(problems, fmt.Sprintf(format, a...)) }
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := ent.Name()
		schemaFile, ok := fixtureSchemaV3[dir]
		if !ok {
			bad("%s: directory has no row in the table", dir)
			continue
		}
		if _, err := os.Stat(filepath.Join(schemas, schemaFile)); err != nil {
			bad("%s: schema file %s is missing", dir, schemaFile)
			continue
		}
		s := compileURL(t, schemas, schemaHost+schemaFile)
		docs := map[string]any{}
		for _, name := range []string{"ok", "gap", "stale", "bad-version"} {
			path := filepath.Join(root, dir, name+".json")
			b, err := os.ReadFile(path)
			if err != nil {
				bad("%s/%s.json: %v", dir, name, err)
				continue
			}
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
			if err != nil {
				bad("%s/%s.json: %v", dir, name, err)
				continue
			}
			var plain any
			if err := json.Unmarshal(b, &plain); err != nil {
				bad("%s/%s.json: %v", dir, name, err)
				continue
			}
			docs[name] = plain
			verr := s.Validate(inst)
			if name != "bad-version" {
				if verr != nil {
					bad("%s/%s.json: want valid: %v", dir, name, verr)
				}
				if hasNull(plain) {
					bad("%s/%s.json: holds null", dir, name)
				}
				continue
			}
			ve, ok := verr.(*jsonschema.ValidationError)
			if !ok {
				bad("%s/bad-version.json: want a validation error, got %v", dir, verr)
				continue
			}
			var ls []leaf
			leaves(ve, &ls)
			found := false
			for _, l := range ls {
				found = found || (l.location == "/schema_version" && l.keyword == "const")
			}
			if !found {
				bad("%s/bad-version.json: no leaf at /schema_version with const; leaves %v", dir, ls)
			}
		}
		if d, ok := docs["ok"]; ok {
			if anyObject(d, isGap) {
				bad("%s/ok.json: holds an object with kind gap", dir)
			}
			if anyObject(d, isStaleTrue) {
				bad("%s/ok.json: holds is_stale true", dir)
			}
		}
		if d, ok := docs["gap"]; ok && !anyObject(d, isGap) {
			bad("%s/gap.json: holds no object with kind gap", dir)
		}
		if d, ok := docs["stale"]; ok && !anyObject(d, isStaleTrue) {
			bad("%s/stale.json: holds no is_stale true", dir)
		}
	}
	return problems
}

func TestFixturesV3Validate(t *testing.T) {
	t.Parallel()
	t.Run("repository", func(t *testing.T) {
		t.Parallel()
		if p := fixturesV3Problems(t, fixturesV3Dir, schemasDir); len(p) > 0 {
			t.Errorf("fixture rule broken:\n%s", strings.Join(p, "\n"))
		}
	})
	t.Run("no directory", func(t *testing.T) {
		t.Parallel()
		if p := fixturesV3Problems(t, filepath.Join(t.TempDir(), "absent"), schemasDir); len(p) > 0 {
			t.Errorf("want none, got %v", p)
		}
	})
	t.Run("gap.json without a gap object fails", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		dir := filepath.Join(root, "market-summary")
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"ok", "gap", "stale", "bad-version"} {
			src := name
			if name == "gap" {
				src = "ok" // the gap file carries no gap object
			}
			b, err := os.ReadFile(filepath.Join(fixturesV3Dir, "market-summary", src+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, name+".json"), b, 0o600); err != nil {
				t.Fatal(err)
			}
		}
		p := fixturesV3Problems(t, root, schemasDir)
		if len(p) != 1 || !strings.Contains(p[0], "gap.json: holds no object with kind gap") {
			t.Errorf("problems = %v", p)
		}
	})
	t.Run("directory without a row fails", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "unknown-page"), 0o750); err != nil {
			t.Fatal(err)
		}
		if p := fixturesV3Problems(t, root, schemasDir); len(p) != 1 {
			t.Errorf("problems = %v", p)
		}
	})
	t.Run("missing schema file fails", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, "market-summary"), 0o750); err != nil {
			t.Fatal(err)
		}
		if p := fixturesV3Problems(t, root, t.TempDir()); len(p) != 1 || !strings.Contains(p[0], "is missing") {
			t.Errorf("problems = %v", p)
		}
	})
}
