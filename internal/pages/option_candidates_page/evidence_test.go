package option_candidates_page

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optionevidenceapi"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestEvidenceColumnsOnShortPut(t *testing.T) {
	t.Parallel()
	rows := map[string][]store.Candidate{}
	for _, s := range groupSpecs {
		rows["2026-11-09/"+s.kind] = []store.Candidate{cand(s.kind, 1, "ok"), cand(s.kind, 2, "blocked")}
	}
	p := buildPage(t, rows, buildDay)
	got := p.Groups[0].Main.ids()
	if want := append(groupSpecs[0].colIDs(), evidenceIDs...); !reflect.DeepEqual(got, want) || len(got) != 23 {
		t.Errorf("short put columns = %v", got)
	}
	for _, g := range p.Groups[1:] {
		for _, id := range g.Main.ids() {
			if slices.Contains(evidenceIDs, id) {
				t.Errorf("%s has %s", g.Kind, id)
			}
		}
	}
}

func (s groupSpec) colIDs() []string {
	var ids []string
	for _, c := range s.main {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestEvidenceFromEvaluator(t *testing.T) {
	t.Parallel()
	f := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
		e := fullEvidence()
		e.SkewPct, e.ForecastPct, e.OneSigmaPct = "7.7777", "31.2500", "9.1234"
		return e, nil
	}}
	c := evConfig([]store.Candidate{okPut(1)}, f)
	c.ATMFor = (&atmCalls{iv: "0.3", strike: "95.00"}).fn
	c.Readings, c.Names = allReadings("2026-11-09"), &namesFake{names: map[int]string{7: "AAA"}}
	r := firstRow(buildOut(t, c, nil))
	for id, want := range map[string]struct{ text, value string }{
		"skew": {"+7.8 pts", "7.7777"}, "forecast_vol": {"31.3%", "31.2500"}, "one_sigma": {"$8.58 (9.1%)", "8.5775"},
	} {
		cell := r.Cells[id]
		if cell.Text != want.text || cell.Value == nil || cell.Value.String() != want.value {
			t.Errorf("%s = %+v, want %v", id, cell, want)
		}
	}
}

func TestRowCarriesStoredText(t *testing.T) {
	t.Parallel()
	c0 := okPut(1)
	c0.Delta, c0.Iv, c0.NextExDate = strp("-0.19"), strp("0.312"), dayp("2026-12-01")
	f := &fakeEval{}
	buildOut(t, evConfig([]store.Candidate{c0}, f), nil)
	if len(f.rows) != 1 {
		t.Fatalf("calls = %d", len(f.rows))
	}
	got := f.rows[0]
	if got.Spot != "96.50" || got.Strike != "90.00" || got.Bid != "0.94" || got.BuyPrice != "90.00" || got.IV != "0.312" ||
		got.Delta != "-0.19" || got.ExDiv != "2026-12-01" || got.Multiplier != "100" || got.EarningsBeforeExpiry ||
		got.ChainDay != "2026-11-09" || got.Expiry != "2026-12-18" || got.DTE != 39 {
		t.Errorf("row = %+v", got)
	}
}

func TestATMPairCopied(t *testing.T) {
	t.Parallel()
	a := &atmCalls{iv: "0.294", strike: "95.00"}
	f := &fakeEval{}
	c := evConfig([]store.Candidate{okPut(1), okPut(2)}, f)
	c.ATMFor = a.fn
	buildOut(t, c, nil)
	if len(a.got) != 2 || a.got[0].id != 7 || a.got[0].day != "2026-11-09" || a.got[0].expiry != "2026-12-18" || a.got[0].spot != "96.50" {
		t.Errorf("atm calls = %+v", a.got)
	}
	if f.rows[0].ATMIV != "0.294" || f.rows[0].ATMStrike != "95.00" {
		t.Errorf("row = %+v", f.rows[0])
	}
}

func TestATMEmptyIsGap(t *testing.T) {
	t.Parallel()
	f := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
		return optionevidenceapi.Evidence{SkewWhy: "no_atm", Labels: []string{}}, nil
	}}
	c := evConfig([]store.Candidate{okPut(1)}, f)
	c.ATMFor = (&atmCalls{}).fn
	cell := firstRow(buildOut(t, c, nil)).Cells["skew"]
	if cell.Kind != "gap" || cell.Text != "no ATM quote" || cell.Reason != "no_data" {
		t.Errorf("skew = %+v", cell)
	}
}

func TestBlockedRowsNoEvidence(t *testing.T) {
	t.Parallel()
	f := &fakeEval{}
	a := &atmCalls{}
	blocked := okPut(2)
	blocked.Status = "blocked"
	c := evConfig([]store.Candidate{okPut(1), blocked}, f)
	c.ATMFor = a.fn
	p := buildOut(t, c, nil)
	for _, id := range p.Groups[0].Other.ids() {
		if slices.Contains(evidenceIDs, id) {
			t.Errorf("second table has %s", id)
		}
	}
	if f.calls() != 1 || len(a.got) != 1 {
		t.Errorf("evaluate %d, atm %d calls", f.calls(), len(a.got))
	}
}

func TestNilEvaluatorNoReader(t *testing.T) {
	t.Parallel()
	r := firstRow(buildOut(t, evConfig([]store.Candidate{okPut(1)}, nil), nil))
	for _, id := range evidenceIDs {
		if c := r.Cells[id]; c.Kind != "gap" || c.Reason != "no_reader" {
			t.Errorf("%s = %+v", id, c)
		}
	}
}

func TestNilATMForSkewNoReader(t *testing.T) {
	t.Parallel()
	r := firstRow(buildOut(t, evConfig([]store.Candidate{okPut(1)}, &fakeEval{}), nil))
	if c := r.Cells["skew"]; c.Kind != "gap" || c.Reason != "no_reader" {
		t.Errorf("skew = %+v", c)
	}
	if c := r.Cells["below_buy"]; c.Kind == "gap" {
		t.Errorf("below_buy = %+v", c)
	}
}

func TestBadInputUnreadable(t *testing.T) {
	t.Parallel()
	f := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
		return optionevidenceapi.Evidence{}, errors.Join(errors.New("x"), optionevidenceapi.ErrBadInput)
	}}
	r := firstRow(buildOut(t, evConfig([]store.Candidate{okPut(1)}, f), nil))
	for _, id := range evidenceIDs {
		if c := r.Cells[id]; c.Kind != "gap" || c.Reason != "unreadable" {
			t.Errorf("%s = %+v", id, c)
		}
	}
}

func TestOtherErrorReturned(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	f := &fakeEval{fn: func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
		return optionevidenceapi.Evidence{}, boom
	}}
	_, err := Build(context.Background(), evConfig([]store.Candidate{okPut(1)}, f), nil, buildDay, testNow)
	if !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
	a := &atmCalls{err: boom}
	c := evConfig([]store.Candidate{okPut(1)}, &fakeEval{})
	c.ATMFor = a.fn
	if _, err := Build(context.Background(), c, nil, buildDay, testNow); !errors.Is(err, boom) {
		t.Errorf("atm err = %v", err)
	}
}

func TestNoSpotReason(t *testing.T) {
	t.Parallel()
	nospot := okPut(1)
	nospot.Spot = nil
	for _, e := range []optionevidenceapi.Evaluator{nil, &fakeEval{}} {
		a := &atmCalls{}
		c := evConfig([]store.Candidate{nospot}, e)
		c.ATMFor = a.fn
		r := firstRow(buildOut(t, c, nil))
		for _, id := range evidenceIDs {
			if g := r.Cells[id]; g.Kind != "gap" || g.Text != "no spot" || g.Reason != "no_data" {
				t.Errorf("%s = %+v", id, g)
			}
		}
		if len(a.got) != 0 {
			t.Errorf("atm called")
		}
		if f, ok := e.(*fakeEval); ok && f.calls() != 0 {
			t.Errorf("evaluate called")
		}
	}
}

func TestNoteNamesDGS3MO(t *testing.T) {
	t.Parallel()
	p := buildPage(t, map[string][]store.Candidate{"2026-11-09/short_put": {okPut(1)}}, buildDay)
	if !slices.Contains(p.Notes, "The cash rate used for the bid vs forecast price is DGS3MO for every currency; this is a trial.") {
		t.Errorf("notes = %v", p.Notes)
	}
	for _, c := range p.Groups[0].Main.Columns {
		if c.ID != "bid_vs_forecast" {
			continue
		}
		h := *c.Hint
		i := strings.Index(h, "DGS3MO is the cash rate for every currency (a trial).")
		j := strings.Index(h, "Data days: chain 2026-11-09, rate ")
		if i < 0 || j < i || !strings.HasPrefix(h[j:], "Data days: chain 2026-11-09, rate") || strings.Contains(h[j:], "(a trial)") {
			t.Errorf("hint = %q", h)
		}
	}
}

// goldenScenario is the page of the invented numbers of the golden files.
func goldenScenario(name string) (Config, error) {
	var rows []store.Candidate
	e := &fakeEval{}
	switch name {
	case "evidence_ok":
		a, b := okPut(1), okPut(2)
		a.Delta, a.Iv, b.Strike, b.BidEod, b.Delta = strp("-0.19"), strp("0.305"), "85.00", "0.42", strp("-0.11")
		rows = []store.Candidate{a, b}
	default:
		c := okPut(3)
		c.BuyPrice, c.Delta, c.Spot, c.Strike = nil, nil, strp("41.20"), "40.00"
		rows = []store.Candidate{c}
		e.fn = func(optionevidenceapi.Row) (optionevidenceapi.Evidence, error) {
			return optionevidenceapi.Evidence{
				BelowBuyWhy: "no_buy_price", OneSigma: "2.6400", OneSigmaPct: "6.4000", BreakEvenInSigmas: "1.1000",
				EdgeWhy: "no_forecast", SkewWhy: "no_atm", ForecastErr: "", Labels: []string{},
			}, nil
		}
	}
	c := evConfig(rows, e)
	c.ATMFor = (&atmCalls{iv: "0.294", strike: "95.00"}).fn
	c.Names = &namesFake{names: map[int]string{7: "AAA"}}
	if name != "evidence_ok" {
		c.Names = &namesFake{names: map[int]string{7: "BBB"}}
	}
	if name == "evidence_ok" {
		c.Readings = allReadings("2026-11-09")
	} else {
		c.Readings = &fakeView{}
	}
	return c, nil
}

func TestEvidenceValidatesSchema(t *testing.T) {
	t.Parallel()
	comp := jsonschema.NewCompiler()
	comp.AssertFormat()
	comp.UseLoader(fileLoader{})
	schema, err := comp.Compile(schemaHost + "page.option_candidates_page.v1.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"evidence_ok", "evidence_gaps"} {
		golden, err := os.ReadFile("testdata/" + name + ".golden.json")
		if err != nil {
			t.Fatal(err)
		}
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(golden))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		cfg, _ := goldenScenario(name)
		got, err := Build(context.Background(), cfg, nil, buildDay, testNow)
		if err != nil {
			t.Fatal(err)
		}
		var a, b bytes.Buffer
		if json.Compact(&a, golden) != nil || json.Compact(&b, got) != nil || !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("%s: the page differs from its golden file", name)
		}
	}
}

func TestNoArithmeticImports(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"evidence.go", "evcells.go", "readings.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			switch imp.Path.Value {
			case `"github.com/shopspring/decimal"`, `"math"`, `"math/big"`:
				t.Errorf("%s imports %s", name, imp.Path.Value)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "percentText" || id.Name == "roundDigits") {
					t.Errorf("%s calls %s", name, id.Name)
				}
			}
			return true
		})
	}
}
