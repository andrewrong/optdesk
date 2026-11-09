package optionstruct

import (
	"math/big"
	"strings"
)

// parseDec reads a decimal text (optional sign, digits, at most one dot) as an exact
// rational; it reports false for an empty or malformed text, so absent never reads as zero.
func parseDec(s string) (*big.Rat, bool) {
	body := strings.TrimPrefix(strings.TrimPrefix(s, "-"), "+")
	if body == "" || body == "." || strings.Count(body, ".") > 1 {
		return nil, false
	}
	for _, c := range body {
		if (c < '0' || c > '9') && c != '.' {
			return nil, false
		}
	}
	return new(big.Rat).SetString(s)
}

// fixed4 writes r with exactly 4 decimals, rounding half to even.
func fixed4(r *big.Rat) string {
	const places = 4
	scale := big.NewInt(10000)
	abs := new(big.Rat).Abs(r)
	num := new(big.Int).Mul(abs.Num(), scale)
	q, rem := new(big.Int).QuoRem(num, abs.Denom(), new(big.Int))
	twice := new(big.Int).Lsh(rem, 1)
	if c := twice.Cmp(abs.Denom()); c > 0 || c == 0 && q.Bit(0) == 1 {
		q.Add(q, big.NewInt(1))
	}
	digits := q.String()
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	out := digits[:len(digits)-places] + "." + digits[len(digits)-places:]
	if r.Sign() < 0 && q.Sign() != 0 {
		out = "-" + out
	}
	return out
}

func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
