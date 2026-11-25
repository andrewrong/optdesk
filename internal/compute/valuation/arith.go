// Package valuation implements the valuation rule of compute: the facts of a company
// become ten fiscal years of decimal figures (BuildFilings), read through the parameters
// of its accounting standard (ParamsFor). The types it implements are declared in
// internal/compute/valuationapi. Design: docs/design/valuation.md#Decisions.
//
// Arithmetic is Decimal only. Every division goes through div, which keeps 34
// significant digits explicitly; no package-wide precision setting is used.
package valuation

import (
	"math/big"

	"github.com/shopspring/decimal"
)

// divDigits is the number of significant digits div keeps (the precision of the
// Python code this ports).
const divDigits = 34

// div returns a/b rounded to 34 significant digits, half to even. ok is false when b is
// zero. The quotient is formed exactly on the integer coefficients, so the rounding sees
// every dropped digit.
func div(a, b decimal.Decimal) (q decimal.Decimal, ok bool) {
	if b.IsZero() {
		return decimal.Zero, false
	}
	if a.IsZero() {
		return decimal.Zero, true
	}
	num, den := new(big.Int).Set(a.Coefficient()), new(big.Int).Set(b.Coefficient())
	neg := num.Sign() != den.Sign()
	num.Abs(num)
	den.Abs(den)

	// Scale the numerator so the integer quotient has at least 35 digits.
	scale := divDigits + 1 + digitCount(den) - digitCount(num)
	if scale < 0 {
		scale = 0
	}
	num.Mul(num, pow10(scale))
	quo, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	exp := int64(a.Exponent()) - int64(b.Exponent()) - int64(scale)

	if drop := digitCount(quo) - divDigits; drop > 0 {
		unit := pow10(drop)
		kept, dropped := new(big.Int).QuoRem(quo, unit, new(big.Int))
		twice := new(big.Int).Lsh(dropped, 1)
		switch cmp := twice.Cmp(unit); {
		case cmp > 0, cmp == 0 && rem.Sign() != 0:
			kept.Add(kept, big.NewInt(1))
		case cmp == 0 && kept.Bit(0) == 1:
			kept.Add(kept, big.NewInt(1))
		}
		quo, exp = kept, exp+int64(drop)
	}
	if neg {
		quo.Neg(quo)
	}
	return decimal.NewFromBigInt(quo, int32(exp)), true
}

func digitCount(n *big.Int) int {
	return len(new(big.Int).Abs(n).String())
}

func pow10(n int) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
}
