package ease

import (
	"encoding/json"
	"math"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type fakeRules map[string]compute.Rule

func (f fakeRules) Rule(id string) (compute.Rule, bool) { r, ok := f[id]; return r, ok }

func (f fakeRules) IDs() []string {
	var ids []string
	for id := range f {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func newEase() compute.Ease {
	return New(fakeRules{"macro.ease": {
		ID: "macro.ease", Version: 2, Convention: "ease_v2", Unit: "decimal", Scale: 2,
		Window: 126, MinValid: 3, MaxAgeDays: 10,
	}})
}

// wed is a Wednesday.
const wed contracts.Day = "2026-11-04"

func addDays(d contracts.Day, n int) contracts.Day {
	t, err := time.Parse(time.DateOnly, string(d))
	if err != nil {
		panic(err)
	}
	return contracts.Day(t.AddDate(0, 0, n).Format(time.DateOnly))
}

func isWeekend(d contracts.Day) bool {
	t, _ := time.Parse(time.DateOnly, string(d))
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

// weekdayPoints gives n points on the weekdays ending on or before end, oldest first; val
// gets the index from the oldest.
func weekdayPoints(end contracts.Day, n int, val func(i int) float64) compute.Series {
	days := make([]contracts.Day, 0, n)
	for d := end; len(days) < n; d = addDays(d, -1) {
		if !isWeekend(d) {
			days = append(days, d)
		}
	}
	s := make(compute.Series, n)
	for i := range days {
		s[n-1-i] = compute.Point{Day: days[i], Value: val(n - 1 - i)}
	}
	return s
}

func sortSeries(s compute.Series) compute.Series {
	sort.Slice(s, func(i, j int) bool { return s[i].Day < s[j].Day })
	return s
}

// nlSeries builds WALCL, WDTGAL and RRPONTSYD for the Wednesdays w0, w0-7, ...: weeks gives
// the weekly net liquidity of each, newest first, in billions; NaN leaves WDTGAL out of
// that week. RRPONTSYD is on the Tuesday before the Wednesday.
func nlSeries(w0 contracts.Day, weeks []float64, scale float64) (walcl, wdtgal, rrp compute.Series) {
	for k, fig := range weeks {
		w := addDays(w0, -7*k)
		rrp = append(rrp, compute.Point{Day: addDays(w, -1), Value: 20 * scale})
		if math.IsNaN(fig) {
			walcl = append(walcl, compute.Point{Day: w, Value: 100 * scale})
			continue
		}
		walcl = append(walcl, compute.Point{Day: w, Value: (fig + 30) * scale})
		wdtgal = append(wdtgal, compute.Point{Day: w, Value: 10 * scale})
	}
	return sortSeries(walcl), sortSeries(wdtgal), sortSeries(rrp)
}

// weeksUp gives n weeks, newest first: the first four at recent, the rest at 100.
func weeksUp(n int, recent float64) []float64 {
	w := make([]float64, n)
	for k := range w {
		w[k] = 100
		if k < 4 {
			w[k] = recent
		}
	}
	return w
}

// figures are the four figures of the score; nil is absent.
type figures struct{ NL, US, Fed, HY *float64 }

func fp(x float64) *float64 { return &x }

// seriesFor builds series that give the figures on asOf, a Wednesday: the change of net
// liquidity, the us10y and fed_funds changes in basis points and the hy_oas level.
func seriesFor(f figures, asOf contracts.Day) compute.EaseSeries {
	var s compute.EaseSeries
	if f.NL != nil {
		weeks := make([]float64, 17)
		for k := range weeks {
			weeks[k] = 100
			if k < 4 {
				weeks[k] = math.Round(1e11*(1+*f.NL)) / 1e9
			}
		}
		s.WALCL, s.WDTGAL, s.RRPONTSYD = nlSeries(asOf, weeks, 1e9)
	}
	if f.US != nil {
		s.DGS10 = weekdayPoints(asOf, 64, func(i int) float64 {
			if i == 0 {
				return 4
			}
			return 4 + *f.US/100
		})
	}
	if f.Fed != nil {
		s.SP500 = weekdayPoints(asOf, 127, func(int) float64 { return 5000 })
		s.DFF = weekdayPoints(asOf, 127, func(i int) float64 {
			if i == 0 {
				return 1
			}
			return 1 + *f.Fed/100
		})
	}
	if f.HY != nil {
		s.BAMLH0A0HYM2 = compute.Series{{Day: asOf, Value: *f.HY}}
	}
	return s
}

type goldenCase struct {
	Name   string `json:"name"`
	Origin string `json:"origin"`
	Source string `json:"source"`
	Kind   string `json:"kind"`
	Figs   struct {
		NL  *float64 `json:"net_liquidity"`
		US  *float64 `json:"us10y"`
		Fed *float64 `json:"fed_funds"`
		HY  *float64 `json:"hy_oas"`
	} `json:"figures"`
	WantValue   float64    `json:"want_value"`
	WantBand    string     `json:"want_band"`
	WantMissing bool       `json:"want_missing"`
	Weeks       []*float64 `json:"weeks_billions"`
	WantChange  *float64   `json:"want_change"`
	WantReason  string     `json:"want_reason"`
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Cases []goldenCase `json:"cases"`
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	for _, c := range g.Cases {
		if c.Source == "" {
			t.Fatalf("golden case %q has no source line", c.Name)
		}
	}
	return g.Cases
}

func runScoreCase(t *testing.T, c goldenCase) {
	t.Helper()
	got := newEase().Ease("macro.ease", seriesFor(figures{c.Figs.NL, c.Figs.US, c.Figs.Fed, c.Figs.HY}, wed), wed)
	if c.WantMissing {
		if got.Status != compute.StatusMissing || got.Reason != compute.NoInput || got.Band != "" {
			t.Errorf("%s: got %+v, want missing no_input", c.Name, got)
		}
		return
	}
	if got.Status != compute.StatusOK || got.Value != c.WantValue || string(got.Band) != c.WantBand {
		t.Errorf("%s: got value %v band %q status %q, want %v %q", c.Name, got.Value, got.Band, got.Status, c.WantValue, c.WantBand)
	}
}

func TestGoldenPorted(t *testing.T) {
	t.Parallel()
	n := 0
	for _, c := range loadGolden(t) {
		if c.Origin == "option-trader" {
			n++
			runScoreCase(t, c)
		}
	}
	if n != 3 {
		t.Fatalf("%d ported cases, want 3", n)
	}
}

func TestGoldenScoreCases(t *testing.T) {
	t.Parallel()
	for _, c := range loadGolden(t) {
		if c.Origin != "option-trader" && c.Kind == "score" {
			runScoreCase(t, c)
		}
	}
}

func TestThreeInputsMean(t *testing.T) {
	t.Parallel()
	got := newEase().Ease("macro.ease", seriesFor(figures{NL: fp(0.015), US: fp(37.5), HY: fp(4.0)}, wed), wed)
	if got.Status != compute.StatusOK || got.Value != 50 || got.Observations != 3 || got.Band != compute.EaseSteady {
		t.Fatalf("got %+v", got)
	}
	want := []float64{75, 25, 0, 50}
	for i, p := range got.Parts {
		if p.Score != want[i] {
			t.Errorf("part %s score %v, want %v", p.Input, p.Score, want[i])
		}
	}
	if got.Parts[2].Status != compute.StatusMissing || got.Parts[2].Reason != compute.NoInput {
		t.Errorf("fed_funds part %+v", got.Parts[2])
	}
	if got.DataDay != wed {
		t.Errorf("data day %q", got.DataDay)
	}
}

func TestTwoInputsIsMissing(t *testing.T) {
	t.Parallel()
	got := newEase().Ease("macro.ease", seriesFor(figures{US: fp(0), HY: fp(4)}, wed), wed)
	if got.Status != compute.StatusMissing || got.Reason != compute.NoInput || got.Band != "" || got.Value != 0 {
		t.Fatalf("got %+v", got)
	}
	if len(got.Parts) != 4 || got.Observations != 2 {
		t.Fatalf("parts %d, observations %d", len(got.Parts), got.Observations)
	}
}

func TestHalfEvenTie(t *testing.T) {
	t.Parallel()
	got := newEase().Ease("macro.ease", seriesFor(figures{NL: fp(-0.05), US: fp(100), HY: fp(5.49955)}, wed), wed)
	if got.Status != compute.StatusOK || got.Value != 0 {
		t.Fatalf("value %v, want 0.00 (half-even), got %+v", got.Value, got)
	}
	if got.Parts[0].Score != 0 || got.Parts[1].Score != 0 || got.Parts[3].Score != 0.02 {
		t.Errorf("scores %+v", got.Parts)
	}
	if got.Band != compute.EaseTighter {
		t.Errorf("band %q", got.Band)
	}
}

func TestBandEdges(t *testing.T) {
	t.Parallel()
	zero := figures{NL: fp(-0.03), US: fp(75), Fed: fp(50)}
	hundred := figures{NL: fp(0.03), US: fp(-75), Fed: fp(-50)}
	low := figures{NL: fp(0.03), US: fp(75), Fed: fp(50)}   // one at 100, two at 0
	high := figures{NL: fp(0.03), US: fp(-75), Fed: fp(50)} // two at 100, one at 0
	cases := []struct {
		base figures
		hy   float64
		want float64
		band compute.EaseBand
	}{
		{zero, 5.5, 0, compute.EaseTighter},
		{low, 4.4812, 33.49, compute.EaseTighter},
		{low, 4.48, 33.5, compute.EaseSteady},
		{low, 4.42, 34, compute.EaseSteady},
		{high, 3.5212, 66.49, compute.EaseSteady},
		{high, 3.52, 66.5, compute.EaseLooser},
		{high, 3.46, 67, compute.EaseLooser},
		{hundred, 2.5, 100, compute.EaseLooser},
	}
	for _, c := range cases {
		f := c.base
		f.HY = fp(c.hy)
		got := newEase().Ease("macro.ease", seriesFor(f, wed), wed)
		if got.Value != c.want || got.Band != c.band {
			t.Errorf("hy %v: value %v band %q, want %v %q", c.hy, got.Value, got.Band, c.want, c.band)
		}
	}
}

func TestStaleHyOasCountsMissing(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		lag  int // weekdays before wed
		days contracts.Day
		ok   bool
	}{{4, "2026-10-29", false}, {3, "2026-10-30", true}} {
		s := seriesFor(figures{NL: fp(0), US: fp(0), Fed: fp(0)}, wed)
		s.BAMLH0A0HYM2 = compute.Series{{Day: c.days, Value: 4}}
		got := newEase().Ease("macro.ease", s, wed)
		hy := got.Parts[3]
		if c.ok {
			if hy.Status != compute.StatusOK || got.Observations != 4 {
				t.Errorf("%d weekdays: %+v", c.lag, hy)
			}
			continue
		}
		if hy.Status != compute.StatusMissing || hy.Reason != compute.StaleInput || hy.DataDay != c.days {
			t.Errorf("%d weekdays: part %+v", c.lag, hy)
		}
		if got.Status != compute.StatusOK || got.Observations != 3 || got.Value != 50 {
			t.Errorf("%d weekdays: figure %+v", c.lag, got)
		}
	}
}

func TestWeekdaysAcrossWeekend(t *testing.T) {
	t.Parallel()
	const friday contracts.Day = "2026-10-30"
	if got := weekdaysAfter(friday, "2026-11-02"); got != 1 {
		t.Errorf("Friday to Monday: %d, want 1", got)
	}
	if got := weekdaysAfter(friday, "2026-11-04"); got != 3 {
		t.Errorf("Friday to Wednesday: %d, want 3", got)
	}
	if got := weekdaysAfter(friday, friday); got != 0 {
		t.Errorf("same day: %d, want 0", got)
	}
}

func TestFixedPointClip(t *testing.T) {
	t.Parallel()
	beyond := newEase().Ease("macro.ease", seriesFor(figures{NL: fp(-0.2), US: fp(500), Fed: fp(300), HY: fp(9)}, wed), wed)
	for _, p := range beyond.Parts {
		if p.Score != 0 {
			t.Errorf("%s beyond the 0 point scores %v", p.Input, p.Score)
		}
	}
	above := newEase().Ease("macro.ease", seriesFor(figures{NL: fp(0.2), US: fp(-500), Fed: fp(-300), HY: fp(1)}, wed), wed)
	for _, p := range above.Parts {
		if p.Score != 100 {
			t.Errorf("%s beyond the 100 point scores %v", p.Input, p.Score)
		}
	}
	if above.Value != 100 || above.Band != compute.EaseLooser {
		t.Errorf("figure %+v", above)
	}
}

func TestUnknownIDBadInput(t *testing.T) {
	t.Parallel()
	got := newEase().Ease("macro.nope", seriesFor(figures{NL: fp(0), US: fp(0), Fed: fp(0), HY: fp(4)}, wed), wed)
	if got.Status != compute.StatusMissing || got.Reason != compute.BadInput || got.Band != "" || len(got.Parts) != 4 {
		t.Fatalf("got %+v", got)
	}
}

func TestScoreExactNotFloat(t *testing.T) {
	t.Parallel()
	a, ok1 := ratOfFloat(0.1)
	b, ok2 := ratOfFloat(0.2)
	c, ok3 := ratOfFloat(0.3)
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("not a number")
	}
	if a.Add(a, b).Cmp(c) != 0 {
		t.Fatalf("0.1 + 0.2 = %v, want 3/10", a)
	}
	x, y := 0.1, 0.2
	if x+y == 0.3 {
		t.Fatal("the float sum is exact; the test shows nothing")
	}
	if _, ok := ratOfFloat(math.NaN()); ok {
		t.Error("NaN is a number")
	}
}
