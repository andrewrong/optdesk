package optioncand

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
)

var _ optioncandapi.NewNameOpenRuleFunc = NewNameOpen

type factSpec struct {
	Status      int    `json:"status"`
	Code        string `json:"code"`
	OutsideBand bool   `json:"outside_band"`
	N           int    `json:"n"`
}

type wantSpec struct {
	Outcome  string `json:"outcome"`
	NoQuote  int    `json:"no_quote"`
	Failed   int    `json:"failed"`
	NoSpot   int    `json:"no_spot"`
	Deferred int    `json:"deferred"`
	Ok       int    `json:"ok"`
	Other    int    `json:"other"`
}

func (w wantSpec) open() optioncandapi.NameOpen {
	return optioncandapi.NameOpen{Outcome: w.Outcome, NoQuote: w.NoQuote, Failed: w.Failed,
		NoSpot: w.NoSpot, Deferred: w.Deferred, Ok: w.Ok, Other: w.Other}
}

func expand(specs []factSpec) []optioncandapi.OpenFact {
	var out []optioncandapi.OpenFact
	for _, s := range specs {
		n := s.N
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, optioncandapi.OpenFact{Status: s.Status, ErrorCode: s.Code, OutsideBand: s.OutsideBand})
		}
	}
	return out
}

func TestNameOpenOutcomes(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/nameopen.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name  string     `json:"name"`
		Facts []factSpec `json:"facts"`
		Want  wantSpec   `json:"want"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got := NewNameOpen().Classify(optioncandapi.NameOpenFacts{Name: "n", Facts: expand(c.Facts)})
			if got != c.Want.open() {
				t.Fatalf("got %+v want %+v", got, c.Want.open())
			}
		})
	}
}

func TestNameOpenLiveCases(t *testing.T) {
	t.Parallel()
	classify := func(specs ...factSpec) optioncandapi.NameOpen {
		return NewNameOpen().Classify(optioncandapi.NameOpenFacts{Facts: expand(specs)})
	}
	// 2413: 11 NO_QUOTE rows.
	if got := classify(factSpec{Status: 3, Code: "NO_QUOTE", N: 11}); got.Outcome != optioncandapi.OutcomeNoQuote || got.NoQuote != 11 {
		t.Errorf("2413: %+v", got)
	}
	// 2408: 28 status 7 rows with NO_DATA.
	if got := classify(factSpec{Status: 7, Code: "NO_DATA", N: 28}); got.Outcome != optioncandapi.OutcomeNoSpot || got.NoSpot != 28 {
		t.Errorf("2408: %+v", got)
	}
	// 2409: 28 NO_QUOTE, 1 partial, 3 deferred without code.
	got := classify(factSpec{Status: 3, Code: "NO_QUOTE", N: 28}, factSpec{Status: 2}, factSpec{Status: 7, N: 3})
	if got.Outcome != optioncandapi.OutcomeDeferred || got.NoQuote != 28 || got.Deferred != 3 || got.Ok != 1 {
		t.Errorf("2409: %+v", got)
	}
}
