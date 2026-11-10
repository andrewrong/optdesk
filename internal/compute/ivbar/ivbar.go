// Package ivbar turns the close of an IBKR implied-volatility bar into the stored text of the
// reading market.iv_ibkr.
package ivbar

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var closeText = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`)

// FromClose takes the close of a bar as plain decimal text (digits with an optional fraction),
// above 0 and not above 10, and gives it rounded half-even to 6 places as decimal text. Any
// other text, a value of 0 or below, or a value above 10 is an error.
func FromClose(close string) (string, error) {
	if !closeText.MatchString(close) {
		return "", fmt.Errorf("ivbar: close %q is not decimal text", close)
	}
	r, ok := new(big.Rat).SetString(close)
	if !ok {
		return "", fmt.Errorf("ivbar: close %q is not decimal text", close)
	}
	if r.Sign() <= 0 {
		return "", fmt.Errorf("ivbar: close %q is not above 0", close)
	}
	if r.Cmp(big.NewRat(10, 1)) > 0 {
		return "", fmt.Errorf("ivbar: close %q is above 10", close)
	}
	const scale = 6
	pow := big.NewInt(1000000)
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
	return digits[:len(digits)-scale] + "." + digits[len(digits)-scale:], nil
}
