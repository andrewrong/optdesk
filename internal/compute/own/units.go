// Package own holds our own glue behind the compute interfaces: unit conversions, series
// handling and rolling windows. It uses the standard library only.
package own

import (
	"math"
	"math/big"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// permanent is the TypedError of a bad input.
type permanent string

func (e permanent) Error() string             { return string(e) }
func (e permanent) Kind() contracts.ErrorKind { return contracts.Permanent }

type units struct{}

// NewUnits is a compute.NewUnitsFunc.
func NewUnits() compute.Units { return units{} }

func (units) PercentToDecimal(p float64) float64 { return p / 100 }

func (units) ThetaPerCalendarDay(perYear float64) float64 { return perYear / 365 }

func (units) VegaPerVolPoint(perUnitVol float64) float64 { return perUnitVol * 0.01 }

// RoundHalfEven rounds the exact binary value of v: a big.Rat of the float, scaled by
// 10^scale, rounded half to even, written as plain decimal text. A result of zero has no sign.
func (units) RoundHalfEven(v float64, scale int) (string, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "", permanent("round: v is not a finite number")
	}
	if scale < 0 {
		return "", permanent("round: negative scale")
	}
	r := new(big.Rat).SetFloat64(v)
	neg := r.Sign() < 0
	r.Abs(r)
	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	r.Mul(r, new(big.Rat).SetInt(pow))
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	switch new(big.Int).Lsh(rem, 1).Cmp(r.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	digits := q.String()
	if len(digits) <= scale {
		digits = strings.Repeat("0", scale+1-len(digits)) + digits
	}
	out := digits
	if scale > 0 {
		out = digits[:len(digits)-scale] + "." + digits[len(digits)-scale:]
	}
	if neg && q.Sign() != 0 {
		out = "-" + out
	}
	return out, nil
}
