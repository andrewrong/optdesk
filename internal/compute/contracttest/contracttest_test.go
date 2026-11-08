package contracttest

import (
	"encoding/json"
	"math"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// fake records failures instead of failing the real test. Run runs the subtest in a
// goroutine of its own and waits for it, so Fatalf and FailNow can stop it with Goexit.
type fake struct {
	testing.TB
	failed bool
	log    *[]string // the name of every subtest run under the root, in order
}

func record(f func(T)) *fake {
	root := &fake{log: new([]string)}
	root.exec(func() { f(root) })
	return root
}

func (f *fake) exec(fn func()) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	<-done
}

func (f *fake) Run(name string, fn func(T)) bool {
	*f.log = append(*f.log, name)
	child := &fake{log: f.log}
	child.exec(func() { fn(child) })
	if child.failed {
		f.failed = true
	}
	return !child.failed
}

func (f *fake) Parallel()             {}
func (f *fake) Helper()               {}
func (f *fake) Name() string          { return "fake" }
func (f *fake) Logf(string, ...any)   {}
func (f *fake) Fail()                 { f.failed = true }
func (f *fake) Failed() bool          { return f.failed }
func (f *fake) Errorf(string, ...any) { f.failed = true }
func (f *fake) Fatalf(string, ...any) { f.FailNow() }
func (f *fake) FailNow()              { f.failed = true; runtime.Goexit() }

// script holds the answers of the table-driven fakes: per method, one answer for each case
// of its group in fixture order. A recording fake runs the subtests in that order, so the
// nth call of a method answers the nth case of its group.
type script struct {
	answers map[string][]any
	next    map[string]int
}

func (s *script) take(method string) any {
	a := s.answers[method][s.next[method]]
	s.next[method]++
	return a
}

// corrupt returns a copy of answer that is off by 1, far more than any tolerance of a fixture.
func corrupt(a any) any {
	switch v := a.(type) {
	case float64:
		return v + 1
	case string:
		return "x" + v
	case []float64:
		v = slices.Clone(v)
		v[len(v)-1]++
		return v
	case compute.Series:
		v = slices.Clone(v)
		v[len(v)-1].Value++
		return v
	case [2]compute.Series:
		v[0] = corrupt(v[0]).(compute.Series)
		return v
	case figureWant:
		v.Value++
		return v
	}
	panic("corrupt: unknown answer type")
}

func mustDecode(t *testing.T, raw json.RawMessage, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

// answer builds the correct answer of a case from its expected value.
func answer(t *testing.T, file string, c Case, zeroWarm bool) any {
	t.Helper()
	switch {
	case file == "figures.json":
		var w figureWant
		mustDecode(t, c.Expected, &w)
		return w
	case file == "units.json" && c.Group == "half_even":
		var s string
		mustDecode(t, c.Expected, &s)
		return s
	case strings.HasPrefix(c.Group, "rolling_"):
		var raw []*float64
		mustDecode(t, c.Expected, &raw)
		out := make([]float64, len(raw))
		for i, p := range raw {
			out[i] = math.NaN()
			if zeroWarm {
				out[i] = 0
			}
			if p != nil {
				out[i] = *p
			}
		}
		return out
	case c.Group == "align":
		var w struct{ A, B []dayPoint }
		mustDecode(t, c.Expected, &w)
		return [2]compute.Series{toSeries(w.A), toSeries(w.B)}
	case file == "series.json":
		var w []dayPoint
		mustDecode(t, c.Expected, &w)
		return toSeries(w)
	}
	var f float64
	mustDecode(t, c.Expected, &f)
	return f
}

// newScript answers every case of the four fixture files; the case named mut, if any, is corrupted.
func newScript(t *testing.T, mut string, zeroWarm bool) *script {
	t.Helper()
	s := &script{answers: map[string][]any{}, next: map[string]int{}}
	for _, name := range []string{"units.json", "stats.json", "series.json", "figures.json"} {
		f, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range f.Cases {
			a := answer(t, name, c, zeroWarm)
			if c.Name == mut {
				a = corrupt(a)
			}
			s.answers[c.Group] = append(s.answers[c.Group], a)
		}
	}
	return s
}

type fakeUnits struct{ s *script }

func (f fakeUnits) PercentToDecimal(float64) float64 { return f.s.take("iv_decimal").(float64) }
func (f fakeUnits) ThetaPerCalendarDay(float64) float64 {
	return f.s.take("theta_per_calendar_day").(float64)
}
func (f fakeUnits) VegaPerVolPoint(float64) float64 { return f.s.take("vega_per_vol_point").(float64) }
func (f fakeUnits) RoundHalfEven(float64, int) (string, error) {
	return f.s.take("half_even").(string), nil
}

type fakeStats struct{ s *script }

func (f fakeStats) Mean([]float64) (float64, error)   { return f.s.take("mean").(float64), nil }
func (f fakeStats) StdDev([]float64) (float64, error) { return f.s.take("std_dev").(float64), nil }
func (f fakeStats) PercentileRank([]float64, float64, compute.Compare) (float64, error) {
	return f.s.take("percentile_rank").(float64), nil
}

type fakeRolling struct{ s *script }

func (f fakeRolling) Mean([]float64, int) ([]float64, error) {
	return f.s.take("rolling_mean").([]float64), nil
}
func (f fakeRolling) StdDev([]float64, int) ([]float64, error) {
	return f.s.take("rolling_std_dev").([]float64), nil
}
func (f fakeRolling) Min([]float64, int) ([]float64, error) {
	return f.s.take("rolling_min").([]float64), nil
}
func (f fakeRolling) Max([]float64, int) ([]float64, error) {
	return f.s.take("rolling_max").([]float64), nil
}
func (f fakeRolling) PercentileRank([]float64, int, compute.Compare) ([]float64, error) {
	return f.s.take("rolling_percentile_rank").([]float64), nil
}

type fakeOps struct{ s *script }

func (f fakeOps) FromSeriesPoints([]contracts.SeriesPoint) compute.Series {
	return f.s.take("from_series_points").(compute.Series)
}
func (f fakeOps) Align(a, b compute.Series) (compute.Series, compute.Series) {
	p := f.s.take("align").([2]compute.Series)
	return p[0], p[1]
}
func (f fakeOps) LogReturns(compute.Series) compute.Series {
	return f.s.take("log_returns").(compute.Series)
}
func (f fakeOps) Values(s compute.Series) []float64 {
	out := make([]float64, len(s))
	for i, p := range s {
		out[i] = p.Value
	}
	return out
}

type fakeFigures struct {
	s     *script
	rules compute.Rules
}

func (f fakeFigures) figure(method, id string, asOf contracts.Day) compute.Figure {
	r, _ := f.rules.Rule(id)
	w := f.s.take(method).(figureWant)
	return compute.Figure{ID: r.ID, Version: r.Version, Day: asOf, Value: w.Value, Status: w.Status, Reason: w.Reason,
		DataDay: contracts.Day(w.DataDay), Observations: w.Observations, StaleRate: w.StaleRate != nil && *w.StaleRate}
}
func (f fakeFigures) HistoricalVol(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.figure("HistoricalVol", id, d)
}
func (f fakeFigures) Iv30(id string, _ []compute.ExpiryQuotes, _ compute.Point, d contracts.Day) compute.Figure {
	return f.figure("Iv30", id, d)
}
func (f fakeFigures) IvPercentile(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.figure("IvPercentile", id, d)
}
func (f fakeFigures) IvRank(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.figure("IvRank", id, d)
}
func (f fakeFigures) MovingAverageGap(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.figure("MovingAverageGap", id, d)
}
func (f fakeFigures) PercentileInYears(id string, _ compute.Series, d contracts.Day) compute.Figure {
	return f.figure("PercentileInYears", id, d)
}
func (f fakeFigures) Ratio(id string, _, _ compute.Series, d contracts.Day) compute.Figure {
	return f.figure("Ratio", id, d)
}

func figuresOf(s *script) compute.NewFiguresFunc {
	return func(_ compute.Stats, _ compute.Rolling, _ compute.SeriesOps, r compute.Rules) compute.Figures {
		return fakeFigures{s, r}
	}
}

// suite is one golden suite with the fixture file and groups it runs.
type suite struct {
	name  string
	file  string
	group func(string) bool
	run   func(T, *script)
}

var suites = []suite{
	{"Units", "units.json", func(string) bool { return true }, func(t T, s *script) { Units(t, fakeUnits{s}) }},
	{"Stats", "stats.json", func(g string) bool { return !strings.HasPrefix(g, "rolling_") }, func(t T, s *script) { Stats(t, fakeStats{s}) }},
	{"Rolling", "stats.json", func(g string) bool { return strings.HasPrefix(g, "rolling_") }, func(t T, s *script) { Rolling(t, fakeRolling{s}) }},
	{"SeriesOps", "series.json", func(string) bool { return true }, func(t T, s *script) { SeriesOps(t, fakeOps{s}) }},
	{"Figures", "figures.json", func(string) bool { return true }, func(t T, s *script) { Figures(t, figuresOf(s), nil, nil, nil) }},
}

// cases lists the names of the cases the suite runs, in fixture order.
func (su suite) cases(t *testing.T) []string {
	t.Helper()
	f, err := Load(su.file)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range f.Cases {
		if su.group(c.Group) {
			names = append(names, c.Name)
		}
	}
	return names
}

func TestSuiteAcceptsCorrectFake(t *testing.T) {
	t.Parallel()
	for _, su := range suites {
		rec := record(func(ft T) { su.run(ft, newScript(t, "", false)) })
		if rec.failed {
			t.Errorf("%s: a correct table-driven fake recorded a failure", su.name)
		}
		if want := su.cases(t); !slices.Equal(*rec.log, want) {
			t.Errorf("%s: subtests %v, want one per case %v", su.name, *rec.log, want)
		}
	}
}

func TestSuiteRejectsMutant(t *testing.T) {
	t.Parallel()
	rejected := 0
	for _, su := range suites {
		t.Run(su.name, func(t *testing.T) {
			mut := su.cases(t)[0]
			rec := record(func(ft T) { su.run(ft, newScript(t, mut, false)) })
			if !rec.failed {
				t.Fatalf("%s accepted a fake that is off by 1 in case %s", su.name, mut)
			}
			rejected++
		})
	}
	if rejected != 5 || len(suites) != 5 {
		t.Fatalf("%d of %d suites rejected their mutant, want 5 of 5", rejected, len(suites))
	}
}

func TestSuiteWarmupNaN(t *testing.T) {
	t.Parallel()
	rec := record(func(ft T) { Rolling(ft, fakeRolling{newScript(t, "", true)}) })
	if !rec.failed {
		t.Fatal("Rolling accepted an implementation that returns 0 for a warm-up entry")
	}
}

func TestFiguresMethodSelector(t *testing.T) {
	t.Parallel()
	f, err := Load("figures.json")
	if err != nil {
		t.Fatal(err)
	}
	var ratio []string
	for _, c := range f.Cases {
		if c.Group == "Ratio" {
			ratio = append(ratio, c.Name)
		}
	}
	rec := record(func(ft T) { Figures(ft, figuresOf(newScript(t, "", false)), nil, nil, nil, "Ratio") })
	if rec.failed || !slices.Equal(*rec.log, ratio) {
		t.Errorf("Ratio: failed %v, ran %v, want only %v", rec.failed, *rec.log, ratio)
	}
	rec = record(func(ft T) { Figures(ft, figuresOf(newScript(t, "", false)), nil, nil, nil, "Ratio", "NoSuchMethod") })
	if !rec.failed || len(*rec.log) != 0 {
		t.Errorf("unknown name: failed %v, ran %v, want a failure before any case", rec.failed, *rec.log)
	}
}

func TestWrapRunsSubtest(t *testing.T) {
	t.Parallel()
	var name string
	ok := Wrap(t).Run("sub", func(s T) { name = s.Name() })
	if !ok || !strings.HasSuffix(name, "/sub") {
		t.Errorf("Run reported %v with subtest name %q, want true and a name ending /sub", ok, name)
	}
}
