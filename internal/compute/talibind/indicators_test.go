package talibind

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/contracttest"
	"github.com/andrewrong/optdesk/internal/contracts"
)

var _ compute.NewIndicatorsFunc = New

func TestContract(t *testing.T) {
	t.Parallel()
	contracttest.Indicators(contracttest.Wrap(t), New)
}

type input struct {
	Values []float64 `json:"values"`
	High   []float64 `json:"high"`
	Low    []float64 `json:"low"`
	Close  []float64 `json:"close"`
	Period int       `json:"period"`
	Fast   int       `json:"fast"`
	Slow   int       `json:"slow"`
	Signal int       `json:"signal"`
	UpDev  float64   `json:"up_dev"`
	DnDev  float64   `json:"down_dev"`
	MAType string    `json:"ma_type"`
}

// call runs the function of group on in and names each result array as the fixture does.
func call(t *testing.T, ind compute.Indicators, group string, in input) map[string][]float64 {
	t.Helper()
	var (
		a, b, c []float64
		err     error
	)
	one := func(v []float64, e error) map[string][]float64 {
		a, err = v, e
		return map[string][]float64{"values": a}
	}
	var out map[string][]float64
	switch group {
	case "SMA":
		out = one(ind.SMA(in.Values, in.Period))
	case "EMA":
		out = one(ind.EMA(in.Values, in.Period))
	case "WMA":
		out = one(ind.WMA(in.Values, in.Period))
	case "RSI":
		out = one(ind.RSI(in.Values, in.Period))
	case "ROC":
		out = one(ind.ROC(in.Values, in.Period))
	case "ATR":
		out = one(ind.ATR(in.High, in.Low, in.Close, in.Period))
	case "MACD":
		a, b, c, err = ind.MACD(in.Values, in.Fast, in.Slow, in.Signal)
		out = map[string][]float64{"macd": a, "signal": b, "hist": c}
	case "BBands":
		ma := map[string]compute.MAType{"sma": compute.MASMA, "ema": compute.MAEMA, "wma": compute.MAWMA}[in.MAType]
		a, b, c, err = ind.BBands(in.Values, in.Period, in.UpDev, in.DnDev, ma)
		out = map[string][]float64{"upper": a, "middle": b, "lower": c}
	default:
		t.Fatalf("unknown group %q", group)
	}
	if err != nil {
		t.Fatalf("%s: %v", group, err)
	}
	return out
}

type divergence struct {
	Reason     string   `json:"reason"`
	FirstIndex *int     `json:"first_index"`
	MaxAbsDiff *float64 `json:"max_abs_diff"`
}

// measured is what the raw adapter does to one group over all its cases of the fixture.
type measured struct {
	first int
	max   float64
}

// rawDivergence compares the unwrapped adapter with the fixture, per group; a group whose
// output is within the tolerance of every case is absent.
func rawDivergence(t *testing.T) map[string]measured {
	t.Helper()
	f, err := contracttest.Load("indicators.json")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]measured{}
	for _, c := range f.Cases {
		var in input
		if err := json.Unmarshal(c.Input, &in); err != nil {
			t.Fatal(err)
		}
		var want map[string][]*float64
		if err := json.Unmarshal(c.Expected, &want); err != nil {
			t.Fatal(err)
		}
		got := call(t, raw{}, c.Group, in)
		for name, w := range want {
			g := got[name]
			if len(g) != len(w) {
				t.Fatalf("%s.%s: length %d, want %d", c.Name, name, len(g), len(w))
			}
			for i, p := range w {
				var d float64
				switch {
				case p == nil && math.IsNaN(g[i]):
					continue
				case p == nil || math.IsNaN(g[i]):
					t.Fatalf("%s.%s[%d]: padding differs from the fixture", c.Name, name, i)
				default:
					d = math.Abs(g[i] - *p)
				}
				if d <= c.Tolerance {
					continue
				}
				m, ok := out[c.Group]
				if !ok || i < m.first {
					m.first = i
				}
				m.max = math.Max(m.max, d)
				out[c.Group] = m
			}
		}
	}
	return out
}

func TestDivergenceListIsExact(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/divergence.json")
	if err != nil {
		t.Fatal(err)
	}
	var listed map[string]divergence
	if err := json.Unmarshal(data, &listed); err != nil {
		t.Fatal(err)
	}
	for name, d := range listed {
		if d.Reason == "" || d.FirstIndex == nil || d.MaxAbsDiff == nil {
			t.Errorf("%s: entry lacks reason, first_index or max_abs_diff", name)
		}
	}
	got := rawDivergence(t)
	for name, m := range got {
		d, ok := listed[name]
		if !ok {
			t.Errorf("%s diverges from TA-Lib (first index %d, max diff %g) and is not listed", name, m.first, m.max)
			continue
		}
		if d.FirstIndex != nil && *d.FirstIndex != m.first {
			t.Errorf("%s: first_index %d, measured %d", name, *d.FirstIndex, m.first)
		}
		if d.MaxAbsDiff != nil && math.Abs(*d.MaxAbsDiff-m.max) > 1e-6*math.Max(1, m.max) {
			t.Errorf("%s: max_abs_diff %g, measured %g", name, *d.MaxAbsDiff, m.max)
		}
	}
	for name := range listed {
		if _, ok := got[name]; !ok {
			t.Errorf("%s is listed but the raw library matches the fixture: stale entry", name)
		}
	}
}

func TestWarmupIsNaN(t *testing.T) {
	t.Parallel()
	v := make([]float64, 40)
	for i := range v {
		v[i] = 10 + float64(i%7)
	}
	ind := New()
	checkNaN := func(name string, got []float64, n int) {
		t.Helper()
		if len(got) != len(v) {
			t.Fatalf("%s: length %d, want %d", name, len(got), len(v))
		}
		for i, x := range got {
			if (i < n) != math.IsNaN(x) {
				t.Errorf("%s[%d] = %v, want NaN exactly for the first %d", name, i, x, n)
			}
		}
	}
	for name, f := range map[string]func([]float64, int) ([]float64, error){
		"SMA": ind.SMA, "EMA": ind.EMA, "WMA": ind.WMA,
	} {
		got, err := f(v, 5)
		if err != nil {
			t.Fatal(err)
		}
		checkNaN(name, got, 4)
	}
	for name, f := range map[string]func([]float64, int) ([]float64, error){"RSI": ind.RSI, "ROC": ind.ROC} {
		got, err := f(v, 5)
		if err != nil {
			t.Fatal(err)
		}
		checkNaN(name, got, 5)
	}
	m, s, h, err := ind.MACD(v, 3, 6, 4)
	if err != nil {
		t.Fatal(err)
	}
	checkNaN("macd", m, 8)
	checkNaN("signal", s, 8)
	checkNaN("hist", h, 8)
	u, mid, l, err := ind.BBands(v, 5, 2, 2, compute.MASMA)
	if err != nil {
		t.Fatal(err)
	}
	checkNaN("upper", u, 4)
	checkNaN("middle", mid, 4)
	checkNaN("lower", l, 4)
	a, err := ind.ATR(v, v, v, 5)
	if err != nil {
		t.Fatal(err)
	}
	checkNaN("atr", a, 5)
	short, err := ind.SMA(v[:3], 5)
	if err != nil {
		t.Fatal(err)
	}
	checkNaN2 := func(got []float64) {
		if len(got) != 3 || !math.IsNaN(got[0]) || !math.IsNaN(got[2]) {
			t.Errorf("input shorter than the period: %v, want all NaN", got)
		}
	}
	checkNaN2(short)
}

func TestBadPeriod(t *testing.T) {
	t.Parallel()
	v := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ind := New()
	permanentErr := func(name string, err error) {
		t.Helper()
		var te contracts.TypedError
		if !errors.As(err, &te) || te.Kind() != contracts.Permanent {
			t.Errorf("%s: err = %v, want Permanent TypedError", name, err)
		}
	}
	for name, f := range map[string]func([]float64, int) ([]float64, error){
		"SMA": ind.SMA, "EMA": ind.EMA, "WMA": ind.WMA, "RSI": ind.RSI, "ROC": ind.ROC,
	} {
		for _, p := range []int{0, -1} {
			got, err := f(v, p)
			permanentErr(name, err)
			if got != nil {
				t.Errorf("%s(%d): result %v with an error", name, p, got)
			}
		}
	}
	_, err := ind.RSI(v, 1)
	permanentErr("RSI 1", err)
	_, _, _, err = ind.MACD(v, 0, 5, 3)
	permanentErr("MACD fast 0", err)
	_, _, _, err = ind.MACD(v, 5, 5, 3)
	permanentErr("MACD fast = slow", err)
	_, _, _, err = ind.MACD(v, 3, 5, 0)
	permanentErr("MACD signal 0", err)
	_, _, _, err = ind.BBands(v, 0, 2, 2, compute.MASMA)
	permanentErr("BBands 0", err)
	_, _, _, err = ind.BBands(v, 3, 2, 2, compute.MAType(99))
	permanentErr("BBands maType", err)
	_, err = ind.ATR(v, v, v, 0)
	permanentErr("ATR 0", err)
	_, err = ind.ATR(v, v[:5], v, 3)
	permanentErr("ATR unequal", err)
}

func TestShortInputIsAllNaN(t *testing.T) {
	t.Parallel()
	v := []float64{1, 2, 3}
	ind := New()
	r, err := ind.RSI(v, 14)
	if err != nil || len(r) != 3 || !math.IsNaN(r[0]) || !math.IsNaN(r[2]) {
		t.Errorf("RSI short: %v %v", r, err)
	}
	a, err := ind.ATR(v, v, v, 14)
	if err != nil || len(a) != 3 || !math.IsNaN(a[2]) {
		t.Errorf("ATR short: %v %v", a, err)
	}
	ro, err := ind.ROC(v, 14)
	if err != nil || len(ro) != 3 || !math.IsNaN(ro[2]) {
		t.Errorf("ROC short: %v %v", ro, err)
	}
	m, s, h, err := ind.MACD(v, 12, 26, 9)
	if err != nil || len(m) != 3 || len(s) != 3 || len(h) != 3 || !math.IsNaN(m[2]) || !math.IsNaN(s[2]) || !math.IsNaN(h[2]) {
		t.Errorf("MACD short: %v %v %v %v", m, s, h, err)
	}
}
