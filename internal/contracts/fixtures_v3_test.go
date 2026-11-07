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

// checkV3File validates one file of a directory: the three good ones are valid and hold no null, the bad one
// fails at /schema_version with const. It returns the problems and the parsed document.
func checkV3File(t *testing.T, s *jsonschema.Schema, path, label string) (any, []string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", label, err)}
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(b)))
	if err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", label, err)}
	}
	var plain any
	if err := json.Unmarshal(b, &plain); err != nil {
		return nil, []string{fmt.Sprintf("%s: %v", label, err)}
	}
	verr := s.Validate(inst)
	if !strings.HasSuffix(label, "bad-version.json") {
		var out []string
		if verr != nil {
			out = append(out, fmt.Sprintf("%s: want valid: %v", label, verr))
		}
		if hasNull(plain) {
			out = append(out, label+": holds null")
		}
		return plain, out
	}
	ve, ok := verr.(*jsonschema.ValidationError)
	if !ok {
		return plain, []string{fmt.Sprintf("%s: want a validation error, got %v", label, verr)}
	}
	var ls []leaf
	leaves(ve, &ls)
	for _, l := range ls {
		if l.location == "/schema_version" && l.keyword == "const" {
			return plain, nil
		}
	}
	return plain, []string{fmt.Sprintf("%s: no leaf at /schema_version with const; leaves %v", label, ls)}
}

// checkV3Tags holds the tag rules of the three good files of one directory.
func checkV3Tags(dir string, docs map[string]any) []string {
	var out []string
	if d, ok := docs["ok"]; ok {
		if anyObject(d, isGap) {
			out = append(out, dir+"/ok.json: holds an object with kind gap")
		}
		if anyObject(d, isStaleTrue) {
			out = append(out, dir+"/ok.json: holds is_stale true")
		}
	}
	if d, ok := docs["gap"]; ok && !anyObject(d, isGap) {
		out = append(out, dir+"/gap.json: holds no object with kind gap")
	}
	if d, ok := docs["stale"]; ok && !anyObject(d, isStaleTrue) {
		out = append(out, dir+"/stale.json: holds no is_stale true")
	}
	return out
}

// fixturesV3Problems applies the fixture rule to the directory root (the place of web/fixtures/v3), with the
// schema files of schemas. It returns one line per broken rule.
func fixturesV3Problems(t *testing.T, root, schemas string) []string {
	t.Helper()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var problems []string
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		dir := ent.Name()
		schemaFile, ok := fixtureSchemaV3[dir]
		if !ok {
			problems = append(problems, dir+": directory has no row in the table")
			continue
		}
		if _, err := os.Stat(filepath.Join(schemas, schemaFile)); err != nil {
			problems = append(problems, fmt.Sprintf("%s: schema file %s is missing", dir, schemaFile))
			continue
		}
		s := compileURL(t, schemas, schemaHost+schemaFile)
		docs := map[string]any{}
		for _, name := range []string{"ok", "gap", "stale", "bad-version"} {
			doc, p := checkV3File(t, s, filepath.Join(root, dir, name+".json"), dir+"/"+name+".json")
			problems = append(problems, p...)
			if doc != nil {
				docs[name] = doc
			}
		}
		problems = append(problems, checkV3Tags(dir, docs)...)
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
