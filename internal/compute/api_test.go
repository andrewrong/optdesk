package compute_test

import (
	"context"
	"errors"
	"io"
	"math"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type permErr struct{}

func (permErr) Error() string             { return "bad input" }
func (permErr) Kind() contracts.ErrorKind { return contracts.Permanent }

type fakeIndicators struct{}

func (fakeIndicators) SMA(v []float64, p int) ([]float64, error) { return fill(v, p) }
func (fakeIndicators) EMA(v []float64, p int) ([]float64, error) { return fill(v, p) }
func (fakeIndicators) WMA(v []float64, p int) ([]float64, error) { return fill(v, p) }
func (fakeIndicators) RSI(v []float64, p int) ([]float64, error) { return fill(v, p) }
func (fakeIndicators) ROC(v []float64, p int) ([]float64, error) { return fill(v, p) }
func (fakeIndicators) MACD(v []float64, f, s, g int) ([]float64, []float64, []float64, error) {
	a, err := fill(v, s)
	return a, a, a, err
}
func (fakeIndicators) BBands(v []float64, p int, _, _ float64, _ compute.MAType) ([]float64, []float64, []float64, error) {
	a, err := fill(v, p)
	return a, a, a, err
}
func (fakeIndicators) ATR(h, l, c []float64, p int) ([]float64, error) {
	if len(h) != len(l) || len(l) != len(c) {
		return nil, permErr{}
	}
	return fill(c, p)
}

// fill returns a slice of the input's length with NaN before the first defined value.
func fill(v []float64, p int) ([]float64, error) {
	if p < 1 {
		return nil, permErr{}
	}
	out := make([]float64, len(v))
	for i := range out {
		if i < p-1 {
			out[i] = math.NaN()
		} else {
			out[i] = v[i]
		}
	}
	return out, nil
}

type fakeStats struct{}

func (fakeStats) Mean(v []float64) (float64, error) {
	if len(v) == 0 {
		return 0, permErr{}
	}
	return v[0], nil
}
func (fakeStats) StdDev(v []float64) (float64, error) {
	if len(v) < 2 {
		return 0, permErr{}
	}
	return 1, nil
}
func (fakeStats) PercentileRank(v []float64, x float64, c compute.Compare) (float64, error) {
	if len(v) == 0 {
		return 0, permErr{}
	}
	n := 0
	for _, e := range v {
		if e < x || (c == compute.AtOrBelow && e == x) {
			n++
		}
	}
	return float64(n) / float64(len(v)), nil
}

type fakeRolling struct{}

func (fakeRolling) Mean(v []float64, w int) ([]float64, error)   { return fill(v, w) }
func (fakeRolling) StdDev(v []float64, w int) ([]float64, error) { return fill(v, w) }
func (fakeRolling) Min(v []float64, w int) ([]float64, error)    { return fill(v, w) }
func (fakeRolling) Max(v []float64, w int) ([]float64, error)    { return fill(v, w) }
func (fakeRolling) PercentileRank(v []float64, w int, _ compute.Compare) ([]float64, error) {
	return fill(v, w)
}

type fakeSeriesOps struct{}

func (fakeSeriesOps) FromSeriesPoints(p []contracts.SeriesPoint) compute.Series {
	return make(compute.Series, len(p))
}
func (fakeSeriesOps) Align(a, b compute.Series) (compute.Series, compute.Series) { return a, b }
func (fakeSeriesOps) LogReturns(s compute.Series) compute.Series                 { return s }
func (fakeSeriesOps) Values(s compute.Series) []float64 {
	out := make([]float64, len(s))
	for i, p := range s {
		out[i] = p.Value
	}
	return out
}

type fakeUnits struct{}

func (fakeUnits) PercentToDecimal(p float64) float64    { return p / 100 }
func (fakeUnits) ThetaPerCalendarDay(y float64) float64 { return y / 365 }
func (fakeUnits) VegaPerVolPoint(v float64) float64     { return v * 0.01 }
func (fakeUnits) RoundHalfEven(v float64, s int) (string, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || s < 0 {
		return "", permErr{}
	}
	return "0", nil
}

type fakeRules struct{ rules map[string]compute.Rule }

func (f fakeRules) Rule(id string) (compute.Rule, bool) { r, ok := f.rules[id]; return r, ok }
func (f fakeRules) IDs() []string {
	out := make([]string, 0, len(f.rules))
	for id := range f.rules {
		out = append(out, id)
	}
	return out
}

// fakeFigures is missing for an id with no rule and ok otherwise.
type fakeFigures struct{ rules compute.Rules }

func (f fakeFigures) make(id string, asOf contracts.Day) compute.Figure {
	r, ok := f.rules.Rule(id)
	if !ok {
		return compute.Figure{ID: id, Day: asOf, Status: compute.StatusMissing, Reason: compute.NoInput}
	}
	return compute.Figure{ID: id, Version: r.Version, Day: asOf, DataDay: asOf, Value: 1, Status: compute.StatusOK}
}
func (f fakeFigures) HistoricalVol(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.make(id, d)
}
func (f fakeFigures) Iv30(id string, _ []compute.ExpiryQuotes, _ compute.Point, d contracts.Day) compute.Figure {
	return f.make(id, d)
}
func (f fakeFigures) IvPercentile(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.make(id, d)
}
func (f fakeFigures) IvRank(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.make(id, d)
}
func (f fakeFigures) MovingAverageGap(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.make(id, d)
}
func (f fakeFigures) PercentileInYears(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.make(id, d)
}
func (f fakeFigures) Ratio(id string, _, _ compute.Series, d contracts.Day) compute.Figure {
	return f.make(id, d)
}

// Compile-time proof that each fake satisfies its interface, and each constructor type fits.
var (
	_ compute.Indicators = fakeIndicators{}
	_ compute.Stats      = fakeStats{}
	_ compute.Rolling    = fakeRolling{}
	_ compute.SeriesOps  = fakeSeriesOps{}
	_ compute.Units      = fakeUnits{}
	_ compute.Rules      = fakeRules{}
	_ compute.Figures    = fakeFigures{}

	_ compute.NewIndicatorsFunc = func() compute.Indicators { return fakeIndicators{} }
	_ compute.NewStatsFunc      = func() compute.Stats { return fakeStats{} }
	_ compute.NewRollingFunc    = func(compute.Stats) compute.Rolling { return fakeRolling{} }
	_ compute.NewSeriesOpsFunc  = func() compute.SeriesOps { return fakeSeriesOps{} }
	_ compute.NewUnitsFunc      = func() compute.Units { return fakeUnits{} }
	_ compute.NewRulesFunc      = func() compute.Rules { return fakeRules{} }
	_ compute.NewFiguresFunc    = func(compute.Stats, compute.Rolling, compute.SeriesOps, compute.Rules) compute.Figures {
		return fakeFigures{}
	}
)

func TestMissingNeverNaN(t *testing.T) {
	t.Parallel()
	var figs compute.Figures = fakeFigures{rules: fakeRules{}}
	f := figs.HistoricalVol("nope", nil, "2026-11-06")
	if f.Status != compute.StatusMissing || f.Value != 0 || f.Reason == "" {
		t.Fatalf("missing figure = %+v, want Status missing, Value 0, non-empty Reason", f)
	}
	if f.Day != "2026-11-06" {
		t.Fatalf("Day = %q, want the as-of day", f.Day)
	}
}

func TestFiguresTakeRuleVersion(t *testing.T) {
	t.Parallel()
	rules := fakeRules{rules: map[string]compute.Rule{"x.hv30": {ID: "x.hv30", Version: 3}}}
	var figs compute.Figures = fakeFigures{rules: rules}
	f := figs.Ratio("x.hv30", nil, nil, "2026-11-06")
	if f.Status != compute.StatusOK || f.Version != 3 || f.ID != "x.hv30" {
		t.Fatalf("figure = %+v, want ok, version 3", f)
	}
}

func TestRulesThroughInterface(t *testing.T) {
	t.Parallel()
	var r compute.Rules = fakeRules{rules: map[string]compute.Rule{"a": {ID: "a", Compare: compute.Below}}}
	got, ok := r.Rule("a")
	if !ok || got.Compare != compute.Below {
		t.Fatalf("Rule(a) = %+v, %v", got, ok)
	}
	if _, ok := r.Rule("b"); ok {
		t.Fatal("Rule(b) found, want absent")
	}
	if ids := r.IDs(); len(ids) != 1 || ids[0] != "a" {
		t.Fatalf("IDs = %v", ids)
	}
}

func TestWarmupKeepsLength(t *testing.T) {
	t.Parallel()
	var ind compute.Indicators = fakeIndicators{}
	out, err := ind.SMA([]float64{1, 2, 3, 4}, 3)
	if err != nil || len(out) != 4 || !math.IsNaN(out[0]) || !math.IsNaN(out[1]) || out[2] != 3 {
		t.Fatalf("SMA = %v, %v", out, err)
	}
	var roll compute.Rolling = fakeRolling{}
	out, err = roll.Mean([]float64{1, 2}, 2)
	if err != nil || len(out) != 2 || !math.IsNaN(out[0]) {
		t.Fatalf("Rolling.Mean = %v, %v", out, err)
	}
}

func TestBadInputIsPermanent(t *testing.T) {
	t.Parallel()
	check := func(name string, err error) {
		t.Helper()
		te, ok := err.(contracts.TypedError)
		if !ok || te.Kind() != contracts.Permanent {
			t.Errorf("%s: err = %v, want Permanent TypedError", name, err)
		}
	}
	var ind compute.Indicators = fakeIndicators{}
	_, err := ind.SMA([]float64{1}, 0)
	check("SMA period 0", err)
	_, err = ind.ATR([]float64{1}, []float64{1, 2}, []float64{1}, 1)
	check("ATR lengths", err)
	var st compute.Stats = fakeStats{}
	_, err = st.Mean(nil)
	check("Mean empty", err)
	var u compute.Units = fakeUnits{}
	_, err = u.RoundHalfEven(math.NaN(), 2)
	check("NaN", err)
	_, err = u.RoundHalfEven(1, -1)
	check("scale", err)
}

func TestStatsAndSeriesAndUnits(t *testing.T) {
	t.Parallel()
	var st compute.Stats = fakeStats{}
	v := []float64{1, 2, 2, 3}
	below, _ := st.PercentileRank(v, 2, compute.Below)
	atOr, _ := st.PercentileRank(v, 2, compute.AtOrBelow)
	if below != 0.25 || atOr != 0.75 {
		t.Fatalf("below %v atOrBelow %v", below, atOr)
	}
	var ops compute.SeriesOps = fakeSeriesOps{}
	s := compute.Series{{Day: "2026-11-05", Value: 1}, {Day: "2026-11-06", Value: 2}}
	a, b := ops.Align(s, s)
	if len(a) != 2 || len(b) != 2 || ops.Values(a)[1] != 2 {
		t.Fatalf("align/values: %v %v", a, b)
	}
	var u compute.Units = fakeUnits{}
	if u.PercentToDecimal(25) != 0.25 || u.VegaPerVolPoint(100) != 1 || u.ThetaPerCalendarDay(365) != 1 {
		t.Fatal("unit conversions")
	}
}

var _ compute.EvaluateAlertsFunc = fakeEvaluate
var _ compute.LoadAlertRulesFunc = fakeLoad
var _ compute.DefaultAlertRulesFunc = fakeDefault

func fakeDefault() (compute.AlertRules, error) {
	return compute.AlertRules{Rules: []compute.AlertRule{{ID: "r1", Figure: "market.vix_pctile_3y", Subject: "VIXCLS"}}, Hash: "h"}, nil
}

func fakeLoad(r io.Reader) (compute.AlertRules, error) {
	if _, err := io.ReadAll(r); err != nil {
		return compute.AlertRules{}, err
	}
	return fakeDefault()
}

// fakeEvaluate reads once per rule and reports the newest row as fired; no row is no_data.
func fakeEvaluate(ctx context.Context, rs compute.AlertRules, read compute.ReadingFunc, today contracts.Day) ([]compute.AlertResult, error) {
	var out []compute.AlertResult
	for _, r := range rs.Rules {
		rows, err := read(ctx, r.Figure, r.SubjectKind, r.Subject, today, today)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			out = append(out, compute.AlertResult{RuleID: r.ID, State: "no_data", Reason: "no_reading"})
			continue
		}
		out = append(out, compute.AlertResult{RuleID: r.ID, State: "fired", Day: today, DataDay: today})
	}
	return out, nil
}

func TestAlertContract(t *testing.T) {
	t.Parallel()
	rules, err := compute.DefaultAlertRulesFunc(fakeDefault)()
	if err != nil {
		t.Fatal(err)
	}
	var evaluate compute.EvaluateAlertsFunc = fakeEvaluate
	empty := compute.ReadingFunc(func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, nil
	})
	got, err := evaluate(context.Background(), rules, empty, "2026-11-08")
	if err != nil || len(got) != 1 || got[0].State != "no_data" || got[0].Reason != "no_reading" {
		t.Fatalf("empty reader: %v %v", got, err)
	}
	one := compute.ReadingFunc(func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return []contracts.ReadingRecord{{}}, nil
	})
	got, err = evaluate(context.Background(), rules, one, "2026-11-08")
	if err != nil || len(got) != 1 || got[0].State != "fired" || got[0].Day != "2026-11-08" {
		t.Fatalf("one row: %v %v", got, err)
	}
	boom := errors.New("reader down")
	bad := compute.ReadingFunc(func(context.Context, string, string, string, contracts.Day, contracts.Day) ([]contracts.ReadingRecord, error) {
		return nil, boom
	})
	if _, err = evaluate(context.Background(), rules, bad, "2026-11-08"); !errors.Is(err, boom) {
		t.Fatalf("reader error must be an error of the call: %v", err)
	}
	loaded, err := compute.LoadAlertRulesFunc(fakeLoad)(emptyReader{})
	if err != nil || len(loaded.Rules) != 1 {
		t.Fatalf("load: %v %v", loaded, err)
	}
}

type emptyReader struct{}

func (emptyReader) Read([]byte) (int, error) { return 0, io.EOF }

// fakeSharp is a stand-in implementation of the sharp-move types: inclusive at the limit,
// false for a nil change.
func fakeSharp() (compute.SharpMarkFunc, compute.SharpLimitsFunc) {
	limits := func() (float64, float64) { return 3, 8 }
	mark := func(day, five *float64) (bool, bool) {
		d, f := limits()
		return day != nil && math.Abs(*day) >= d, five != nil && math.Abs(*five) >= f
	}
	return mark, limits
}

// sharpCaller uses only the two types, as a builder does.
func sharpCaller(mark compute.SharpMarkFunc, limits compute.SharpLimitsFunc, day, five *float64) (bool, bool, float64, float64) {
	ds, fs := mark(day, five)
	d, f := limits()
	return ds, fs, d, f
}

func TestSharpContract(t *testing.T) {
	t.Parallel()
	mark, limits := fakeSharp()
	p := func(v float64) *float64 { return &v }
	cases := []struct {
		name      string
		day, five *float64
		wantDay   bool
		wantFive  bool
	}{
		{"below", p(2.99), p(7.99), false, false},
		{"at limit", p(3), p(8), true, true},
		{"negative at limit", p(-3), p(-8), true, true},
		{"independent", p(3), p(1), true, false},
		{"nil is false", nil, nil, false, false},
	}
	for _, c := range cases {
		ds, fs, d, f := sharpCaller(mark, limits, c.day, c.five)
		if ds != c.wantDay || fs != c.wantFive {
			t.Errorf("%s: got %v %v, want %v %v", c.name, ds, fs, c.wantDay, c.wantFive)
		}
		if d != 3 || f != 8 {
			t.Errorf("%s: limits %v %v", c.name, d, f)
		}
	}
}

// fakeEase answers like the contract says: four parts in the order of the doc, a band
// only when the value is present.
type fakeEase struct{}

func (fakeEase) Ease(id string, s compute.EaseSeries, asOf contracts.Day) compute.EaseFigure {
	inputs := []string{"net_liquidity", "us10y", "fed_funds", "hy_oas"}
	parts := make([]compute.EasePart, len(inputs))
	for i, in := range inputs {
		parts[i] = compute.EasePart{Input: in, Status: compute.StatusMissing, Reason: compute.NoInput}
	}
	if id != "macro.ease" {
		return compute.EaseFigure{
			Figure: compute.Figure{ID: id, Day: asOf, Status: compute.StatusMissing, Reason: compute.BadInput},
			Parts:  parts,
		}
	}
	return compute.EaseFigure{
		Figure: compute.Figure{ID: id, Day: asOf, Status: compute.StatusMissing, Reason: compute.NoInput},
		Parts:  parts,
	}
}

// easeCaller uses only the types, as a job does.
func easeCaller(e compute.Ease, id string, asOf contracts.Day) compute.EaseFigure {
	return e.Ease(id, compute.EaseSeries{}, asOf)
}

func TestEaseContract(t *testing.T) {
	t.Parallel()
	newEase := compute.NewEaseFunc(func(compute.Rules) compute.Ease { return fakeEase{} })
	e := newEase(nil)
	for _, id := range []string{"macro.ease", "unknown"} {
		f := easeCaller(e, id, "2026-01-02")
		if len(f.Parts) != 4 {
			t.Fatalf("%s: %d parts, want 4", id, len(f.Parts))
		}
		for i, want := range []string{"net_liquidity", "us10y", "fed_funds", "hy_oas"} {
			if f.Parts[i].Input != want {
				t.Errorf("%s: part %d is %q, want %q", id, i, f.Parts[i].Input, want)
			}
		}
		if f.Status == compute.StatusMissing && f.Band != "" {
			t.Errorf("%s: missing figure has band %q", id, f.Band)
		}
	}
	if got := easeCaller(e, "unknown", "2026-01-02").Reason; got != compute.BadInput {
		t.Errorf("unknown id reason %q, want bad_input", got)
	}
}

func TestEaseWords(t *testing.T) {
	t.Parallel()
	bands := map[compute.EaseBand]string{
		compute.EaseTighter: "tighter",
		compute.EaseSteady:  "steady",
		compute.EaseLooser:  "looser",
	}
	for b, want := range bands {
		if string(b) != want {
			t.Errorf("band %q, want %q", b, want)
		}
	}
	f := easeCaller(fakeEase{}, "macro.ease", "2026-01-02")
	for i, in := range []string{"net_liquidity", "us10y", "fed_funds", "hy_oas"} {
		if f.Parts[i].Input != in {
			t.Errorf("fake part %d %q disagrees with table %q", i, f.Parts[i].Input, in)
		}
	}
}

// fakeCandidates answers like a rule that obeys the contract: it ranks only the ok rows.
type fakeCandidates struct{ sessions *[]contracts.Day }

func (f fakeCandidates) Evaluate(in compute.CandidateInput, _ compute.CandidateParams) []compute.Candidate {
	*f.sessions = in.SessionDays
	gates := func(buy compute.GateStatus) []compute.GateResult {
		g := []compute.GateResult{{Gate: "buy_price", Status: buy}}
		if buy == compute.GateBlock {
			g[0].Reason = "no_buy_price"
		}
		for _, name := range []string{"quote", "events", "funding", "cash_rate"} {
			g = append(g, compute.GateResult{Gate: name, Status: compute.GatePass})
		}
		return g
	}
	return []compute.Candidate{
		{Kind: compute.KindShortPut, Status: "ok", Gates: gates(compute.GatePass), Rank: 1, RankInUnderlying: 1},
		{Kind: compute.KindShortPut, Status: "blocked", Gates: gates(compute.GateBlock)},
	}
}

// candidatesCaller builds the session-day list (ascending, oldest input date to the run
// day) and calls through the interface only.
func candidatesCaller(c compute.OptionCandidates, days []contracts.Day) []compute.Candidate {
	return c.Evaluate(compute.CandidateInput{
		RunDay:      days[len(days)-1],
		ChainDay:    days[len(days)-1],
		SessionDays: days,
		Underlying:  "AAA",
		Spot:        "100.00",
		Rate:        "4.20",
	}, compute.CandidateParams{MinDTE: 21, MaxDTE: 45})
}

func TestOptionCandidatesContract(t *testing.T) {
	t.Parallel()
	var seen []contracts.Day
	var newFn compute.NewOptionCandidatesFunc = func() compute.OptionCandidates {
		return fakeCandidates{sessions: &seen}
	}
	days := []contracts.Day{"2026-01-02", "2026-01-05", "2026-01-06"}
	rows := candidatesCaller(newFn(), days)
	if len(seen) != 3 || seen[0] != "2026-01-02" || seen[2] != "2026-01-06" {
		t.Fatalf("rule saw session days %v, want %v", seen, days)
	}
	order := []string{"buy_price", "quote", "events", "funding", "cash_rate"}
	for i, r := range rows {
		if len(r.Gates) != len(order) {
			t.Fatalf("row %d has %d gates, want %d", i, len(r.Gates), len(order))
		}
		for j, g := range r.Gates {
			if g.Gate != order[j] {
				t.Errorf("row %d gate %d is %q, want %q", i, j, g.Gate, order[j])
			}
		}
		blocked := false
		for _, g := range r.Gates {
			blocked = blocked || g.Status == compute.GateBlock
		}
		if blocked && (r.Status != "blocked" || r.Rank != 0 || r.RankInUnderlying != 0) {
			t.Errorf("row %d is blocked but has status %q rank %d/%d", i, r.Status, r.Rank, r.RankInUnderlying)
		}
	}
	if rows[0].Rank != 1 {
		t.Errorf("ok row rank %d, want 1", rows[0].Rank)
	}
	words := map[string]string{
		string(compute.KindShortPut):      "short_put",
		string(compute.KindCoveredCall):   "covered_call",
		string(compute.KindHeldCallGuard): "held_call_guard",
		string(compute.GatePass):          "pass",
		string(compute.GateBlock):         "block",
		string(compute.GateUnknown):       "unknown",
	}
	for got, want := range words {
		if got != want {
			t.Errorf("word %q, want %q", got, want)
		}
	}
}

var _ compute.OptionLedger = fakeLedger{}

// fakeLedger records what the rule received and returns one position and one cycle.
type fakeLedger struct {
	in *compute.LedgerInput
	p  *compute.LedgerParams
}

func (f fakeLedger) Replay(in compute.LedgerInput, p compute.LedgerParams) compute.LedgerResult {
	*f.in, *f.p = in, p
	return compute.LedgerResult{
		RuleVersion: 1,
		Positions: []compute.LedgerPosition{{
			PositionID: in.Alias + ":" + in.Trades[0].TransactionID + ":1", Alias: in.Alias,
			Status: "open", PremiumIncome: "1.2500", Currency: "USD", MarkStatus: "no_mark",
		}},
		Cycles:     []compute.LedgerCycle{{CycleID: in.Alias + ":AAA:t1", StockRealizedPnL: "0.0000"}},
		Unresolved: []string{"t0"},
	}
}

func TestOptionLedgerContractFake(t *testing.T) {
	t.Parallel()
	var in compute.LedgerInput
	var params compute.LedgerParams
	var newFn compute.NewOptionLedgerFunc = func() compute.OptionLedger { return fakeLedger{in: &in, p: &params} }
	res := newFn().Replay(compute.LedgerInput{
		Alias:       "a",
		Trades:      []compute.LedgerTrade{{TransactionID: "t1", Revision: 2, Attributes: map[string]string{"assetCategory": "OPT"}}},
		Marks:       []compute.LedgerMark{{Contract: "c1", Day: "2026-01-05", Bid: "1.00", Ask: "1.10", UnderlyingClose: "100.00"}},
		SessionDays: []contracts.Day{"2026-01-02", "2026-01-05"},
		AsOf:        "2026-01-05",
	}, compute.LedgerParams{MarkMaxAgeSessions: 5})
	if in.Alias != "a" || in.Trades[0].Revision != 2 || in.SessionDays[1] != "2026-01-05" || params.MarkMaxAgeSessions != 5 {
		t.Fatalf("rule saw %+v %+v", in, params)
	}
	if res.RuleVersion != 1 || res.Positions[0].PositionID != "a:t1:1" || res.Positions[0].PremiumIncome != "1.2500" ||
		res.Cycles[0].CycleID != "a:AAA:t1" || len(res.Unresolved) != 1 || res.Review != nil {
		t.Fatalf("caller read %+v", res)
	}
}

type fakeStructures struct {
	in  *compute.StructureInput
	p   *compute.StructureParams
	out compute.StructureResult
}

func (f fakeStructures) Evaluate(in compute.StructureInput, p compute.StructureParams) compute.StructureResult {
	*f.in, *f.p = in, p
	return f.out
}

var _ compute.OptionStructures = fakeStructures{}

func TestOptionStructuresContract(t *testing.T) {
	t.Parallel()
	want := compute.StructureResult{
		Structures: []compute.Structure{{
			ID: "a:s1", Alias: "a", Underlying: "AAA", Currency: "USD", Kind: "short_put",
			Legs: []compute.StructureLeg{{
				PositionID: "p1", Right: "P", Strike: "50", Expiry: "2026-02-20", Quantity: "-1", Multiplier: "100",
				OpenPrice: "1.20", Mid: "0.90", LegPnL: "30.0000", MarkStatus: "ok",
			}},
			Figures: compute.StructureFigures{
				MaxLoss: compute.FigureValue{Value: "4880.0000"}, MaxGain: compute.FigureValue{Gap: "no_data"},
				BreakEven: compute.FigureValue{Value: "48.8"}, UnrealizedPnL: compute.FigureValue{Value: "30"},
				EarnedShare: compute.FigureValue{Value: "0.25"}, ExpiryDistance: compute.FigureValue{Value: "10"},
				CloseCost: compute.FigureValue{Value: "90"}, RollCredit: compute.FigureValue{Gap: "missing_quote"},
				RollNet: compute.FigureValue{Gap: "missing_input"}, RecentCommission: compute.FigureValue{Value: "1"},
				CashAfter: compute.FigureValue{Gap: "no_rule"},
			},
			Payoff: []compute.PayoffPoint{{Price: "50", PnL: "120.0000"}},
			Greeks: compute.GreekSet{
				Delta: compute.FigureValue{Value: "-0.3"}, Gamma: compute.FigureValue{Value: "0.01"},
				Theta: compute.FigureValue{Value: "-0.02"}, Vega: compute.FigureValue{Gap: "stale_input"}, Day: "2026-01-06",
			},
			Levels: []compute.StructureLevel{{Name: "strike", Price: "50", Distance: "5"}},
			Decide: true, CaptureParam: "0.5", ExitDTEParam: "21",
		}},
		Other: []compute.Structure{{Kind: "other", Currency: "EUR", Figures: compute.StructureFigures{CashAfter: compute.FigureValue{Gap: "unsupported_currency"}}}},
		Diff:  compute.CaptureDiff{CaptureOnly: []string{"x"}, LedgerOnly: []string{"y"}, Skipped: "no_capture"},
	}
	var in compute.StructureInput
	var params compute.StructureParams
	var newFn compute.NewOptionStructuresFunc = func() compute.OptionStructures {
		return fakeStructures{in: &in, p: &params, out: want}
	}
	sent := compute.StructureInput{
		Alias:     "a",
		Positions: []compute.LedgerPosition{{PositionID: "p1"}},
		Cycles:    []compute.LedgerCycle{{CycleID: "c1"}},
		Quotes: []compute.StructureQuote{{
			Underlying: "AAA", Right: "P", Strike: "50", Expiry: "2026-02-20", Day: "2026-01-05",
			Bid: "1", Ask: "1.1", UnderlyingClose: "55", Delta: "-0.3", Gamma: "0.01", Theta: "-0.02", Vega: "0.05",
		}},
		Held:        []compute.HeldOption{{Underlying: "AAA", Right: "P", Strike: "50", Expiry: "2026-02-20", Quantity: "-1"}},
		HeldDay:     "2026-01-05",
		AsOf:        "2026-01-06",
		SessionDays: []contracts.Day{"2026-01-05", "2026-01-06"},
	}
	sentParams := compute.StructureParams{MarkMaxAgeSessions: 5, CaptureTarget: "0.5", ExitDTE: 21, CommissionDays: 30}
	res := newFn().Evaluate(sent, sentParams)
	if !reflect.DeepEqual(in, sent) || params != sentParams {
		t.Fatalf("rule saw %+v %+v", in, params)
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("caller read %+v", res)
	}
}

type fakeResults struct {
	in  *compute.ResultInput
	out compute.ResultSummary
}

func (f fakeResults) Summarise(in compute.ResultInput) compute.ResultSummary {
	*f.in = in
	return f.out
}

var _ compute.OptionResults = fakeResults{}

func TestOptionResultsContract(t *testing.T) {
	t.Parallel()
	want := compute.ResultSummary{Sections: []compute.ResultSection{{
		Side: "sold", Currency: "USD",
		Months:   []compute.ResultMonth{{Month: "2026-01", Realized: "10.0000", Running: "10.0000", Over: true}},
		Endings:  []compute.ResultEnding{{Status: "expired", Count: 2, Realized: "10.0000"}},
		Realized: "10.0000", Premium: "12.0000", KeptShare: "0.833333", AssignedPremium: "1.0000", AssignedShares: "100.0000",
		Reliability: compute.ResultReliability{Equal: 1, Differ: 2, NoBroker: 3, Unresolved: []string{"t0"}, Review: []string{"t9"}},
	}}}
	var in compute.ResultInput
	var newFn compute.NewOptionResultsFunc = func() compute.OptionResults { return fakeResults{in: &in, out: want} }
	sent := compute.ResultInput{
		Alias: "a", Positions: []compute.LedgerPosition{{PositionID: "p1"}},
		Unresolved: []string{"t0"}, Review: []string{"t9"}, AsOf: "2026-01-06",
	}
	res := newFn().Summarise(sent)
	if !reflect.DeepEqual(in, sent) {
		t.Fatalf("rule saw %+v", in)
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("caller read %+v", res)
	}
}

func TestLedgerPositionTerms(t *testing.T) {
	t.Parallel()
	p := compute.LedgerPosition{
		PositionID: "p1", Right: "C", Expiry: "2026-02-20", Strike: "55", OpenedOn: "2026-01-02", ClosedOn: "2026-01-20",
		OpenPrice: "1.20", BrokerRealized: "30.00",
		Fills: []compute.LedgerFill{{Day: "2026-01-02", Action: "open", Quantity: "-1", Price: "1.20", Commission: "1.00"}},
	}
	f := p.Fills[0]
	if p.Right != "C" || p.Expiry != "2026-02-20" || p.Strike != "55" || p.OpenedOn != "2026-01-02" || p.ClosedOn != "2026-01-20" ||
		p.OpenPrice != "1.20" || p.BrokerRealized != "30.00" || f.Day != "2026-01-02" || f.Action != "open" || f.Quantity != "-1" ||
		f.Price != "1.20" || f.Commission != "1.00" {
		t.Fatalf("terms %+v", p)
	}
}

type fakeNotices struct {
	in  *compute.NoticeInput
	par *compute.NoticeParams
	out compute.NoticeResult
}

func (f fakeNotices) Evaluate(i compute.NoticeInput, p compute.NoticeParams) compute.NoticeResult {
	*f.in, *f.par = i, p
	return f.out
}

var _ compute.OptionNotices = fakeNotices{}

func TestOptionNoticesContract(t *testing.T) {
	t.Parallel()
	want := compute.NoticeResult{
		RuleVersion: 1,
		Notices: []compute.Notice{{
			Code: "expiry_near", Severity: "warn", Symbol: "AAPL", Contract: "AAPL 20260220 P 150", Expiry: "2026-02-20",
			Facts: []compute.NoticeFact{{Label: "days", Value: "3"}},
		}},
		CashByDate: []compute.CashDay{{Day: "2026-02-20", Currency: "USD", Amount: "15000.0000"}},
		Cushion:    "0.1200",
	}
	var in compute.NoticeInput
	var par compute.NoticeParams
	var newFn compute.NewOptionNoticesFunc = func() compute.OptionNotices { return fakeNotices{in: &in, par: &par, out: want} }
	next := contracts.Day("2026-02-10")
	pos := compute.NoticePosition{
		Symbol: "AAPL", SecType: "OPT", Expiry: "2026-02-20", Strike: "150", Right: "P",
		Multiplier: "100", Quantity: "-1", Currency: "USD",
	}
	sent := compute.NoticeInput{
		Day: "2026-02-17", Alias: "main", CaptureDay: "2026-02-17",
		Positions: []compute.NoticePosition{pos}, PrevDay: "2026-02-13", PrevPositions: []compute.NoticePosition{pos},
		Guards:   []compute.NoticeGuard{{Symbol: "AAPL", Expiry: "2026-02-20", Strike: "150", Status: "blocked", Reasons: []string{"earnings"}}},
		Earnings: []compute.NoticeEarnings{{Symbol: "AAPL", Next: &next}},
		Values:   []compute.NoticeValue{{Tag: "Cushion", Value: "0.12", Currency: "USD"}},
	}
	sentPar := compute.NoticeParams{ExpiryDays: 5, CushionWarn: "0.15", CushionAct: "0.05"}
	res := newFn().Evaluate(sent, sentPar)
	if !reflect.DeepEqual(in, sent) || par != sentPar {
		t.Fatalf("rule saw %+v %+v", in, par)
	}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("caller read %+v", res)
	}
	var def compute.DefaultNoticeParamsFunc = func() compute.NoticeParams { return sentPar }
	if def() != sentPar {
		t.Fatalf("params %+v", def())
	}
}

type fakeTracker struct {
	in  *compute.TrackInput
	p   *compute.TrackParams
	out compute.TrackResult
}

func (f fakeTracker) Track(in compute.TrackInput, p compute.TrackParams) compute.TrackResult {
	*f.in, *f.p = in, p
	return f.out
}

var _ compute.OptionTracker = fakeTracker{}

func TestOptionTrackerContract(t *testing.T) {
	t.Parallel()
	want := compute.TrackResult{
		Open: []compute.TrackedOption{{
			PositionID: "a:t1:1", Underlying: "AAA", Right: "P", Strike: "50", Expiry: "2026-02-20",
			Quantity: "-1", Multiplier: "100", Currency: "USD", Status: "open", Early: true, DTE: 46,
			Moneyness: "otm", OtmPercent: "4.00", Spot: "52", Mid: "0.90", MarkDay: "2026-01-05", MarkStatus: "ok",
			Extrinsic: "0.90", PremiumIncome: "120", Commissions: "1", Kept: "30", KeptPercent: "25.00",
			RealizedPnL: "29", Delta: "-0.2", Iv: "0.35", NextEarnings: "2026-02-01", NextExDate: "2026-02-10",
			ExAmount: "0.5", EarningsInLife: true, ExInLife: true, EventStatus: "ok",
			Guard: "unknown", GuardReason: "no_guard_row", GuardBasis: "bid", OpenedOn: "2025-12-01", ClosedOn: "2026-01-04",
		}},
		Recent:          []compute.TrackedOption{{PositionID: "a:t0:1", Status: "expire"}},
		Recon:           []compute.TrackRecon{{Underlying: "AAA", Right: "P", Strike: "50", Quantity: "-1", Expiry: "2026-02-20", Code: "not_in_flex"}},
		LongLegsLeftOut: 2,
	}
	var in compute.TrackInput
	var params compute.TrackParams
	var newFn compute.NewOptionTrackerFunc = func() compute.OptionTracker { return fakeTracker{in: &in, p: &params, out: want} }
	got := newFn().Track(compute.TrackInput{
		Alias:       "a",
		AsOf:        "2026-01-05",
		SessionDays: []contracts.Day{"2026-01-02", "2026-01-05"},
		Positions:   []compute.LedgerPosition{{PositionID: "a:t1:1"}},
		Quotes: []compute.TrackQuote{{Underlying: "AAA", Expiry: "2026-02-20", Strike: "50", Right: "P", Day: "2026-01-05",
			Bid: "0.8", Ask: "1.0", UnderlyingClose: "52", Delta: "-0.2", Iv: "0.35"}},
		Events: []compute.TrackEvent{{Underlying: "AAA", Expiry: "2026-02-20", Strike: "50", Right: "C", Kind: "held_call_guard",
			Day: "2026-01-05", NextEarnings: "2026-02-01", EarningsState: "known", NextExDate: "2026-02-10", ExAmount: "0.5",
			GuardStatus: "block", GuardReason: "early_assignment_risk"}},
		Held:      []compute.TrackHeld{{Underlying: "AAA", Expiry: "2026-02-20", Strike: "50", Right: "P", Quantity: "-1"}},
		HeldKnown: true,
	}, compute.TrackParams{MarkMaxAgeSessions: 5, RecentDays: 30})
	if in.Alias != "a" || in.AsOf != "2026-01-05" || in.SessionDays[1] != "2026-01-05" || in.Positions[0].PositionID != "a:t1:1" ||
		in.Quotes[0].Iv != "0.35" || in.Events[0].GuardReason != "early_assignment_risk" || in.Held[0].Quantity != "-1" ||
		!in.HeldKnown || params.MarkMaxAgeSessions != 5 || params.RecentDays != 30 {
		t.Fatalf("rule saw %+v %+v", in, params)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("caller read %+v, want %+v", got, want)
	}
	o := got.Open[0]
	strs := []string{o.PositionID, o.Underlying, o.Right, o.Strike, string(o.Expiry), o.Quantity, o.Multiplier, o.Currency, o.Status,
		o.Moneyness, o.OtmPercent, o.Spot, o.Mid, string(o.MarkDay), o.MarkStatus, o.Extrinsic, o.PremiumIncome, o.Commissions,
		o.Kept, o.KeptPercent, o.RealizedPnL, o.Delta, o.Iv, string(o.NextEarnings), string(o.NextExDate), o.ExAmount,
		o.EventStatus, o.Guard, o.GuardReason, o.GuardBasis, string(o.OpenedOn), string(o.ClosedOn)}
	for i, s := range strs {
		if s == "" {
			t.Errorf("field %d of TrackedOption read empty", i)
		}
	}
	if !o.Early || o.DTE != 46 || !o.EarningsInLife || !o.ExInLife || got.Recon[0].Code != "not_in_flex" || got.LongLegsLeftOut != 2 {
		t.Fatalf("caller read %+v", got)
	}
}
