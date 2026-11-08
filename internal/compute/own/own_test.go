package own_test

import (
	"math"
	"math/rand"
	"sort"
	"testing"

	"github.com/atombender/go-jsonschema/pkg/types"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/compute/contracttest"
	"github.com/andrewrong/optdesk/internal/compute/own"
	"github.com/andrewrong/optdesk/internal/contracts"
)

type badInput struct{}

func (badInput) Error() string             { return "bad input" }
func (badInput) Kind() contracts.ErrorKind { return contracts.Permanent }

// testStats is a small Stats over the standard library, so no gonum is needed.
type testStats struct{}

func (testStats) Mean(v []float64) (float64, error) {
	if len(v) == 0 {
		return 0, badInput{}
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v)), nil
}

func (s testStats) StdDev(v []float64) (float64, error) {
	if len(v) < 2 {
		return 0, badInput{}
	}
	m, _ := s.Mean(v)
	ss := 0.0
	for _, x := range v {
		ss += (x - m) * (x - m)
	}
	return math.Sqrt(ss / float64(len(v)-1)), nil
}

func (testStats) PercentileRank(v []float64, x float64, cmp compute.Compare) (float64, error) {
	if len(v) == 0 {
		return 0, badInput{}
	}
	n := 0
	for _, y := range v {
		if y < x || (cmp == compute.AtOrBelow && y == x) {
			n++
		}
	}
	return float64(n) / float64(len(v)), nil
}

func TestUnitsContract(t *testing.T) {
	t.Parallel()
	contracttest.Units(contracttest.Wrap(t), own.NewUnits())
}

func TestSeriesOpsContract(t *testing.T) {
	t.Parallel()
	contracttest.SeriesOps(contracttest.Wrap(t), own.NewSeriesOps())
}

func TestRollingContract(t *testing.T) {
	t.Parallel()
	contracttest.Rolling(contracttest.Wrap(t), own.NewRolling(testStats{}))
}

func day(t *testing.T, s string) types.SerializableDate {
	t.Helper()
	var d types.SerializableDate
	if err := d.UnmarshalJSON([]byte(`"` + s + `"`)); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFromSeriesPointsLatestVintage(t *testing.T) {
	t.Parallel()
	f := func(v float64) *float64 { return &v }
	pt := func(obs, start, status string, v *float64) contracts.SeriesPoint {
		return contracts.SeriesPoint{ObservationDate: day(t, obs), RealtimeStart: day(t, start), Status: status, Value: v}
	}
	got := own.NewSeriesOps().FromSeriesPoints([]contracts.SeriesPoint{
		pt("2024-01-03", "2024-01-10", "present", f(3)),
		pt("2024-01-02", "2024-01-05", "present", f(2)),
		pt("2024-01-02", "2024-01-09", "present", f(2.5)), // later vintage wins
		pt("2024-01-03", "2024-01-05", "present", f(9)),   // earlier vintage loses
		pt("2024-01-04", "2024-01-05", "no_value", nil),
		pt("2024-01-05", "2024-01-05", "present", nil), // null value
		pt("2024-01-01", "2024-01-05", "present", f(1)),
	})
	want := compute.Series{{Day: "2024-01-01", Value: 1}, {Day: "2024-01-02", Value: 2.5}, {Day: "2024-01-03", Value: 3}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("point %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestRoundHalfEvenBadInput(t *testing.T) {
	t.Parallel()
	u := own.NewUnits()
	for name, c := range map[string]struct {
		v     float64
		scale int
	}{
		"nan": {math.NaN(), 2}, "inf": {math.Inf(1), 2}, "neg inf": {math.Inf(-1), 2}, "negative scale": {1, -1},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := u.RoundHalfEven(c.v, c.scale)
			te, ok := err.(contracts.TypedError)
			if !ok || te.Kind() != contracts.Permanent {
				t.Errorf("err = %v, want Permanent", err)
			}
		})
	}
}

func TestAlignGap(t *testing.T) {
	t.Parallel()
	a := compute.Series{{Day: "2024-01-01", Value: 1}, {Day: "2024-01-02", Value: 2}, {Day: "2024-01-04", Value: 4}}
	b := compute.Series{{Day: "2024-01-02", Value: 20}, {Day: "2024-01-03", Value: 30}, {Day: "2024-01-04", Value: 40}}
	ga, gb := own.NewSeriesOps().Align(a, b)
	if len(ga) != 2 || len(gb) != 2 || ga[0].Day != "2024-01-02" || ga[1].Value != 4 || gb[0].Value != 20 || gb[1].Day != "2024-01-04" {
		t.Errorf("Align = %v, %v", ga, gb)
	}
}

func TestLogReturnsGap(t *testing.T) {
	t.Parallel()
	s := compute.Series{{Day: "2024-01-01", Value: 1}, {Day: "2024-01-05", Value: math.E}, {Day: "2024-01-06", Value: math.E * math.E}}
	got := own.NewSeriesOps().LogReturns(s)
	if len(got) != 2 || got[0].Day != "2024-01-05" || got[1].Day != "2024-01-06" ||
		math.Abs(got[0].Value-1) > 1e-12 || math.Abs(got[1].Value-1) > 1e-12 {
		t.Errorf("LogReturns = %v", got)
	}
}

func TestRollingMatchesNaive(t *testing.T) {
	t.Parallel()
	rng := rand.New(rand.NewSource(408))
	const w = 20
	values := make([]float64, 200+w-1) // 200 full windows
	for i := range values {
		values[i] = rng.NormFloat64()*10 + 50
	}
	r := own.NewRolling(testStats{})
	mean, err := r.Mean(values, w)
	if err != nil {
		t.Fatal(err)
	}
	sd, err := r.StdDev(values, w)
	if err != nil {
		t.Fatal(err)
	}
	for i := w - 1; i < len(values); i++ {
		win := append([]float64(nil), values[i-w+1:i+1]...)
		sort.Float64s(win) // another summation order than the code under test
		sum := 0.0
		for _, x := range win {
			sum += x
		}
		m := sum / w
		ss := 0.0
		for _, x := range win {
			ss += (x - m) * (x - m)
		}
		wantSD := math.Sqrt(ss / (w - 1))
		if math.Abs(mean[i]-m) > 1e-12 || math.Abs(sd[i]-wantSD) > 1e-12 {
			t.Fatalf("window ending %d: mean %v sd %v, naive %v %v", i, mean[i], sd[i], m, wantSD)
		}
	}
}

func TestRollingWarmup(t *testing.T) {
	t.Parallel()
	r := own.NewRolling(testStats{})
	v := []float64{3, 1, 2, 5, 4}
	for name, fn := range map[string]func([]float64, int) ([]float64, error){"mean": r.Mean, "min": r.Min, "max": r.Max} {
		got, err := fn(v, 3)
		if err != nil || len(got) != len(v) || !math.IsNaN(got[0]) || !math.IsNaN(got[1]) || math.IsNaN(got[2]) {
			t.Errorf("%s = %v, %v", name, got, err)
		}
	}
	if _, err := r.Mean(v, 0); err == nil {
		t.Error("window 0: want an error")
	}
}
