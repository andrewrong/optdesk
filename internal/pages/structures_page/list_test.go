package structures_page

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/atombender/go-jsonschema/pkg/types"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/dividend"
)

func groupRows(t *testing.T, m map[string]any, i int) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, r := range m["groups"].([]any)[i].(map[string]any)["rows"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

func symbols(rows []map[string]any) string {
	var s []string
	for _, r := range rows {
		s = append(s, r["symbol"].(string))
	}
	return strings.Join(s, ",")
}

func TestOrderKept(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	if got := symbols(groupRows(t, m, 1)); got != "MSFT,NVDA,AMD" {
		t.Fatalf("others = %s, want the Evaluate order then Other", got)
	}
}

func TestDecisionGroupFirst(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	groups := m["groups"].([]any)
	if len(groups) != 2 || groups[0].(map[string]any)["title"] != "To decide this week" {
		t.Fatalf("groups = %v", groups)
	}
	if got := symbols(groupRows(t, m, 0)); got != "AAPL" {
		t.Fatalf("decide rows = %s", got)
	}
}

func TestRowFieldsFromFigures(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	r := groupRows(t, m, 0)[0]
	if r["kind_text"] != "Short put" || r["expiry_text"] != "2026-12-18" || r["distance"] != "9.1200%" || r["earned"] != "52.0000%" {
		t.Fatalf("row = %v", r)
	}
	if r["moneyness"] != "out of the money" {
		t.Fatalf("moneyness = %v", r["moneyness"])
	}
	loss := r["max_loss"].(map[string]any)
	if loss["value"] != -14750.0 || loss["text"] != "−$14,750.00" || loss["unit"] != "usd" {
		t.Fatalf("max_loss = %v", loss)
	}
	if m["scope"] != "main" || m["generated_at"] != "2026-11-09T09:00:00+08:00" {
		t.Fatalf("scope = %v, generated_at = %v", m["scope"], m["generated_at"])
	}
}

func TestRowMoneyTextsHaveTwoDecimals(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	r := groupRows(t, m, 0)[0]
	for key, want := range map[string]struct {
		value float64
		text  string
	}{"max_gain": {250, "$250.00"}, "pnl": {130, "$130.00"}} {
		c := r[key].(map[string]any)
		if c["value"] != want.value || c["text"] != want.text {
			t.Fatalf("%s = %v, want value %v text %s", key, c, want.value, want.text)
		}
	}
}

func TestCompareParamsFromStructureParams(t *testing.T) {
	t.Parallel()
	res := readResult(t, "list_ok")
	res.Structures[0].CaptureParam, res.Structures[0].ExitDTEParam = "0.60", "14"
	r := newRig(res)
	m := decode(t, r.build(t, "main"))
	if r.params.CaptureTarget != "0.50" || r.params.ExitDTE != 21 || r.params.CommissionDays != 30 {
		t.Fatalf("params passed = %+v", r.params)
	}
	cmp := groupRows(t, m, 0)[0]["compare"].(map[string]any)
	if cmp["captured_param"] != "Earned ratio 60%" || cmp["exit_dte_param"] != "Days before expiry 14" || cmp["close_cost"] != "120.0000" {
		t.Fatalf("compare = %v", cmp)
	}
}

func TestOtherRowsListed(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	rows := groupRows(t, m, 1)
	last := rows[len(rows)-1]
	if last["symbol"] != "AMD" || last["kind_text"] != "Unidentified" || last["compare"] != nil {
		t.Fatalf("other row = %v", last)
	}
	if !strings.Contains(m["note"].(string), "Unidentified structures, shown leg by leg") {
		t.Fatalf("note = %v", m["note"])
	}
}

func TestGapReasonsPassed(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	aapl := groupRows(t, m, 0)[0]["compare"].(map[string]any)
	if g := aapl["roll_net"].(map[string]any); g["reason"] != "missing_quote" {
		t.Fatalf("roll_net = %v", g)
	}
	nvda := groupRows(t, m, 1)[1]
	if g := nvda["max_loss"].(map[string]any); g["reason"] != "no_rule" {
		t.Fatalf("max_loss = %v", g)
	}
	if g := groupRows(t, m, 1)[0]["earned"]; g != "20.0000%" {
		t.Fatalf("earned = %v", g)
	}
}

func TestJPYRowIsGap(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_gaps")).build(t, "main"))
	r := groupRows(t, m, 1)[0]
	for _, k := range []string{"max_loss", "max_gain", "pnl", "distance", "earned", "moneyness"} {
		if g, ok := r[k].(map[string]any); !ok || g["reason"] != "unsupported_currency" {
			t.Fatalf("%s = %v", k, r[k])
		}
	}
	if m["run_status"] != "partial" {
		t.Fatalf("run_status = %v", m["run_status"])
	}
}

func TestNoReaderIsGap(t *testing.T) {
	t.Parallel()
	r := newRig(readResult(t, "list_ok"))
	r.cfg.Ledger = nil
	m := decode(t, r.build(t, "main"))
	if m["run_status"] != "failed" || !strings.Contains(m["note"].(string), "no_reader") || len(groupRows(t, m, 0))+len(groupRows(t, m, 1)) != 0 {
		t.Fatalf("page = %v", m)
	}
	// no quote reader: a missing quote is the gap no_reader
	q := newRig(readResult(t, "list_gaps"))
	q.cfg.Eod = nil
	row := groupRows(t, decode(t, q.build(t, "main")), 1)[1]
	if g := row["pnl"].(map[string]any); g["reason"] != "no_reader" {
		t.Fatalf("pnl = %v", g)
	}
}

func TestUnknownAliasIsError(t *testing.T) {
	t.Parallel()
	r := newRig(compute.StructureResult{})
	if _, err := buildList(context.Background(), r.cfg, "nobody", testDay, testAt); err == nil {
		t.Fatal("unknown alias gave no error")
	}
	r.cfg.Zone = nil
	if _, err := buildList(context.Background(), r.cfg, "main", testDay, testAt); err == nil {
		t.Fatal("nil zone gave no error")
	}
}

func TestMainAliasDefault(t *testing.T) {
	t.Parallel()
	r := newRig(compute.StructureResult{})
	m := decode(t, r.build(t, ""))
	if m["scope"] != "main" || r.in.Alias != "main" {
		t.Fatalf("scope = %v, alias = %q", m["scope"], r.in.Alias)
	}
}

func TestCaptureNoteShown(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	if n := m["note"].(string); !strings.Contains(n, "Positions not yet in Flex: TSLA 260116P00200000") {
		t.Fatalf("note = %q", n)
	}
	m = decode(t, newRig(readResult(t, "list_gaps")).build(t, "main"))
	if n := m["note"].(string); !strings.Contains(n, "No position snapshot") {
		t.Fatalf("note = %q", n)
	}
}

func TestHrefIsStructureRoute(t *testing.T) {
	t.Parallel()
	res := compute.StructureResult{Structures: []compute.Structure{{ID: "a:T123:1", Underlying: "X", Kind: "long_put", Currency: "USD",
		Legs: []compute.StructureLeg{{Right: "P", Expiry: "2026-12-18"}}}}}
	r := newRig(res)
	r.cfg.Aliases = []string{"a"}
	m := decode(t, r.build(t, "a"))
	want := "/options/structures/" + url.PathEscape("a:T123:1")
	if got := groupRows(t, m, 1)[0]["href"]; got != want {
		t.Fatalf("href = %v, want %s", got, want)
	}
}

// schemaOf compiles the carried schema as it is.
func schemaOf(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw, err := os.ReadFile("../../../contracts/schemas/page.structures_list.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile("schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOutputValidatesAgainstSchema(t *testing.T) {
	t.Parallel()
	schema := schemaOf(t)
	for name, res := range map[string]compute.StructureResult{
		"rows": readResult(t, "list_ok"), "gaps": readResult(t, "list_gaps"), "nothing": {},
	} {
		b := newRig(res).build(t, "main")
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v\n%s", name, err, b)
		}
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

func TestUnidentifiedHasReason(t *testing.T) {
	t.Parallel()
	leg := func(right, expiry, qty string) compute.StructureLeg {
		return compute.StructureLeg{Right: right, Expiry: expiry, Quantity: qty}
	}
	cases := []struct {
		name string
		legs []compute.StructureLeg
		want string
	}{
		{"opposite sides at different expiries", []compute.StructureLeg{leg("P", "2026-12-18", "-1"), leg("P", "2027-01-15", "1")}, reasonOppositeSidesExpiries},
		{"put and call at one expiry", []compute.StructureLeg{leg("P", "2026-12-18", "-1"), leg("C", "2026-12-18", "-1")}, reasonPutCallOneExpiry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := compute.StructureResult{Other: []compute.Structure{{ID: "main:T9:1", Underlying: "AMD", Kind: "other", Legs: tc.legs}}}
			m := decode(t, newRig(res).build(t, "main"))
			note := m["note"].(string)
			if !strings.Contains(note, "AMD: "+tc.want) {
				t.Fatalf("note = %q, want reason %q", note, tc.want)
			}
		})
	}
	one := compute.Structure{ID: "x", Underlying: "AMD", Kind: "other", Legs: []compute.StructureLeg{leg("P", "2026-12-18", "-1")}}
	single := compute.StructureResult{Other: []compute.Structure{one}}
	if n := decode(t, newRig(single).build(t, "main"))["note"].(string); strings.Contains(n, "AMD:") {
		t.Fatalf("single-leg note = %q", n)
	}
}

func TestOtherReasonIsChinese(t *testing.T) {
	t.Parallel()
	legs := []compute.StructureLeg{{Right: "P", Expiry: "2026-12-18", Quantity: "-1"}, {Right: "C", Expiry: "2026-12-18", Quantity: "-1"}}
	r := otherReason(legs)
	for _, c := range r {
		if unicode.Is(unicode.Han, c) {
			return
		}
	}
	t.Fatalf("reason %q has no Chinese text", r)
}

func TestShortPutMoneynessMatchesSign(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(readResult(t, "list_ok")).build(t, "main"))
	r := groupRows(t, m, 0)[0]
	if r["kind_text"] != "Short put" || r["distance"] != "9.1200%" || r["moneyness"] != "out of the money" {
		t.Fatalf("row = %v", r)
	}
}

// aaplOnly is the AAPL structure of list_ok alone: no other unit, no capture difference, no gap cell.
func aaplOnly(t *testing.T) compute.StructureResult {
	t.Helper()
	res := readResult(t, "list_ok")
	var keep []compute.Structure
	for _, s := range res.Structures {
		if s.Underlying == "AAPL" {
			s.Figures.RollNet = compute.FigureValue{Value: "-10.0000"}
			keep = append(keep, s)
		}
	}
	return compute.StructureResult{Structures: keep}
}

func TestNoteNamesCloseDays(t *testing.T) {
	t.Parallel()
	r := printsRig()
	r.cfg.Structures = newRig(aaplOnly(t)).cfg.Structures
	r.cfg.Bars = stubBars{rows: []contracts.PriceBar{barOn(6, "94.1")}}
	m := decode(t, r.build(t, "main"))
	if n, _ := m["note"].(string); n != "距现价所用价格：AAPL 收盘 2026-11-06" {
		t.Fatalf("bar note = %q", n)
	}
	r.cfg.Bars = nil
	m = decode(t, r.build(t, "main"))
	if n, _ := m["note"].(string); n != "距现价所用价格：AAPL 最近成交 2026-11-06" {
		t.Fatalf("print note = %q", n)
	}
	// no close: no line
	r = newRig(aaplOnly(t))
	if m = decode(t, r.build(t, "main")); m["note"] != nil {
		t.Fatalf("note without a close = %v", m["note"])
	}
}

func TestFullPageWithCloseNoteStaysOk(t *testing.T) {
	t.Parallel()
	r := printsRig()
	r.cfg.Structures = newRig(aaplOnly(t)).cfg.Structures
	m := decode(t, r.build(t, "main"))
	if m["run_status"] != "ok" || m["note"] == nil {
		t.Fatalf("status = %v, note = %v", m["run_status"], m["note"])
	}
	// another note keeps the page partial and the close line follows it
	res := aaplOnly(t)
	res.Diff.CaptureOnly = readResult(t, "list_ok").Diff.CaptureOnly
	r.cfg.Structures = newRig(res).cfg.Structures
	m = decode(t, r.build(t, "main"))
	n, _ := m["note"].(string)
	if m["run_status"] != "partial" || !strings.Contains(n, "Positions not yet in Flex") || !strings.HasSuffix(n, "AAPL 最近成交 2026-11-06") {
		t.Fatalf("status = %v, note = %q", m["run_status"], n)
	}
}

type stubDividends struct {
	dividend.Reader
	rows     []contracts.DividendExpectation
	id       *int
	from, to *contracts.Day
}

func (s stubDividends) Dividends(_ context.Context, id int, from, to contracts.Day) ([]contracts.DividendExpectation, error) {
	*s.id, *s.from, *s.to = id, from, to
	return s.rows, nil
}

func exDay(d string) *types.SerializableDate {
	tm, _ := time.Parse(dayLayout, d)
	return &types.SerializableDate{Time: tm}
}

// exRig is a rig with one AAPL structure whose only leg has the right and quantity given; the id of AAPL is 77.
func exRig(right, qty string, reader dividend.Reader) *rig {
	res := compute.StructureResult{Structures: []compute.Structure{{ID: "s1", Underlying: "AAPL", Kind: "naked_call",
		Legs: []compute.StructureLeg{{Right: right, Quantity: qty, Expiry: "2026-12-18"}}}}}
	r := newRig(res)
	r.cfg.Dividends = reader
	r.cfg.Account = stubAccount{byDay: map[string][]store.Position{"2026-11-09": {{SecType: "stock", Symbol: "AAPL", ConID: 77}}}}
	return r
}

func exCell(t *testing.T, r *rig) any {
	t.Helper()
	return groupRows(t, decode(t, r.build(t, "main")), 1)[0]["ex_dividend"]
}

func wantGap(t *testing.T, cell any, reason, text string) {
	t.Helper()
	g, ok := cell.(map[string]any)
	if !ok || g["reason"] != reason || g["text"] != text {
		t.Fatalf("cell = %v, want gap %s %q", cell, reason, text)
	}
}

func TestExDividendGapReasons(t *testing.T) {
	t.Parallel()
	wantGap(t, exCell(t, exRig("C", "-1", nil)), "no_reader", "没有除息数据来源")
	var id int
	var from, to contracts.Day
	passed := contracts.DividendExpectation{Status: "present", NextExDate: exDay("2026-11-08")}
	wantGap(t, exCell(t, exRig("C", "-1", stubDividends{rows: []contracts.DividendExpectation{passed}, id: &id, from: &from, to: &to})),
		"no_data", "除息日已过，等待下次采集")
	wantGap(t, exCell(t, exRig("C", "-1", stubDividends{id: &id, from: &from, to: &to})),
		"no_data", "没有除息预期记录（该股可能不派息，或尚未采集）")
	if got := exCell(t, exRig("P", "-1", nil)); got != nil {
		t.Fatalf("put cell = %v, want null", got)
	}
}

func TestExDividendFromStoredRow(t *testing.T) {
	t.Parallel()
	var id int
	var from, to contracts.Day
	amt := "0.25"
	old := contracts.DividendExpectation{Status: "present", NextExDate: exDay("2026-11-20"), NextAmount: ptr("0.20")}
	cur := contracts.DividendExpectation{Status: "present", NextExDate: exDay("2026-11-09"), NextAmount: &amt}
	r := exRig("C", "-1", stubDividends{rows: []contracts.DividendExpectation{old, cur}, id: &id, from: &from, to: &to})
	if got := exCell(t, r); got != "2026-11-09 · 0.25" {
		t.Fatalf("cell = %v", got)
	}
	if id != 77 || from != "2026-10-30" || to != testDay {
		t.Fatalf("read id %d from %s to %s", id, from, to)
	}
}

func TestExDividendNoRowIsChineseGap(t *testing.T) {
	t.Parallel()
	var id int
	var from, to contracts.Day
	for _, rows := range [][]contracts.DividendExpectation{{{Status: "no_value"}}, {{Status: "unsupported"}}, {{Status: "error"}}} {
		wantGap(t, exCell(t, exRig("C", "-1", stubDividends{rows: rows, id: &id, from: &from, to: &to})),
			"no_data", "没有除息预期记录（该股可能不派息，或尚未采集）")
	}
	wantGap(t, exCell(t, exRig("C", "-1", stubDividends{id: &id, from: &from, to: &to})),
		"no_data", "没有除息预期记录（该股可能不派息，或尚未采集）")
}

func TestExDividendPassedDateIsChineseGap(t *testing.T) {
	t.Parallel()
	var id int
	var from, to contracts.Day
	rows := []contracts.DividendExpectation{{Status: "present", NextExDate: exDay("2026-11-08"), NextAmount: ptr("0.25")}}
	wantGap(t, exCell(t, exRig("C", "-1", stubDividends{rows: rows, id: &id, from: &from, to: &to})),
		"no_data", "除息日已过，等待下次采集")
}

func TestExDividendPutOnlyIsNotApplicable(t *testing.T) {
	t.Parallel()
	var id int
	var from, to contracts.Day
	rows := []contracts.DividendExpectation{{Status: "present", NextExDate: exDay("2026-11-20"), NextAmount: ptr("0.25")}}
	for _, leg := range [][2]string{{"P", "-1"}, {"C", "1"}} {
		if got := exCell(t, exRig(leg[0], leg[1], stubDividends{rows: rows, id: &id, from: &from, to: &to})); got != nil {
			t.Fatalf("cell of %v = %v, want null", leg, got)
		}
	}
	if id != 0 {
		t.Fatalf("reader was called for id %d", id)
	}
}

func TestPutOnlyPageIsOk(t *testing.T) {
	t.Parallel()
	m := decode(t, newRig(aaplOnly(t)).build(t, "main"))
	if m["run_status"] != "ok" {
		t.Fatalf("run_status = %v, note = %v", m["run_status"], m["note"])
	}
	if got := groupRows(t, m, 0)[0]["ex_dividend"]; got != nil {
		t.Fatalf("ex_dividend = %v, want null", got)
	}
}
