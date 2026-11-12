package flex_import

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func contractPath(parts ...string) string {
	return filepath.Join(append([]string{"..", "..", "..", "contracts"}, parts...)...)
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, name := range []string{"common.v2.json", "page.flex_import.v1.json"} {
		f, err := os.Open(contractPath("schemas", name))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.AddResource("https://optdesk.invalid/schemas/"+name, doc); err != nil {
			t.Fatal(err)
		}
	}
	s, err := c.Compile("https://optdesk.invalid/schemas/page.flex_import.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func validate(t *testing.T, out []byte) {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if err := compileSchema(t).Validate(doc); err != nil {
		t.Fatalf("schema: %v", err)
	}
}

// checkFixture builds in and compares it byte for byte with the named fixture (indented by two spaces, one newline).
func checkFixture(t *testing.T, in Input, name string) {
	t.Helper()
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, out, "", "  "); err != nil {
		t.Fatal(err)
	}
	buf.WriteByte('\n')
	want, err := os.ReadFile(contractPath("fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("output differs from %s:\n got %s\nwant %s", name, buf.Bytes(), want)
	}
	validate(t, out)
}

var testForm = &Form{Expires: "1900000000", MAC: "0123456789abcdef", MaxFiles: 8, MaxFileText: "64 MiB"}

func TestGoldenFixture(t *testing.T) {
	t.Parallel()
	checkFixture(t, Input{
		Form:   testForm,
		Recent: []Recent{{AtText: "2026-11-10 21:05", Alias: "alpha", Status: "present", RowsParsed: 12, RowsPresent: 12}},
	}, "page.flex_import.v1.golden.json")
}

func TestDisabledFixture(t *testing.T) {
	t.Parallel()
	checkFixture(t, Input{Gaps: []Gap{{Kind: "gap", Reason: "no_reader", Text: readOnlyGapText}}},
		"page.flex_import.v1.disabled.json")
}

func TestResultFixture(t *testing.T) {
	t.Parallel()
	checkFixture(t, Input{
		Form: testForm,
		Files: []File{
			{Name: "flex-a.xml", PeriodFirst: "2026-01-01", PeriodLast: "2026-06-30", Unmapped: 1, Aliases: []Alias{
				{Alias: "alpha", Statements: 1, RowsParsed: 12, RowsNew: 7, RowsPresent: 5},
				{Alias: "beta", Statements: 1, RowsParsed: 4, RowsNew: 4},
			}},
			{Name: "flex-b.xml.gz", PeriodFirst: "2026-07-01", PeriodLast: "2026-09-30", Aliases: []Alias{
				{Alias: "alpha", Statements: 1, RowsParsed: 12, RowsPresent: 12},
			}},
			{Name: "flex-c.xml", PeriodFirst: "2026-10-01", PeriodLast: "2026-10-31"},
			{Name: "flex-d.xml", PeriodFirst: "2026-10-01", PeriodLast: "2026-10-31", Unmapped: 2},
			{Name: "notes.xml", Reason: "not_report"},
		},
		Recent: []Recent{
			{AtText: "2026-11-12 09:30", Alias: "alpha", Status: "present", RowsParsed: 12, RowsPresent: 12},
			{AtText: "2026-11-12 09:30", Status: "failed", Reason: "not_report"},
			{AtText: "2026-11-12 09:29", Alias: "beta", Status: "stored", RowsParsed: 4, RowsNew: 4},
		},
	}, "page.flex_import.v1.result.json")
}

func fileStatus(t *testing.T, f File) string {
	t.Helper()
	out, err := Build(Input{Form: testForm, Files: []File{f}})
	if err != nil {
		t.Fatal(err)
	}
	var p struct{ Files []struct{ Status string } }
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	return p.Files[0].Status
}

func TestFileStatusOrder(t *testing.T) {
	t.Parallel()
	// Every file below also meets the conditions of all later statuses.
	both := []Alias{{Alias: "a", Statements: 1, RowsParsed: 3, RowsNew: 1, RowsPresent: 2}}
	cases := []struct {
		name string
		file File
		want string
	}{
		{"failed over stored", File{Reason: "store_error", Aliases: both, Unmapped: 1}, "failed"},
		{"stored over present", File{Aliases: both, Unmapped: 1}, "stored"},
		{"present over unconfigured", File{Aliases: []Alias{{Alias: "a", RowsParsed: 3, RowsPresent: 3}}, Unmapped: 1}, "present"},
		{"unconfigured over nothing", File{Unmapped: 1}, "unconfigured"},
		{"nothing found", File{}, "nothing_found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := fileStatus(t, c.file); got != c.want {
				t.Fatalf("status = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFileNameCut(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("é", 100)
	out, err := Build(Input{Form: testForm, Files: []File{{Name: "/tmp/dir/" + long + ".xml"}, {Name: "../x/flex.xml"}}})
	if err != nil {
		t.Fatal(err)
	}
	var p struct{ Files []struct{ Name string } }
	if err := json.Unmarshal(out, &p); err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCountInString(p.Files[0].Name); n != 80 || strings.Contains(p.Files[0].Name, "/") {
		t.Fatalf("name = %q (%d runes)", p.Files[0].Name, n)
	}
	if p.Files[1].Name != "flex.xml" {
		t.Fatalf("name = %q", p.Files[1].Name)
	}
	validate(t, out)
}

func TestSentences(t *testing.T) {
	t.Parallel()
	sentence := func(in Input) string {
		out, err := Build(in)
		if err != nil {
			t.Fatal(err)
		}
		validate(t, out)
		var p struct{ Sentence string }
		if err := json.Unmarshal(out, &p); err != nil {
			t.Fatal(err)
		}
		return p.Sentence
	}
	if got := sentence(Input{Form: testForm}); got != "Choose up to 8 Flex XML files; no earlier import is listed." {
		t.Fatal(got)
	}
	two := []Recent{{AtText: "t", Status: "present"}, {AtText: "t", Status: "stored"}}
	if got := sentence(Input{Form: testForm, Recent: two}); got != "Choose up to 8 Flex XML files; 2 earlier imports are listed below." {
		t.Fatal(got)
	}
	one := Input{Form: testForm, Files: []File{{Name: "a", Aliases: []Alias{{Alias: "x", RowsParsed: 1, RowsNew: 1}}}}}
	if got := sentence(one); got != "1 file read: 1 stored." {
		t.Fatal(got)
	}
	if got := out1(t, one); !strings.Contains(got, `"status_text":"Stored: 1 new row"`) {
		t.Fatal(got)
	}
}

func out1(t *testing.T, in Input) string {
	t.Helper()
	out, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestScopeNoteWords(t *testing.T) {
	t.Parallel()
	out := out1(t, Input{})
	for _, w := range []string{"EXECUTION", "option_ledger", "TradeQueries", "not imported"} {
		if !strings.Contains(out, w) {
			t.Fatalf("scope_note lacks %q", w)
		}
	}
}
