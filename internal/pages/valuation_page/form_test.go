package valuation_page

import (
	"context"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/store/underlying"
)

func formInput(calc *fakeCalc, key string, typed Typed) Input {
	in := baseInput(calc)
	in.Company = key
	in.Typed = typed
	in.Underlying = []underlying.Row{{Key: "US:XXX", Symbol: "XXX", Market: "US", Currency: "USD", Name: "Made Up X"}}
	return in
}

func TestPreviewUsesCalculator(t *testing.T) {
	t.Parallel()
	calc := newFake()
	in := formInput(calc, "US:XXX", Typed{V: "100", P: "3"})
	b, err := BuildForm(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	if calc.callCount() != 1 || calc.inputs[0].V != "100" || calc.inputs[0].P != "3" || calc.inputs[0].Market != "US" {
		t.Errorf("Lines calls = %+v", calc.inputs)
	}
	l := obj(m["lines"])
	if l["c"] != "90.00" || l["heavy"] != "63.00" || l["core"] != "150.00" || l["zone"] != "buy" {
		t.Errorf("lines = %v", l)
	}
	if m["view"] != "form" || m["current"] != nil || m["history"] != nil || m["rows"] != nil {
		t.Errorf("doc = %v", m)
	}
}

func TestProblemsListedNotRefused(t *testing.T) {
	t.Parallel()
	calc := newFake()
	calc.linesErr = valuelinesapi.ErrBadInput
	calc.problems = []valuelinesapi.Problem{{Field: "v", Text: "V must be above 0"}}
	b, err := BuildForm(context.Background(), formInput(calc, "US:XXX", Typed{Type: "first", VersionDate: "2026-11-01", V: "0", P: "3"}))
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	ps := m["problems"].([]any)
	if len(ps) != 1 || obj(ps[0])["field"] != "v" || obj(ps[0])["text"] != "V must be above 0" || m["error"] != "" || m["lines"] != nil {
		t.Errorf("doc = %v", m)
	}
	if got := calc.checkInputs[0]; got.V != "0" || got.Type != "first" || len(got.Margin) != 2 {
		t.Errorf("Check input = %+v", got)
	}
}

func TestEmptyFormHasNoProblems(t *testing.T) {
	t.Parallel()
	calc := newFake()
	calc.problems = []valuelinesapi.Problem{{Field: "v", Text: "never asked"}}
	b, err := BuildForm(context.Background(), formInput(calc, "US:XXX", Typed{}))
	if err != nil {
		t.Fatal(err)
	}
	if m := validate(t, b); len(m["problems"].([]any)) != 0 || m["lines"] != nil || calc.callCount() != 0 {
		t.Errorf("doc = %v", m)
	}
}

func TestOnlyFourRequired(t *testing.T) {
	t.Parallel()
	b, err := BuildForm(context.Background(), formInput(newFake(), "US:XXX", Typed{}))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range validate(t, b)["required"].([]any) {
		got = append(got, r.(string))
	}
	if want := []string{"type", "version_date", "v", "p"}; !reflect.DeepEqual(got, want) {
		t.Errorf("required = %v, want %v", got, want)
	}
}

func TestMarketDefaultFromKey(t *testing.T) {
	t.Parallel()
	b, err := BuildForm(context.Background(), formInput(newFake(), "US:XXX", Typed{}))
	if err != nil {
		t.Fatal(err)
	}
	m := validate(t, b)
	if obj(m["company"])["r_market"] != "US" || len(m["problems"].([]any)) != 0 {
		t.Errorf("US key: %v", m)
	}
	for key, want := range map[string]string{"US:ABC": "US", "JP:7203": "JP"} {
		if got, ok := MarketDefault(key); !ok || got != want {
			t.Errorf("MarketDefault(%q) = %q, %v", key, got, ok)
		}
	}

	// a free key asks for a choice until one is typed
	b, err = BuildForm(context.Background(), formInput(newFake(), "HK:0000", Typed{Name: "Made Up HK"}))
	if err != nil {
		t.Fatal(err)
	}
	ps := validate(t, b)["problems"].([]any)
	if _, ok := MarketDefault("HK:0000"); ok || len(ps) != 1 || obj(ps[0])["field"] != "r_market" {
		t.Errorf("free key problems = %v", ps)
	}
	b, err = BuildForm(context.Background(), formInput(newFake(), "HK:0000", Typed{Name: "Made Up HK", RMarket: "HKCN"}))
	if err != nil {
		t.Fatal(err)
	}
	if m := validate(t, b); len(m["problems"].([]any)) != 0 || obj(m["company"])["r_market"] != "HKCN" {
		t.Errorf("chosen market: %v", m)
	}
}
