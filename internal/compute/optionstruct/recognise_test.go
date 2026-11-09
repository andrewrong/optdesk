package optionstruct

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

type wantStructure struct{ Kind, ID string }

type fixtureCase struct {
	Name       string
	Positions  []compute.LedgerPosition
	Cycles     []compute.LedgerCycle
	Structures []wantStructure
	Other      []wantStructure
}

// fixture reads one case of a testdata file.
func fixture(t *testing.T, file, name string) fixtureCase {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Cases []fixtureCase }
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no case %s in %s", name, file)
	return fixtureCase{}
}

func evaluate(positions []compute.LedgerPosition, cycles []compute.LedgerCycle) compute.StructureResult {
	return New().Evaluate(compute.StructureInput{Alias: "acct", Positions: positions, Cycles: cycles}, compute.StructureParams{})
}

func kinds(ss []compute.Structure) []wantStructure {
	out := []wantStructure{}
	for _, s := range ss {
		out = append(out, wantStructure{s.Kind, s.ID})
	}
	return out
}

// checkCase evaluates a fixture case and compares the kinds and ids of both lists.
func checkCase(t *testing.T, file, name string) compute.StructureResult {
	t.Helper()
	c := fixture(t, file, name)
	res := evaluate(c.Positions, c.Cycles)
	if got := kinds(res.Structures); !equalWant(got, c.Structures) {
		t.Errorf("structures = %v, want %v", got, c.Structures)
	}
	if got := kinds(res.Other); !equalWant(got, c.Other) {
		t.Errorf("other = %v, want %v", got, c.Other)
	}
	return res
}

func equalWant(a, b []wantStructure) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// pos builds an open USD position of AAPL; qty is signed.
func pos(id, right, strike, qty, price string) compute.LedgerPosition {
	return compute.LedgerPosition{
		PositionID: id, Alias: "acct", Underlying: "AAPL", Currency: "USD", Status: "open",
		Right: right, Expiry: "2026-12-18", Strike: strike, Quantity: qty, Multiplier: "100",
		OpenPrice: price, OpenedOn: "2026-01-02", MarkStatus: "no_mark",
	}
}

func TestPairIntoPutSpread(t *testing.T) {
	t.Parallel()
	res := checkCase(t, "spread_and_put.json", "put_spread")
	if len(res.Structures) == 1 && len(res.Structures[0].Legs) != 2 {
		t.Errorf("legs = %d, want 2", len(res.Structures[0].Legs))
	}
}

func TestPairIntoCallSpread(t *testing.T) {
	t.Parallel()
	checkCase(t, "spread_and_put.json", "call_spread")
}

func TestPutDebitSpreadIsOther(t *testing.T) {
	t.Parallel()
	res := checkCase(t, "spread_and_put.json", "put_debit_spread")
	if len(res.Other) == 1 && res.Other[0].Figures.MaxLoss.Gap != "no_rule" {
		t.Errorf("other max loss = %+v, want gap no_rule", res.Other[0].Figures.MaxLoss)
	}
}

func TestCoveredNeedsCycleShares(t *testing.T) {
	t.Parallel()
	checkCase(t, "covered_and_naked.json", "covered")
	short := fixture(t, "covered_and_naked.json", "covered")
	short.Cycles[0].Shares = "99"
	res := evaluate(short.Positions, short.Cycles)
	if len(res.Structures) != 1 || res.Structures[0].Kind != "naked_call" {
		t.Errorf("99 shares: %v, want naked_call", kinds(res.Structures))
	}
}

func TestSharesUsedFirstInFirstOut(t *testing.T) {
	t.Parallel()
	checkCase(t, "covered_and_naked.json", "first_in_first_out")
}

func TestNakedCall(t *testing.T) {
	t.Parallel()
	checkCase(t, "covered_and_naked.json", "naked")
}

func TestSingles(t *testing.T) {
	t.Parallel()
	checkCase(t, "spread_and_put.json", "short_put")
	lc := pos("lc", "C", "200", "1", "1.00")
	lc.Expiry = "2027-01-15"
	res := evaluate([]compute.LedgerPosition{pos("lp", "P", "40", "1", "1.00"), lc}, nil)
	got := kinds(res.Structures)
	if len(got) != 2 {
		t.Fatalf("singles = %v", got)
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g.Kind] = true
	}
	if !seen["long_put"] || !seen["long_call"] {
		t.Errorf("kinds = %v, want long_put and long_call", got)
	}
}

func TestDiagonalIsOther(t *testing.T) {
	t.Parallel()
	checkCase(t, "other_kinds.json", "diagonal")
	checkCase(t, "other_kinds.json", "straddle")
}

func TestPositionNeverSplit(t *testing.T) {
	t.Parallel()
	// Sold 2, bought 1: the quantities do not match, nothing is split to force a pair.
	res := checkCase(t, "other_kinds.json", "quantity_mismatch")
	if len(res.Other) == 1 {
		o := res.Other[0]
		if len(o.Legs) != 2 || o.Legs[1].Quantity != "-2" {
			t.Errorf("legs = %+v, want both whole positions", o.Legs)
		}
	}
}

func TestStructureIDStable(t *testing.T) {
	t.Parallel()
	a := []compute.LedgerPosition{pos("s1", "P", "50", "-1", "1.50"), pos("b1", "P", "45", "1", "0.50")}
	b := []compute.LedgerPosition{a[1], a[0]}
	ra, rb := evaluate(a, nil), evaluate(b, nil)
	if len(ra.Structures) != 1 || len(rb.Structures) != 1 {
		t.Fatalf("structures = %d, %d", len(ra.Structures), len(rb.Structures))
	}
	if ra.Structures[0].ID != "acct:b1" || rb.Structures[0].ID != "acct:b1" {
		t.Errorf("ids = %s, %s, want acct:b1 (the leg of the lowest strike)", ra.Structures[0].ID, rb.Structures[0].ID)
	}
	// Equal strikes: the lower position id names the structure.
	tie := evaluate([]compute.LedgerPosition{pos("z", "P", "50", "-1", "1"), pos("a", "P", "50", "-1", "1")}, nil)
	if len(tie.Structures) != 2 || tie.Structures[0].ID == tie.Structures[1].ID {
		t.Errorf("tie ids = %v", kinds(tie.Structures))
	}
}

func TestOrderMostNegativeFirst(t *testing.T) {
	t.Parallel()
	naked := pos("naked", "C", "300", "-1", "1") // gap: last
	naked.Expiry = "2027-01-15"
	res := evaluate([]compute.LedgerPosition{
		naked,
		pos("small", "P", "10", "-1", "1"), // -900
		pos("big", "P", "90", "-1", "3"),   // -8700
		pos("mid", "P", "50", "-1", "1"),   // -4900
	}, nil)
	var ids []string
	for _, s := range res.Structures {
		ids = append(ids, s.ID)
	}
	want := "acct:big,acct:mid,acct:small,acct:naked"
	if got := strings.Join(ids, ","); got != want {
		t.Errorf("order = %s, want %s", got, want)
	}
}

func TestOnlyOpenPositionsRecognised(t *testing.T) {
	t.Parallel()
	closed := pos("c1", "P", "50", "-1", "1.50")
	closed.Status = "closed"
	res := evaluate([]compute.LedgerPosition{closed, pos("o1", "P", "40", "-1", "1")}, nil)
	if len(res.Structures) != 1 || res.Structures[0].ID != "acct:o1" || len(res.Other) != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestUnparsedPositionIsOther(t *testing.T) {
	t.Parallel()
	bad := pos("x", "P", "", "-1", "1")
	res := evaluate([]compute.LedgerPosition{bad}, nil)
	if len(res.Other) != 1 || res.Other[0].Figures.MaxLoss.Gap != "no_rule" || len(res.Structures) != 0 {
		t.Errorf("result = %+v", res)
	}
}

func TestFiguresOfIssue4AreNoData(t *testing.T) {
	t.Parallel()
	res := evaluate([]compute.LedgerPosition{pos("s1", "P", "90", "-1", "3")}, nil)
	f := res.Structures[0].Figures
	for name, v := range map[string]compute.FigureValue{
		"unrealized": f.UnrealizedPnL, "earned": f.EarnedShare, "expiry": f.ExpiryDistance,
		"close": f.CloseCost, "rollc": f.RollCredit, "rolln": f.RollNet,
		"commission": f.RecentCommission, "cash": f.CashAfter,
	} {
		if v != (compute.FigureValue{Gap: "no_data"}) {
			t.Errorf("%s = %+v, want gap no_data", name, v)
		}
	}
}

var (
	bannedImports = []string{"os", "net/http", "database/sql", "sort"}
	bannedModules = []string{"internal/store", "internal/notify", "internal/pages", "internal/jobs"}
)

func TestNoFloatNoClockNoIO(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("glob: %v %d", err, len(files))
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, f, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		timeName := checkImports(t, f, file)
		checkIdents(t, fset, file, timeName)
	}
}

// checkImports fails on a banned import and returns the local name of "time", or "".
func checkImports(t *testing.T, f string, file *ast.File) string {
	t.Helper()
	timeName := ""
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		for _, b := range bannedImports {
			if path == b {
				t.Errorf("%s imports %s", f, path)
			}
		}
		for _, b := range bannedModules {
			if strings.HasSuffix(path, b) || strings.Contains(path, b+"/") {
				t.Errorf("%s imports %s", f, path)
			}
		}
		if path == "time" {
			timeName = "time"
		}
	}
	return timeName
}

// checkIdents fails on float32, float64 and on time.Now or time.Sleep.
func checkIdents(t *testing.T, fset *token.FileSet, file *ast.File, timeName string) {
	t.Helper()
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if x.Name == "float32" || x.Name == "float64" {
				t.Errorf("%s: identifier %s", fset.Position(x.Pos()), x.Name)
			}
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && timeName != "" && id.Name == timeName &&
				(x.Sel.Name == "Now" || x.Sel.Name == "Sleep") {
				t.Errorf("%s: time.%s", fset.Position(x.Pos()), x.Sel.Name)
			}
		}
		return true
	})
}
