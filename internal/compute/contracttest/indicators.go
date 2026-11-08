package contracttest

import (
	"math"

	"github.com/andrewrong/optdesk/internal/compute"
)

// indIn is the input of every case of indicators.json; a case sets the keys its group needs.
type indIn struct {
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

// Indicators runs the cases of indicators.json (the TA-Lib output of the golden generator)
// against the Indicators that newIndicators returns. A null in the fixture is NaN.
func Indicators(t T, newIndicators compute.NewIndicatorsFunc) {
	t.Helper()
	ind := newIndicators()
	// series compares the named arrays of Expected with the results, in order.
	series := func(names []string, fn func(in indIn) ([][]float64, error)) func(T, Case) {
		return func(t T, c Case) {
			var in indIn
			decode(t, c.Input, &in)
			var raw map[string][]*float64
			decode(t, c.Expected, &raw)
			got, err := fn(in)
			if err != nil {
				t.Errorf("%s: unexpected error %v", c.Name, err)
				return
			}
			if len(got) != len(names) {
				t.Fatalf("%s: %d results, want %d", c.Name, len(got), len(names))
			}
			for i, n := range names {
				want := make([]float64, len(raw[n]))
				for j, p := range raw[n] {
					want[j] = math.NaN()
					if p != nil {
						want[j] = *p
					}
				}
				compareFloats(t, c.Name+"."+n, got[i], want, c.Tolerance)
			}
		}
	}
	one := func(f func(in indIn) ([]float64, error)) func(T, Case) {
		return series([]string{"values"}, func(in indIn) ([][]float64, error) {
			r, err := f(in)
			return [][]float64{r}, err
		})
	}
	maTypes := map[string]compute.MAType{"sma": compute.MASMA, "ema": compute.MAEMA, "wma": compute.MAWMA}
	runCases(t, load(t, "indicators.json"), map[string]func(T, Case){
		"SMA": one(func(in indIn) ([]float64, error) { return ind.SMA(in.Values, in.Period) }),
		"EMA": one(func(in indIn) ([]float64, error) { return ind.EMA(in.Values, in.Period) }),
		"WMA": one(func(in indIn) ([]float64, error) { return ind.WMA(in.Values, in.Period) }),
		"RSI": one(func(in indIn) ([]float64, error) { return ind.RSI(in.Values, in.Period) }),
		"ROC": one(func(in indIn) ([]float64, error) { return ind.ROC(in.Values, in.Period) }),
		"ATR": one(func(in indIn) ([]float64, error) { return ind.ATR(in.High, in.Low, in.Close, in.Period) }),
		"MACD": series([]string{"macd", "signal", "hist"}, func(in indIn) ([][]float64, error) {
			a, b, c, err := ind.MACD(in.Values, in.Fast, in.Slow, in.Signal)
			return [][]float64{a, b, c}, err
		}),
		"BBands": series([]string{"upper", "middle", "lower"}, func(in indIn) ([][]float64, error) {
			a, b, c, err := ind.BBands(in.Values, in.Period, in.UpDev, in.DnDev, maTypes[in.MAType])
			return [][]float64{a, b, c}, err
		}),
	})
}
