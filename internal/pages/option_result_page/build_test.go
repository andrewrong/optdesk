package option_result_page_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages/option_result_page"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
	buildDay   = contracts.Day("2026-11-09")
)

type noSeries struct{ store.SeriesReader }

type fakeLedger struct {
	store.OptionLedgerReader
	rows []store.FlexTradeRow
	err  error
}

func (f fakeLedger) FlexTradeRows(_ context.Context, _ string) ([]store.FlexTradeRow, error) {
	return f.rows, f.err
}

type fakeSessions struct {
	store.SessionDayReader
	rows     []contracts.SessionDay
	cal      *string
	from, to *contracts.Day
}

func (f fakeSessions) SessionDays(_ context.Context, cal string, from, to contracts.Day) ([]contracts.SessionDay, error) {
	*f.cal, *f.from, *f.to = cal, from, to
	return f.rows, nil
}

type fakeRule struct {
	res compute.LedgerResult
	in  *compute.LedgerInput
}

func (f fakeRule) Replay(in compute.LedgerInput, _ compute.LedgerParams) compute.LedgerResult {
	*f.in = in
	return f.res
}

type fakeResults struct {
	sum compute.ResultSummary
	in  *compute.ResultInput
}

func (f fakeResults) Summarise(in compute.ResultInput) compute.ResultSummary {
	*f.in = in
	return f.sum
}

// probe records what the page gave to its readers and rules.
type probe struct {
	replay  compute.LedgerInput
	summary compute.ResultInput
	cal     string
	from    contracts.Day
	to      contracts.Day
}

type fixture struct {
	Replay  compute.LedgerResult
	Summary compute.ResultSummary
}

func loadFixture(t *testing.T, name string) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func zone8() *time.Location { return time.FixedZone("CST", 8*3600) }

func testConfig(t *testing.T, name string) (option_result_page.Config, *probe) {
	t.Helper()
	f := loadFixture(t, name)
	p := &probe{}
	return option_result_page.Config{
		RunID: "run1", BuildID: "b1", Zone: zone8(), Aliases: []string{"main", "second"},
		Ledger:   fakeLedger{rows: []store.FlexTradeRow{{TransactionID: "T1", Revision: 2, Attributes: `{"buySell":"SELL","quantity":"-1"}`}}},
		Sessions: fakeSessions{cal: &p.cal, from: &p.from, to: &p.to, rows: []contracts.SessionDay{{IsSession: true}}},
		Ledgers:  func() compute.OptionLedger { return fakeRule{res: f.Replay, in: &p.replay} },
		Results:  func() compute.OptionResults { return fakeResults{sum: f.Summary, in: &p.summary} },
	}, p
}

func buildAt() time.Time { return time.Date(2026, 11, 9, 0, 30, 0, 0, time.UTC) }

func buildMap(t *testing.T, c option_result_page.Config, alias string) map[string]any {
	t.Helper()
	b, err := option_result_page.NewPage(c, alias).Build(context.Background(), noSeries{}, buildDay, buildAt())
	if err != nil {
		t.Fatal(err)
	}
	validate(t, b)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func sectionsOf(m map[string]any) []map[string]any {
	var out []map[string]any
	for _, s := range m["sections"].([]any) {
		out = append(out, s.(map[string]any))
	}
	return out
}

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

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

func validate(t *testing.T, b []byte) {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	schema, err := c.Compile(schemaHost + "page.option_result_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(v); err != nil {
		t.Errorf("%v\n%s", err, b)
	}
}

func TestOutputValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"result_ok.json", "result_empty.json"} {
		c, _ := testConfig(t, name)
		b, err := option_result_page.NewPage(c, "main").Build(context.Background(), noSeries{}, buildDay, buildAt())
		if err != nil {
			t.Fatal(err)
		}
		validate(t, b)
	}
}

func TestBuildFeedsTheRules(t *testing.T) {
	t.Parallel()
	c, p := testConfig(t, "result_ok.json")
	m := buildMap(t, c, "second")
	if m["scope"] != "second" || m["generated_at"] != "2026-11-09T08:30:00+08:00" || m["run_status"] != "ok" {
		t.Fatalf("head = %v %v %v", m["scope"], m["generated_at"], m["run_status"])
	}
	if p.cal != "XNYS" || p.from != "2025-11-09" || p.to != buildDay {
		t.Errorf("sessions asked %s %s %s", p.cal, p.from, p.to)
	}
	tr := p.replay.Trades
	if p.replay.Alias != "second" || len(tr) != 1 || tr[0].TransactionID != "T1" || tr[0].Revision != 2 || tr[0].Attributes["buySell"] != "SELL" {
		t.Errorf("replay input = %+v", p.replay)
	}
	if len(p.replay.Marks) != 0 || p.replay.AsOf != buildDay {
		t.Errorf("replay input = %+v", p.replay)
	}
	if got := p.summary; got.Alias != "second" || len(got.Unresolved) != 1 || got.Unresolved[0] != "T9" || len(got.Review) != 2 {
		t.Errorf("summary input = %+v", got)
	}
}

func TestNothingWhenNoClosed(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_empty.json")
	m := buildMap(t, c, "main")
	if m["nothing"] != "没有已平仓的期权持仓" || len(m["sections"].([]any)) != 0 || len(m["reliability"].([]any)) != 0 || m["run_status"] != "ok" {
		t.Errorf("page = %v", m)
	}
	c, _ = testConfig(t, "result_ok.json")
	if buildMap(t, c, "main")["nothing"] != nil {
		t.Error("nothing is set with sections")
	}
}

func TestNoReaderIsGap(t *testing.T) {
	t.Parallel()
	base, _ := testConfig(t, "result_ok.json")
	cases := map[string]func(c *option_result_page.Config){
		"ledger":   func(c *option_result_page.Config) { c.Ledger = nil },
		"sessions": func(c *option_result_page.Config) { c.Sessions = nil },
		"ledgers":  func(c *option_result_page.Config) { c.Ledgers = nil },
		"results":  func(c *option_result_page.Config) { c.Results = nil },
	}
	for name, mut := range cases {
		c := base
		mut(&c)
		m := buildMap(t, c, "main")
		if m["run_status"] != "partial" || m["nothing"] != "没有读取器,无法显示" || len(m["sections"].([]any)) != 0 {
			t.Errorf("%s: page = %v", name, m)
		}
	}
}

func TestUnknownAliasIsError(t *testing.T) {
	t.Parallel()
	base, _ := testConfig(t, "result_ok.json")
	boom := errors.New("boom")
	cases := map[string]func(c *option_result_page.Config) string{
		"unknown alias": func(*option_result_page.Config) string { return "other" },
		"bad alias":     func(c *option_result_page.Config) string { c.Aliases = []string{"a b"}; return "a b" },
		"nil zone":      func(c *option_result_page.Config) string { c.Zone = nil; return "main" },
		"bad run id":    func(c *option_result_page.Config) string { c.RunID = "x y"; return "main" },
		"bad build id":  func(c *option_result_page.Config) string { c.BuildID = ""; return "main" },
		"reader error":  func(c *option_result_page.Config) string { c.Ledger = fakeLedger{err: boom}; return "main" },
		"bad row": func(c *option_result_page.Config) string {
			c.Ledger = fakeLedger{rows: []store.FlexTradeRow{{TransactionID: "T", Attributes: "{"}}}
			return "main"
		},
	}
	for name, mut := range cases {
		c := base
		alias := mut(&c)
		if _, err := option_result_page.NewPage(c, alias).Build(context.Background(), noSeries{}, buildDay, buildAt()); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestJPYSectionSeparate(t *testing.T) {
	t.Parallel()
	c, _ := testConfig(t, "result_ok.json")
	secs := sectionsOf(buildMap(t, c, "main"))
	keys := []string{}
	for _, s := range secs {
		keys = append(keys, s["key"].(string))
	}
	if strings.Join(keys, ",") != "sold_usd,bought_usd,sold_jpy" {
		t.Fatalf("keys = %v", keys)
	}
	jpy := secs[2]
	for _, f := range jpy["figures"].([]any) {
		v := f.(map[string]any)["value"].(string)
		if strings.Contains(v, "109.5") || strings.Contains(v, "5109.5") {
			t.Errorf("jpy figure %q holds a USD figure", v)
		}
	}
	if jpy["figures"].([]any)[0].(map[string]any)["value"] != "5000.0000" {
		t.Errorf("jpy figures = %v", jpy["figures"])
	}
}

// sources parses the non-test .go files of the package.
func sources(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*ast.File{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		out[name] = f
	}
	if len(out) == 0 {
		t.Fatal("no source files")
	}
	return out
}

func TestNoFloatNoSortNoNotify(t *testing.T) {
	t.Parallel()
	banned := map[string]bool{
		`"math/big"`: true, `"sort"`: true, `"os"`: true, `"net/http"`: true, `"database/sql"`: true,
		`"github.com/andrewrong/optdesk/internal/notify"`: true, `"github.com/andrewrong/optdesk/internal/jobs"`: true,
	}
	for name, f := range sources(t) {
		slicesName := ""
		for _, imp := range f.Imports {
			if banned[imp.Path.Value] {
				t.Errorf("%s imports %s", name, imp.Path.Value)
			}
			if imp.Path.Value == `"slices"` {
				slicesName = "slices"
				if imp.Name != nil {
					slicesName = imp.Name.Name
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if x.Name == "float32" || x.Name == "float64" {
					t.Errorf("%s uses %s", name, x.Name)
				}
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && slicesName != "" && id.Name == slicesName && strings.HasPrefix(x.Sel.Name, "Sort") {
					t.Errorf("%s calls slices.%s", name, x.Sel.Name)
				}
			}
			return true
		})
	}
}

func TestNoClockNoIO(t *testing.T) {
	t.Parallel()
	for name, f := range sources(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "time" && (sel.Sel.Name == "Now" || sel.Sel.Name == "Sleep") {
					t.Errorf("%s calls time.%s", name, sel.Sel.Name)
				}
			}
			return true
		})
	}
}
