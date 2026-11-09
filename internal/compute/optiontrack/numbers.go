package optiontrack

import (
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// dec reads a decimal text exactly; empty or malformed text is no value.
func dec(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "/") {
		return nil, false
	}
	return new(big.Rat).SetString(s)
}

// fixed prints x rounded half-even to the given number of places.
func fixed(x *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(x, new(big.Rat).SetInt(scale))
	num := new(big.Int).Abs(scaled.Num())
	q, r := new(big.Int).QuoRem(num, scaled.Denom(), new(big.Int))
	switch new(big.Int).Lsh(r, 1).Cmp(scaled.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	sign := ""
	if scaled.Sign() < 0 && q.Sign() != 0 {
		sign = "-"
	}
	if places == 0 {
		return sign + q.String()
	}
	whole, frac := new(big.Int).QuoRem(q, scale, new(big.Int))
	f := frac.String()
	return sign + whole.String() + "." + strings.Repeat("0", places-len(f)) + f
}

// dayNumber is the count of calendar days of a YYYY-MM-DD day.
func dayNumber(d contracts.Day) (int, bool) {
	t, err := time.Parse("2006-01-02", string(d))
	if err != nil {
		return 0, false
	}
	return int(t.Unix() / 86400), true
}

// daysBetween is to minus from in calendar days.
func daysBetween(from, to contracts.Day) (int, bool) {
	a, ok1 := dayNumber(from)
	b, ok2 := dayNumber(to)
	return b - a, ok1 && ok2
}

// sameStrike compares two strike texts as numbers.
func sameStrike(a, b string) bool {
	x, ok1 := dec(a)
	y, ok2 := dec(b)
	if !ok1 || !ok2 {
		return a == b
	}
	return x.Cmp(y) == 0
}

// strikeKey is a strike text in one canonical form, for keys and sorting.
func strikeKey(s string) string {
	if x, ok := dec(s); ok {
		return x.RatString()
	}
	return s
}

// cmpStrike orders two strike texts numerically.
func cmpStrike(a, b string) int {
	x, ok1 := dec(a)
	y, ok2 := dec(b)
	if ok1 && ok2 {
		return x.Cmp(y)
	}
	return strings.Compare(a, b)
}

// sideName turns P or C into put or call.
func sideName(right string) string {
	switch strings.ToUpper(right) {
	case "P", "PUT":
		return "put"
	case "C", "CALL":
		return "call"
	}
	return ""
}

// side is the single letter P or C of a right text.
func side(right string) string {
	switch sideName(right) {
	case "put":
		return "P"
	case "call":
		return "C"
	}
	return ""
}

// otmPercent is the signed distance to the strike in percent, positive out of the money.
func otmPercent(isPut bool, strike, spot *big.Rat) (string, bool) {
	if strike.Sign() <= 0 {
		return "", false
	}
	d := new(big.Rat).Sub(spot, strike)
	if !isPut {
		d.Neg(d)
	}
	d.Quo(d, strike).Mul(d, big.NewRat(100, 1))
	return fixed(d, 2), true
}

// moneyness is itm, otm or atm (only when spot equals strike exactly).
func moneyness(isPut bool, strike, spot *big.Rat) string {
	c := spot.Cmp(strike)
	if !isPut {
		c = -c
	}
	switch {
	case c == 0:
		return "atm"
	case c > 0:
		return "otm"
	}
	return "itm"
}

// extrinsic is max(0, mid - intrinsic).
func extrinsic(isPut bool, mid, strike, spot *big.Rat) *big.Rat {
	in := new(big.Rat).Sub(spot, strike)
	if isPut {
		in.Neg(in)
	}
	if in.Sign() < 0 {
		in.SetInt64(0)
	}
	out := new(big.Rat).Sub(mid, in)
	if out.Sign() < 0 {
		out.SetInt64(0)
	}
	return out
}
