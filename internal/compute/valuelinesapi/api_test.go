package valuelinesapi_test

import (
	"errors"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
)

// fakeCalc answers fixed made-up numbers and records the input of Lines.
type fakeCalc struct{ seen valuelinesapi.Input }

func (f *fakeCalc) Lines(in valuelinesapi.Input) (valuelinesapi.Lines, error) {
	f.seen = in
	if in.V == "0" {
		return valuelinesapi.Lines{}, valuelinesapi.ErrBadInput
	}
	return valuelinesapi.Lines{
		C: "90", R: "0.9", Zone: "buy", Flags: &valuelinesapi.Flags{AtHeavy: false, InBuy: true},
	}, nil
}

func (f *fakeCalc) Band(l valuelinesapi.Lines, _ string) valuelinesapi.Band {
	return valuelinesapi.Band{Min: "40", Max: "160", Marks: []valuelinesapi.Mark{{Name: "c", Value: l.C, Pos: "0.4167"}}}
}

func (f *fakeCalc) ChangePercent(from, to string) (string, error) {
	if from == "0" {
		return "", valuelinesapi.ErrBadInput
	}
	return "-12", nil
}

func (f *fakeCalc) ProposeR(rows []valuelinesapi.RRow, yield string) (valuelinesapi.RValues, error) {
	return valuelinesapi.RValues{US: rows[0].US, JP: rows[0].JP, HKCN: rows[0].HKCN}, nil
}

func (f *fakeCalc) RStepWarnings(previous, next valuelinesapi.RValues) []string {
	if previous.US != next.US {
		return []string{"US R moves"}
	}
	return []string{}
}

func (f *fakeCalc) Check(in valuelinesapi.CheckInput) []valuelinesapi.Problem {
	if in.V == "" {
		return []valuelinesapi.Problem{{Field: "v", Text: "V is required"}}
	}
	return []valuelinesapi.Problem{}
}

func (f *fakeCalc) RoundText(value string, places int) string { return value + "r" }

// TestCalculatorContractFake: a caller reads every method of a fake Calculator.
func TestCalculatorContractFake(t *testing.T) {
	t.Parallel()
	var c valuelinesapi.Calculator = &fakeCalc{}
	readLinesAndBand(t, c)
	readChange(t, c)
	readRates(t, c)
	if p := c.Check(valuelinesapi.CheckInput{}); len(p) != 1 || p[0].Field != "v" {
		t.Fatalf("Check = %+v", p)
	}
	if c.RoundText("2.345", 2) != "2.345r" {
		t.Fatal("RoundText not called")
	}
}

func readLinesAndBand(t *testing.T, c valuelinesapi.Calculator) {
	t.Helper()
	in := valuelinesapi.Input{V: "100", P: "3", Market: valuelinesapi.MarketUS, Close: "80"}
	l, err := c.Lines(in)
	if err != nil || l.C != "90" || l.Zone != "buy" || l.Flags == nil || !l.Flags.InBuy {
		t.Fatalf("Lines = %+v, %v", l, err)
	}
	if _, err := c.Lines(valuelinesapi.Input{V: "0"}); !errors.Is(err, valuelinesapi.ErrBadInput) {
		t.Fatalf("err = %v", err)
	}
	b := c.Band(l, "80")
	if b.Min != "40" || b.Max != "160" || len(b.Marks) != 1 || b.Marks[0].Pos != "0.4167" {
		t.Fatalf("Band = %+v", b)
	}
}

func readChange(t *testing.T, c valuelinesapi.Calculator) {
	t.Helper()
	if p, err := c.ChangePercent("100", "88"); err != nil || p != "-12" {
		t.Fatalf("ChangePercent = %q, %v", p, err)
	}
	if _, err := c.ChangePercent("0", "1"); !errors.Is(err, valuelinesapi.ErrBadInput) {
		t.Fatalf("err = %v", err)
	}
}

func readRates(t *testing.T, c valuelinesapi.Calculator) {
	t.Helper()
	r, err := c.ProposeR([]valuelinesapi.RRow{{YieldUpTo: "4", US: "1", JP: "0.9", HKCN: "0.8"}}, "3")
	if err != nil || r.US != "1" || r.JP != "0.9" || r.HKCN != "0.8" {
		t.Fatalf("ProposeR = %+v, %v", r, err)
	}
	if w := c.RStepWarnings(valuelinesapi.RValues{US: "1"}, valuelinesapi.RValues{US: "1.1"}); len(w) != 1 {
		t.Fatalf("warnings = %v", w)
	}
	if w := c.RStepWarnings(valuelinesapi.RValues{}, valuelinesapi.RValues{}); w == nil || len(w) != 0 {
		t.Fatalf("empty warnings = %#v", w)
	}
}

// TestRStepWarnIs005: the step above which an R change warns.
func TestRStepWarnIs005(t *testing.T) {
	t.Parallel()
	if valuelinesapi.RStepWarn != "0.05" {
		t.Fatalf("RStepWarn = %q", valuelinesapi.RStepWarn)
	}
}

// TestMarketNames: the three R market columns.
func TestMarketNames(t *testing.T) {
	t.Parallel()
	if valuelinesapi.MarketUS != "US" || valuelinesapi.MarketJP != "JP" || valuelinesapi.MarketHKCN != "HKCN" {
		t.Fatal("market names changed")
	}
}
