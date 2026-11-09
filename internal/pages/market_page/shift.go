package market_page

import (
	"errors"
	"math/big"
	"regexp"
	"strings"
)

var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

var errNotDecimal = errors.New("market_page: not a decimal text")

// shiftRound shifts the decimal text by shift places to the right, rounds it half to even
// to decimals places and writes it as text with exactly decimals places. The arithmetic is
// exact (math/big); a result of zero has no sign. It is the one place a library could replace.
func shiftRound(text string, shift, decimals int) (string, error) {
	if !decimalText.MatchString(text) {
		return "", errNotDecimal
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return "", errNotDecimal
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(shift+decimals)), nil)
	r.Mul(r, new(big.Rat).SetInt(scale))
	neg := r.Sign() < 0
	r.Abs(r)
	q, rem := new(big.Int).QuoRem(r.Num(), r.Denom(), new(big.Int))
	switch rem.Lsh(rem, 1).Cmp(r.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	digits := q.String()
	if decimals > 0 {
		if len(digits) <= decimals {
			digits = strings.Repeat("0", decimals-len(digits)+1) + digits
		}
		digits = digits[:len(digits)-decimals] + "." + digits[len(digits)-decimals:]
	}
	if neg && q.Sign() != 0 {
		digits = "-" + digits
	}
	return digits, nil
}

// inUnitRange reports whether text is a decimal from 0 to 1 inclusive.
func inUnitRange(text string) bool {
	if !decimalText.MatchString(text) {
		return false
	}
	r, ok := new(big.Rat).SetString(text)
	return ok && r.Sign() >= 0 && r.Cmp(big.NewRat(1, 1)) <= 0
}
