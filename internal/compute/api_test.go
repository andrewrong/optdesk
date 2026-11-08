package compute_test

import (
	"context"
	"errors"
	"io"
	"math"
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
