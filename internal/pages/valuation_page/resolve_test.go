package valuation_page

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/valuelinesapi"
	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
	"github.com/andrewrong/optdesk/internal/store/valuation"
	"github.com/atombender/go-jsonschema/pkg/types"
)

// fakeCalc answers from fixed values and records what it was asked. Every test makes its own.
type fakeCalc struct {
	mu          sync.Mutex
	lines       valuelinesapi.Lines
	linesErr    error
	band        valuelinesapi.Band
	percent     string
	problems    []valuelinesapi.Problem
	inputs      []valuelinesapi.Input
	checkInputs []valuelinesapi.CheckInput
}

func (f *fakeCalc) Lines(in valuelinesapi.Input) (valuelinesapi.Lines, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.inputs = append(f.inputs, in)
	return f.lines, f.linesErr
}
func (f *fakeCalc) Band(valuelinesapi.Lines, string) valuelinesapi.Band { return f.band }
func (f *fakeCalc) ChangePercent(from, to string) (string, error) {
	if f.percent == "" {
		return "", valuelinesapi.ErrBadInput
	}
	return f.percent, nil
}
func (f *fakeCalc) ProposeR([]valuelinesapi.RRow, string) (valuelinesapi.RValues, error) {
	return valuelinesapi.RValues{}, nil
}
func (f *fakeCalc) RStepWarnings(_, _ valuelinesapi.RValues) []string { return nil }
func (f *fakeCalc) Check(in valuelinesapi.CheckInput) []valuelinesapi.Problem {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkInputs = append(f.checkInputs, in)
	return f.problems
}

// RoundText marks every figure it is given with two places, so that a figure that skipped it shows.
func (f *fakeCalc) RoundText(v string, _ int) string {
	if !strings.Contains(v, ".") {
		return v + ".00"
	}
	return v
}

func (f *fakeCalc) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.inputs)
}

func madeUpLines() valuelinesapi.Lines {
	return valuelinesapi.Lines{
		C: "90", R: "0.9", Heavy: "63", BuyLow: "69.3", BuyHigh: "73.8", FairLow: "83.7", FairHigh: "96.3",
		Trim: "109.8", Clear: "120", Core: "150", M: "0.18", W: "0.07", H: "0.3", S: "0.22",
		Zone: "buy", Flags: &valuelinesapi.Flags{AtHeavy: false, InBuy: true},
	}
}

func newFake() *fakeCalc {
	return &fakeCalc{
		lines: madeUpLines(), percent: "-12",
		band: valuelinesapi.Band{Min: "50", Max: "160", Marks: []valuelinesapi.Mark{
			{Name: "heavy", Value: "63", Pos: "0.12"}, {Name: "close", Value: "70", Pos: "0.18"},
		}},
	}
}

// fakeCloses is the stock id reader and the bar reader in memory.
type fakeCloses struct {
	ids      map[string]int64
	bars     []contracts.PriceBar
	names    [][]string
	barCalls []barCall
}

type barCall struct {
	id       int64
	basis    string
	from, to contracts.Day
}

func (f *fakeCloses) StockIDs(_ context.Context, names []string) ([]store.InstrumentName, error) {
	f.names = append(f.names, names)
	var out []store.InstrumentName
	for _, n := range names {
		if id, ok := f.ids[n]; ok {
			out = append(out, store.InstrumentName{InstrumentID: id, Type: "stock", Name: n})
		}
	}
	return out, nil
}

func (f *fakeCloses) PriceBars(_ context.Context, id int64, basis string, from, to contracts.Day) ([]contracts.PriceBar, error) {
	f.barCalls = append(f.barCalls, barCall{id, basis, from, to})
	return f.bars, nil
}

func bar(day, close string) contracts.PriceBar {
	t, _ := time.Parse(time.DateOnly, day)
	return contracts.PriceBar{Close: close, SessionDay: types.SerializableDate{Time: t}}
}

const testDay = contracts.Day("2026-11-17")

func madeUpState() valuation.State {
	return valuation.State{
		RVersion: &valuation.RVersion{RUS: "0.90", RJP: "1.10", RHKCN: "0.80"},
		Margin: &valuation.MarginTableVersion{Rows: []valuation.MarginTableRow{
			{PFrom: "1", M: "0.12", W: "0.05", H: "0.25", S: "0.15"},
			{PFrom: "3", M: "0.18", W: "0.07", H: "0.30", S: "0.22"},
		}},
	}
}

func entryWith(key, symbol, market, v, p string) valuation.Entry {
	return valuation.Entry{
		Profile: valuation.Profile{CompanyKey: key, Name: "Made Up " + key, Symbol: symbol, Currency: "USD", RMarket: market, Pool: "in_pool"},
		Current: &valuation.Review{ReviewID: "r-" + key, CompanyKey: key, Type: "first", VersionDate: "2026-10-01", Status: "effective", Source: "owner", V: v, P: p},
	}
}

func TestResolveChain(t *testing.T) {
	t.Parallel()
	calc := newFake()
	cr := &fakeCloses{ids: map[string]int64{"XXX": 7}, bars: []contracts.PriceBar{bar("2026-11-13", "71.5"), bar("2026-11-14", "72.5")}}
	res, err := Resolve(context.Background(), entryWith("JP:YYY", "XXX", "JP", "100", "3"), madeUpState(), cr, calc, testDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(calc.inputs) != 1 {
		t.Fatalf("Lines called %d times", len(calc.inputs))
	}
	want := valuelinesapi.Input{
		V: "100", P: "3", Market: "JP", R: valuelinesapi.RValues{US: "0.90", JP: "1.10", HKCN: "0.80"},
		Margin: []valuelinesapi.MarginRow{{PFrom: "1", M: "0.12", W: "0.05", H: "0.25", S: "0.15"}, {PFrom: "3", M: "0.18", W: "0.07", H: "0.30", S: "0.22"}},
		Close:  "72.5",
	}
	got := calc.inputs[0]
	sameMargin := len(got.Margin) == 2 && got.Margin[0] == want.Margin[0] && got.Margin[1] == want.Margin[1]
	if got.V != want.V || got.P != want.P || got.Market != want.Market || got.R != want.R || got.Close != want.Close || !sameMargin {
		t.Errorf("Lines input = %+v, want %+v", got, want)
	}
	if res.Lines.C != "90" || res.Band.Min != "50" || res.Close != "72.5" || res.CloseDay != "2026-11-14" {
		t.Errorf("resolved = %+v", res)
	}
}

func TestResolveCloseFromSymbol(t *testing.T) {
	t.Parallel()
	calc := newFake()
	cr := &fakeCloses{ids: map[string]int64{"XXX": 7}, bars: []contracts.PriceBar{bar("2026-11-14", "72.5")}}
	if _, err := Resolve(context.Background(), entryWith("US:XXX", "XXX", "US", "100", "3"), madeUpState(), cr, calc, testDay); err != nil {
		t.Fatal(err)
	}
	if len(cr.names) != 1 || len(cr.names[0]) != 1 || cr.names[0][0] != "XXX" {
		t.Errorf("StockIDs names = %v", cr.names)
	}
	if len(cr.barCalls) != 1 || cr.barCalls[0].id != 7 || cr.barCalls[0].basis != "trades" || cr.barCalls[0].to != testDay {
		t.Errorf("PriceBars calls = %+v", cr.barCalls)
	}

	for name, cr := range map[string]*fakeCloses{
		"no id":  {ids: map[string]int64{}},
		"no bar": {ids: map[string]int64{"XXX": 7}},
	} {
		calc := newFake()
		if _, err := Resolve(context.Background(), entryWith("US:XXX", "XXX", "US", "100", "3"), madeUpState(), cr, calc, testDay); err != nil {
			t.Fatal(name, err)
		}
		if calc.inputs[0].Close != "" {
			t.Errorf("%s: close %q passed, want none", name, calc.inputs[0].Close)
		}
	}
	// a name with no symbol never reads
	cr2 := &fakeCloses{ids: map[string]int64{"": 1}}
	if _, err := Resolve(context.Background(), entryWith("HK:ZZZ", "", "HKCN", "100", "3"), madeUpState(), cr2, newFake(), testDay); err != nil {
		t.Fatal(err)
	}
	if len(cr2.names) != 0 {
		t.Errorf("StockIDs called for an empty symbol: %v", cr2.names)
	}
}

func TestResolveNoCurrentReview(t *testing.T) {
	t.Parallel()
	calc := newFake()
	e := entryWith("US:XXX", "XXX", "US", "100", "3")
	e.Current = nil
	res, err := Resolve(context.Background(), e, madeUpState(), nil, calc, testDay)
	if res != nil || err != nil || calc.callCount() != 0 {
		t.Errorf("res=%v err=%v calls=%d, want no lines", res, err, calc.callCount())
	}
}

func TestResolveNoTables(t *testing.T) {
	t.Parallel()
	st := madeUpState()
	st.Margin = nil
	_, err := Resolve(context.Background(), entryWith("US:XXX", "XXX", "US", "100", "3"), st, nil, newFake(), testDay)
	if !errors.Is(err, ErrNoTables) {
		t.Errorf("err = %v, want ErrNoTables", err)
	}
}
