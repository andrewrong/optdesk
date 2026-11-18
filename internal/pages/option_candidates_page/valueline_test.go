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
	"testing"
	"unicode/utf8"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/valuation"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// valueConfig is the tiers_ok page with the valuation readers set.
func valueConfig(st valuation.State, calc valuelinesapi.Calculator) (Config, *fakeValuation) {
	c, _ := tierScenario("tiers_ok")
	v := &fakeValuation{st: st}
	c.Valuation, c.ValueLines = v, calc
	return c, v
}

func companiesOfBuild(t *testing.T, c Config) []companyView {
	t.Helper()
	return buildTiersOf(t, c).Companies
}

func reasonOf(cv companyView) string {
	if cv.Valuation == nil || cv.Valuation.Reason == nil {
		return ""
	}
	return *cv.Valuation.Reason
}

func TestValueLineFromCalculator(t *testing.T) {
	t.Parallel()
	calc := &fakeLines{buy: "91.2"}
	c, _ := valueConfig(valueState(valueEntry("AAA", valueReview())), calc)
	cs := companiesOfBuild(t, c)
	v := cs[0].Valuation
	if v == nil || v.Reason != nil || v.Text != "$91.20 (review 2026-11-10)" {
		t.Fatalf("valuation = %+v", v)
	}
	if v.Label != "Valuation buy line B_hi" || v.GateLabel != "Buy price used by the gate (hand-typed)" {
		t.Errorf("labels = %q, %q", v.Label, v.GateLabel)
	}
	if len(calc.in) != 1 {
		t.Fatalf("Lines called %d times", len(calc.in))
	}
	in := calc.in[0]
	want := valuelinesapi.Input{
		V: "100", P: "3", Market: "US", R: valuelinesapi.RValues{US: "0.9", JP: "0.8", HKCN: "0.7"},
		Margin: []valuelinesapi.MarginRow{{PFrom: "1", M: "0.1", W: "0.2", H: "0.3", S: "0.4"}},
	}
	if !reflect.DeepEqual(in, want) || in.Close != "" {
		t.Errorf("input = %+v", in)
	}
	if len(cs) != 1 {
		t.Errorf("companies = %d: BBB has no buy price and no figure, so it is on the idle line", len(cs))
	}
}

func TestValueLineMatchIgnoresCase(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("aaa", valueReview())), &fakeLines{buy: "91.2"})
	if v := companiesOfBuild(t, c)[0].Valuation; v == nil || v.Reason != nil {
		t.Fatalf("valuation = %+v", v)
	}
}

func TestValueLineAmbiguousTicker(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("AAA", valueReview())), &fakeLines{buy: "91.2"})
	c.Names = &namesFake{names: map[int]string{7: "AAA", 8: "AAA"}}
	for i, cv := range companiesOfBuild(t, c) {
		if reasonOf(cv) != "ambiguous" || cv.Valuation.Text != valueReasonAmbiguousZh {
			t.Errorf("company %d = %+v", i, cv.Valuation)
		}
	}
}

func TestValueLineReasonOrder(t *testing.T) {
	t.Parallel()
	e := valueEntry("AAA", nil)
	e.Profile.Pool = "removed"
	c, _ := valueConfig(valueState(e), &fakeLines{buy: "91.2"})
	if got := reasonOf(companiesOfBuild(t, c)[0]); got != "removed" {
		t.Errorf("reason = %q", got)
	}
}

func TestValueLineNoReview(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("AAA", nil)), &fakeLines{buy: "91.2"})
	if got := reasonOf(companiesOfBuild(t, c)[0]); got != "no_review" {
		t.Errorf("reason = %q", got)
	}
}

func TestValueLineNoTables(t *testing.T) {
	t.Parallel()
	for name, mod := range map[string]func(*valuation.State){
		"r":      func(s *valuation.State) { s.RVersion = nil },
		"margin": func(s *valuation.State) { s.Margin = nil },
	} {
		st := valueState(valueEntry("AAA", valueReview()))
		mod(&st)
		c, _ := valueConfig(st, &fakeLines{buy: "91.2"})
		if got := reasonOf(companiesOfBuild(t, c)[0]); got != "no_tables" {
			t.Errorf("%s: reason = %q", name, got)
		}
	}
}

func TestValueLineCurrency(t *testing.T) {
	t.Parallel()
	e := valueEntry("AAA", valueReview())
	e.Profile.Currency = "JPY"
	calc := &fakeLines{buy: "91.2"}
	c, _ := valueConfig(valueState(e), calc)
	v := companiesOfBuild(t, c)[0].Valuation
	if reasonOf(companiesOfBuild(t, c)[0]) != "currency" || v.Text != valueReasonCurrencyZh || len(calc.in) != 0 {
		t.Errorf("valuation = %+v, calls %d", v, len(calc.in))
	}
}

func TestValueLineBadInput(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("AAA", valueReview())), &fakeLines{err: valuelinesapi.ErrBadInput})
	if got := reasonOf(companiesOfBuild(t, c)[0]); got != "bad_input" {
		t.Errorf("reason = %q", got)
	}
}

func TestValueLineReadErrorIsSoft(t *testing.T) {
	t.Parallel()
	c, v := valueConfig(valueState(), &fakeLines{buy: "91.2"})
	v.err = errors.New("db down")
	cs := companiesOfBuild(t, c)
	if len(cs) != 1 {
		t.Fatalf("companies = %d, want the one with a buy price", len(cs))
	}
	for i, cv := range cs {
		if reasonOf(cv) != "read_failed" || cv.Valuation.Text != valueReasonReadFailedZh {
			t.Errorf("company %d = %+v", i, cv.Valuation)
		}
	}
	if v.calls != 1 {
		t.Errorf("Current called %d times", v.calls)
	}
}

func TestValueLineNilReadersNull(t *testing.T) {
	t.Parallel()
	st := valueState(valueEntry("AAA", valueReview()))
	c1, _ := valueConfig(st, &fakeLines{buy: "91.2"})
	c1.Valuation = nil
	c2, _ := valueConfig(st, nil)
	for _, c := range []Config{c1, c2} {
		for i, cv := range companiesOfBuild(t, c) {
			if cv.Valuation != nil {
				t.Errorf("company %d has a valuation", i)
			}
		}
	}
}

func TestValueLineGateTextIsBuyPrice(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("AAA", valueReview()), valueEntry("BBB", valueReview())), &fakeLines{buy: "91.2"})
	cs := companiesOfBuild(t, c)
	if cs[0].BuyPrice == nil || cs[0].Valuation.GateText != *cs[0].BuyPrice {
		t.Errorf("AAA = %+v", cs[0])
	}
	if cs[1].BuyPrice != nil || cs[1].Valuation.GateText != "not set" {
		t.Errorf("BBB = %+v", cs[1])
	}
}

func TestValueLineTextsFromZhConsts(t *testing.T) {
	t.Parallel()
	c, _ := valueConfig(valueState(valueEntry("AAA", nil)), &fakeLines{buy: "91.2"})
	v := companiesOfBuild(t, c)[0].Valuation
	for what, pair := range map[string][2]string{
		"hint": {deref(v.Hint), valueHintZh}, "gate hint": {deref(v.GateHint), gateHintZh}, "no_review": {v.Text, valueReasonNoReviewZh},
	} {
		if pair[0] != pair[1] {
			t.Errorf("%s = %q", what, pair[0])
		}
		if n := utf8.RuneCountInString(pair[0]); n < 1 || n > 200 {
			t.Errorf("%s has %d characters", what, n)
		}
	}
	for code, s := range valueReasonText {
		if n := utf8.RuneCountInString(s); n < 1 || n > 200 {
			t.Errorf("%s has %d characters", code, n)
		}
	}
}

func TestValueLineDoesNotChangeTiers(t *testing.T) {
	t.Parallel()
	with, _ := valueConfig(valueState(valueEntry("AAA", valueReview())), &fakeLines{buy: "91.2"})
	without, _ := tierScenario("tiers_ok")
	a, b := buildTiersOf(t, with), buildTiersOf(t, without)
	for i := range a.Companies {
		a.Companies[i].Valuation = nil
	}
	if !reflect.DeepEqual(a, b) {
		t.Error("the valuation readers changed the tiers, rows or counts")
	}
}

func TestNoArithmeticInValueLine(t *testing.T) {
	t.Parallel()
	f, err := parser.ParseFile(token.NewFileSet(), "valueline.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range f.Imports {
		switch imp.Path.Value {
		case `"github.com/shopspring/decimal"`, `"math"`, `"math/big"`, `"sort"`:
			t.Errorf("imports %s", imp.Path.Value)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && (id.Name == "percentText" || id.Name == "roundDigits") {
			t.Errorf("uses %s", id.Name)
		}
		return true
	})
}

func TestValueLineValidatesSchema(t *testing.T) {
	t.Parallel()
	golden, err := os.ReadFile("testdata/valueline.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(golden))
	if err != nil {
		t.Fatal(err)
	}
	if err := pageSchema(t).Validate(v); err != nil {
		t.Error(err)
	}
	st := valueState(valueEntry("AAA", valueReview()), valueEntry("BBB", nil))
	c, _ := valueConfig(st, &fakeLines{buy: "91.2"})
	b, err := Build(context.Background(), c, nil, buildDay, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(compact(t, golden), compact(t, b)) {
		t.Error("the page differs from its golden file")
	}
	var p struct {
		Tiers struct{ Companies []json.RawMessage } `json:"short_put_tiers"`
	}
	if err := json.Unmarshal(b, &p); err != nil || len(p.Tiers.Companies) != 1 {
		t.Fatalf("companies: %v", err)
	}
}
