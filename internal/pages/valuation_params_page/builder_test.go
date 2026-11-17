package valuation_params_page

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func validate(t *testing.T, b []byte) map[string]any {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + "page.valuation_params_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(inst); err != nil {
		t.Fatalf("schema: %v\n%s", err, b)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// fakeCalc: ProposeR answers `proposed`; RStepWarnings answers `warnings` when the two differ; RoundText marks
// every figure by adding ".00" to a whole number.
type fakeCalc struct {
	proposed valuelinesapi.RValues
	warnings []string
	yields   []string
}

func (f *fakeCalc) Lines(valuelinesapi.Input) (valuelinesapi.Lines, error) {
	return valuelinesapi.Lines{}, nil
}
func (f *fakeCalc) Band(valuelinesapi.Lines, string) valuelinesapi.Band { return valuelinesapi.Band{} }
func (f *fakeCalc) ChangePercent(string, string) (string, error)        { return "0", nil }
func (f *fakeCalc) ProposeR(_ []valuelinesapi.RRow, y string) (valuelinesapi.RValues, error) {
	f.yields = append(f.yields, y)
	return f.proposed, nil
}
func (f *fakeCalc) RStepWarnings(prev, next valuelinesapi.RValues) []string {
	if prev == next {
		return nil
	}
	return f.warnings
}
func (f *fakeCalc) Check(valuelinesapi.CheckInput) []valuelinesapi.Problem { return nil }
func (f *fakeCalc) RoundText(v string, _ int) string {
	if !strings.Contains(v, ".") {
		return v + ".00"
	}
	return v
}

func baseInput(c *fakeCalc) Input {
	if c.proposed == (valuelinesapi.RValues{}) {
		c.proposed = valuelinesapi.RValues{US: "0.90", JP: "1.10", HKCN: "0.80"}
	}
	return Input{
		RunID: "run-1", Now: time.Date(2026, 11, 17, 9, 0, 0, 0, time.UTC), StoreOK: true, Calc: c,
		Versions: []valuation.RVersion{
			{EffectiveDate: "2026-10-01", YieldUsed: "4.0", RUS: "0.90", RJP: "1.10", RHKCN: "0.80", AISwitch: true, AIExpiry: "invented"},
			{EffectiveDate: "2026-09-01", YieldUsed: "3.8", RUS: "0.85", RJP: "1.10", RHKCN: "0.80"},
		},
		Yield: &Yield{Value: "4.2", Day: "2026-11-14"},
		RTable: &valuation.RTableVersion{EffectiveDate: "2026-08-01", Rows: []valuation.RTableRow{
			{YieldUpTo: "4.5", US: "0.95", JP: "1.10", HKCN: "0.80"}, {US: "1.00", JP: "1.15", HKCN: "0.85"},
		}},
		Margin: &valuation.MarginTableVersion{EffectiveDate: "2026-08-01", Rows: []valuation.MarginTableRow{
			{PFrom: "3", M: "0.18", W: "0.07", H: "0.30", S: "0.22"},
		}},
	}
}

func TestProposalShownWhenDifferent(t *testing.T) {
	t.Parallel()
	c := &fakeCalc{proposed: valuelinesapi.RValues{US: "0.95", JP: "1.10", HKCN: "0.80"}, warnings: []string{"US R moves"}}
	b, err := Build(baseInput(c))
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	p, ok := m["proposal"].(map[string]any)
	if !ok || p["r"].(map[string]any)["us"] != "0.95" {
		t.Fatalf("proposal = %v", m["proposal"])
	}
	if len(c.yields) != 1 || c.yields[0] != "4.2" {
		t.Errorf("ProposeR yields = %v", c.yields)
	}
	if cur := m["current_r"].(map[string]any); cur["r"].(map[string]any)["us"] != "0.90" || len(m["r_history"].([]any)) != 2 {
		t.Errorf("current_r = %v", cur)
	}
	// the same R as the newest version: no proposal
	c2 := &fakeCalc{proposed: valuelinesapi.RValues{US: "0.90", JP: "1.10", HKCN: "0.80"}}
	b, err = Build(baseInput(c2))
	if err != nil {
		t.Fatal(err)
	}
	if got := validate(t, b)["proposal"]; got != nil {
		t.Errorf("proposal = %v, want null", got)
	}
}

func TestStepWarningsInDocument(t *testing.T) {
	t.Parallel()
	c := &fakeCalc{proposed: valuelinesapi.RValues{US: "1.00", JP: "1.10", HKCN: "0.80"}, warnings: []string{"US R moves from 0.90 to 1.00, more than 0.05"}}
	b, err := Build(baseInput(c))
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	w := m["proposal"].(map[string]any)["step_warnings"].([]any)
	if len(w) != 1 || w[0] != c.warnings[0] {
		t.Errorf("proposal warnings = %v", w)
	}
	// the saved version 2026-10-01 moved US from 0.85 to 0.90: the history row carries the calculator's warning too
	h := m["r_history"].([]any)
	if got := h[0].(map[string]any)["step_warnings"].([]any); len(got) != 1 {
		t.Errorf("history warnings = %v", got)
	}
	if got := h[1].(map[string]any)["step_warnings"].([]any); len(got) != 0 {
		t.Errorf("oldest version has no previous: %v", got)
	}
}

func TestNoTablesGivesGap(t *testing.T) {
	t.Parallel()
	in := baseInput(&fakeCalc{})
	in.RTable, in.Margin = nil, nil
	b, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	reasons := map[string]bool{}
	for _, g := range m["gaps"].([]any) {
		reasons[g.(map[string]any)["reason"].(string)] = true
	}
	if !reasons["no_r_table"] || !reasons["no_margin_table"] || m["proposal"] != nil || m["r_table"] != nil || m["current_r"] == nil {
		t.Errorf("doc = %v", m)
	}
}

func TestStoreUnavailableAndNoYieldAreGaps(t *testing.T) {
	t.Parallel()
	in := baseInput(&fakeCalc{})
	in.StoreOK = false
	b, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if m := validate(t, b); len(m["gaps"].([]any)) != 1 || m["current_r"] != nil {
		t.Errorf("doc = %v", m)
	}
	in = baseInput(&fakeCalc{})
	in.Yield = nil
	b, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if m := validate(t, b); m["latest_yield"] != nil || m["proposal"] != nil || len(m["gaps"].([]any)) != 1 {
		t.Errorf("doc = %v", m)
	}
}

func TestFormNullWithoutOwnerEdit(t *testing.T) {
	t.Parallel()
	in := baseInput(&fakeCalc{})
	b, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	if got := validate(t, b)["form"]; got != nil {
		t.Errorf("form = %v, want null", got)
	}
	in.Form = &Form{Expires: "1789000000", MAC: strings.Repeat("a", 64)}
	b, err = Build(in)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := validate(t, b)["form"].(map[string]any)
	if !ok || f["post_path"] != "/valuation-params-post" {
		t.Errorf("form = %v", f)
	}
}

// TestSourceStaysPure: the package imports no database, file, network, store implementation or clock package, and
// no decimal or math package; it never parses a float.
func TestSourceStaysPure(t *testing.T) {
	t.Parallel()
	forbidden := []string{"database/sql", "os", "net/http", "internal/clock", "internal/store/valuationpg", "github.com/jackc/pgx", "internal/compute/valuelines"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		checked++
		af, err := parser.ParseFile(token.NewFileSet(), f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, im := range af.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			if p == "math" || strings.Contains(p, "decimal") {
				t.Errorf("%s imports %s", f, p)
			}
			for _, bad := range forbidden {
				if p == bad || strings.HasPrefix(p, bad+"/") || strings.HasSuffix(p, "/"+bad) {
					t.Errorf("%s imports %s", f, p)
				}
			}
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "strconv" && strings.HasPrefix(sel.Sel.Name, "ParseFloat") {
					t.Errorf("%s calls strconv.%s", f, sel.Sel.Name)
				}
			}
			return true
		})
	}
	if checked == 0 {
		t.Fatal("no source file checked")
	}
}
