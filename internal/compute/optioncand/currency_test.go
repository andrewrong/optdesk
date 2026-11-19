package optioncand

import (
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/optioncandapi"
)

var _ optioncandapi.NewCurrencyRuleFunc = NewCurrency

var updateGolden = flag.Bool("update", false, "rewrite testdata/currency_jpy.golden.json")

func evalIn(cur string, in compute.CandidateInput) []compute.Candidate {
	return NewCurrency().EvaluateIn(optioncandapi.CurrencyInput{CandidateInput: in, Currency: cur}, testParams())
}

// jpyInput is baseInput with yen prices: strike 100 x multiplier 100 = 10000 yen.
func jpyInput(accounts ...compute.AliasFunds) compute.CandidateInput {
	in := baseInput()
	in.Accounts = accounts
	return in
}

func TestEvaluateInJPYFunds(t *testing.T) {
	t.Parallel()
	jpy := func(funds string) compute.AliasFunds {
		return compute.AliasFunds{Alias: "A", Funds: funds, Currency: "JPY", CaptureDay: runDay}
	}
	usd := compute.AliasFunds{Alias: "B", Funds: "999999", Currency: "USD", CaptureDay: runDay}

	row := evalIn("JPY", jpyInput(jpy("10000")))[0]
	if g := gateOf(t, row, "funding"); g.Status != compute.GatePass || row.Fits["A"] != "fit" || hasMark(row, "no_jpy_funds") {
		t.Errorf("enough yen: gate %+v fits %v marks %v", g, row.Fits, row.Marks)
	}
	row = evalIn("JPY", jpyInput(jpy("9999")))[0]
	if g := gateOf(t, row, "funding"); g.Status != compute.GateBlock || g.Reason != "no_fit" || row.Fits["A"] != "no_fit" {
		t.Errorf("too little yen: gate %+v fits %v", g, row.Fits)
	}
	row = evalIn("JPY", jpyInput(usd))[0]
	if g := gateOf(t, row, "funding"); g.Status != compute.GatePass || !hasMark(row, "no_jpy_funds") || row.Fits["B"] != "unknown" {
		t.Errorf("usd only: gate %+v fits %v marks %v", g, row.Fits, row.Marks)
	}
	if row.Status != "ok" {
		t.Errorf("status %q, want ok", row.Status)
	}
	row = evalIn("JPY", jpyInput())[0]
	if g := gateOf(t, row, "funding"); g.Status != compute.GatePass || hasMark(row, "no_jpy_funds") || !hasMark(row, "no_account") {
		t.Errorf("no account: gate %+v marks %v", g, row.Marks)
	}
}

func TestEvaluateInJPYRate(t *testing.T) {
	t.Parallel()
	in := jpyInput()
	in.Rate = "" // no rate at all: the US gate would be unknown
	row := evalIn("JPY", in)[0]
	if g := gateOf(t, row, "cash_rate"); g.Status != compute.GatePass || !hasMark(row, "no_jpy_rate") {
		t.Errorf("rate gate %+v marks %v", g, row.Marks)
	}
	// bid 2.00 / strike 100 x 365 / 30 = 0.243333...
	if row.AnnualizedYield != "0.243333" {
		t.Errorf("yield %q, want 0.243333", row.AnnualizedYield)
	}
	if usd := evalIn("USD", in)[0]; hasMark(usd, "no_jpy_rate") {
		t.Errorf("USD row carries no_jpy_rate: %v", usd.Marks)
	}
}

func TestEvaluateInEmptyEqualsEvaluate(t *testing.T) {
	t.Parallel()
	in := baseInput()
	in.Quotes = append(in.Quotes, putQuote("P90", "90", "1.00", addDays(runDay, 30)))
	want := eval(in)
	for _, cur := range []string{"", "USD"} {
		if got := evalIn(cur, in); !reflect.DeepEqual(got, want) {
			t.Errorf("currency %q: %+v, want %+v", cur, got, want)
		}
	}
}

func TestEvaluateInJPYGolden(t *testing.T) {
	t.Parallel()
	jpy := compute.AliasFunds{Alias: "A", Funds: "10000", Currency: "JPY", CaptureDay: runDay}
	usd := compute.AliasFunds{Alias: "B", Funds: "20000", Currency: "USD", CaptureDay: runDay}
	cases := map[string]compute.CandidateInput{
		"jpy_account_fits":  jpyInput(jpy),
		"jpy_account_short": jpyInput(compute.AliasFunds{Alias: "A", Funds: "5000", Currency: "JPY", CaptureDay: runDay}),
		"usd_account_only":  jpyInput(usd),
		"no_account":        jpyInput(),
	}
	got := map[string][]compute.Candidate{}
	for name, in := range cases {
		got[name] = evalIn("JPY", in)
	}
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	const path = "testdata/currency_jpy.golden.json"
	if *updateGolden {
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(b) {
		t.Errorf("golden differs; rerun with -update and review the diff\n%s", b)
	}
}
