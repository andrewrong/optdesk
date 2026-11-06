package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `{
  "title": "Thing",
  "description": "demo.thing r1: a thing.",
  "type": "object",
  "additionalProperties": false,
  "required": ["id", "amount", "kind"],
  "x-dataset": "demo.thing",
  "x-revision": "r1",
  "x-primary-key": ["id"],
  "properties": {
    "id": {"type": "string", "description": "Identifier."},
    "amount": {"type": ["number", "null"], "description": "Amount."},
    "kind": {"$ref": "#/$defs/kind"}
  },
  "$defs": {"kind": {"type": "string", "enum": ["a", "b"]}}
}`

func TestRenderPage(t *testing.T) {
	t.Parallel()
	got, err := render([]byte(sample))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# demo.thing r1",
		"DO NOT EDIT",
		"| id | string | no | Identifier. |",
		"| amount | number | yes | Amount. |",
		"| kind | enum(a, b) | no |",
		"Primary key: (id)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("page lacks %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "| id |") > strings.Index(got, "| amount |") {
		t.Error("required order not kept")
	}
}

func TestRenderRejectsMissingDataset(t *testing.T) {
	t.Parallel()
	if _, err := render([]byte(`{"title":"x"}`)); err == nil {
		t.Fatal("want an error for a schema without x-dataset")
	}
}

func TestRunWritesOnePagePerSchema(t *testing.T) {
	t.Parallel()
	in, out := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(in, "demo.thing.r1.json"), []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(in, out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "demo.thing.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "# demo.thing r1") {
		t.Errorf("unexpected page: %s", b)
	}
}
