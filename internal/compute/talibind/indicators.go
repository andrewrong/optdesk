// Package talibind is compute.Indicators over go-talib. The adapter is a thin one: it
// checks the arguments, calls the library and turns its zero padding into NaN. A function
// whose output differs from TA-Lib is replaced by a corrected one in wrap.go.
package talibind

import (
	"fmt"
	"math"

	"github.com/markcheno/go-talib"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// permanent is the TypedError of a bad argument.
type permanent string

func (e permanent) Error() string             { return "talibind: " + string(e) }
func (e permanent) Kind() contracts.ErrorKind { return contracts.Permanent }

// New is a compute.NewIndicatorsFunc. A period below 1 (below 2 for RSI), a MACD whose
// fast period is not below the slow one, an unknown MAType and series of unequal length are
// Permanent errors. Each result has the input's length, NaN before the first defined value.
func New() compute.Indicators { return wrapped{raw: raw{}} }

// raw is the shortest adapter over go-talib: nothing is corrected.
type raw struct{}

var _ compute.Indicators = raw{}

func period(name string, p, min int) error {
	if p < min {
		return permanent(fmt.Sprintf("%s: period %d below %d", name, p, min))
	}
	return nil
}

// pad returns v with its first n entries set to NaN (all of them when n >= len(v)).
func pad(v []float64, n int) []float64 {
	for i := 0; i < n && i < len(v); i++ {
		v[i] = math.NaN()
	}
	return v
}

func (raw) SMA(values []float64, p int) ([]float64, error) {
	if err := period("SMA", p, 1); err != nil {
		return nil, err
	}
	return pad(talib.Sma(values, p), p-1), nil
}

func (raw) EMA(values []float64, p int) ([]float64, error) {
	if err := period("EMA", p, 1); err != nil {
		return nil, err
	}
	return pad(talib.Ema(values, p), p-1), nil
}

func (raw) WMA(values []float64, p int) ([]float64, error) {
	if err := period("WMA", p, 1); err != nil {
		return nil, err
	}
	return pad(talib.Wma(values, p), p-1), nil
}

func (raw) RSI(values []float64, p int) ([]float64, error) {
	if err := period("RSI", p, 2); err != nil {
		return nil, err
	}
	return pad(talib.Rsi(values, p), p), nil
}

func (raw) ROC(values []float64, p int) ([]float64, error) {
	if err := period("ROC", p, 1); err != nil {
		return nil, err
	}
	return pad(talib.Roc(values, p), p), nil
}

func macdPeriods(fast, slow, signal int) error {
	if err := period("MACD fast", fast, 1); err != nil {
		return err
	}
	if slow <= fast {
		return permanent(fmt.Sprintf("MACD: slow period %d not above fast %d", slow, fast))
	}
	return period("MACD signal", signal, 1)
}

func (raw) MACD(values []float64, fast, slow, signal int) (macd, sig, hist []float64, err error) {
	if err := macdPeriods(fast, slow, signal); err != nil {
		return nil, nil, nil, err
	}
	macd, sig, hist = talib.Macd(values, fast, slow, signal)
	return pad(macd, slow+signal-2), pad(sig, slow+signal-2), pad(hist, slow+signal-2), nil
}

func (raw) BBands(values []float64, p int, upDev, downDev float64, maType compute.MAType) (upper, middle, lower []float64, err error) {
	if err := period("BBands", p, 1); err != nil {
		return nil, nil, nil, err
	}
	var t talib.MaType
	switch maType {
	case compute.MASMA:
		t = talib.SMA
	case compute.MAEMA:
		t = talib.EMA
	case compute.MAWMA:
		t = talib.WMA
	default:
		return nil, nil, nil, permanent(fmt.Sprintf("BBands: unknown MAType %d", maType))
	}
	upper, middle, lower = talib.BBands(values, p, upDev, downDev, t)
	return pad(upper, p-1), pad(middle, p-1), pad(lower, p-1), nil
}

func (raw) ATR(high, low, closes []float64, p int) ([]float64, error) {
	if err := period("ATR", p, 1); err != nil {
		return nil, err
	}
	if len(high) != len(low) || len(low) != len(closes) {
		return nil, permanent("ATR: series of unequal length")
	}
	return pad(talib.Atr(high, low, closes, p), p), nil
}
