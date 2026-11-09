package optioncand

import (
	"math/big"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const dayLayout = "2006-01-02"

// parseDec reads a decimal text (optional sign, digits, at most one dot) as an exact
// rational; it reports false for an empty or malformed text, so absent never reads as zero.
func parseDec(s string) (*big.Rat, bool) {
	if s == "" {
		return nil, false
	}
	body := s
	if body[0] == '-' || body[0] == '+' {
		body = body[1:]
	}
	if body == "" || strings.Count(body, ".") > 1 || body == "." {
		return nil, false
	}
	for _, c := range body {
		if (c < '0' || c > '9') && c != '.' {
			return nil, false
		}
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return nil, false
	}
	return r, true
}

// formatFixed writes r with exactly places decimals, rounding half to even.
func formatFixed(r *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	abs := new(big.Rat).Abs(r)
	num := new(big.Int).Mul(abs.Num(), scale)
	q, rem := new(big.Int).QuoRem(num, abs.Denom(), new(big.Int))
	twice := new(big.Int).Lsh(rem, 1)
	switch c := twice.Cmp(abs.Denom()); {
	case c > 0, c == 0 && q.Bit(0) == 1:
		q.Add(q, big.NewInt(1))
	}
	digits := q.String()
	if len(digits) <= places {
		digits = strings.Repeat("0", places-len(digits)+1) + digits
	}
	out := digits
	if places > 0 {
		out = digits[:len(digits)-places] + "." + digits[len(digits)-places:]
	}
	if r.Sign() < 0 && q.Sign() != 0 {
		out = "-" + out
	}
	return out
}

// formatTrim writes r to at most 6 places without trailing zeros.
func formatTrim(r *big.Rat) string {
	s := formatFixed(r, 6)
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

func parseDay(d contracts.Day) (time.Time, bool) {
	t, err := time.Parse(dayLayout, string(d))
	return t, err == nil
}

// dayDiff is b minus a in calendar days.
func dayDiff(a, b contracts.Day) (int, bool) {
	ta, ok1 := parseDay(a)
	tb, ok2 := parseDay(b)
	if !ok1 || !ok2 {
		return 0, false
	}
	return int(tb.Sub(ta).Hours() / 24), true
}

// addMonths is d plus n calendar months as a day text.
func addMonths(d contracts.Day, n int) (contracts.Day, bool) {
	t, ok := parseDay(d)
	if !ok {
		return "", false
	}
	return contracts.Day(t.AddDate(0, n, 0).Format(dayLayout)), true
}

// sessionPos is the index of the last session day on or before d, or -1.
func sessionPos(days []contracts.Day, d contracts.Day) int {
	pos := -1
	for i, s := range days {
		if s <= d {
			pos = i
		}
	}
	return pos
}

// sessionAge counts session days from d to to by positions in the supplied list; ok is
// false when d lies before the list, so its age cannot be counted.
func sessionAge(days []contracts.Day, d, to contracts.Day) (int, bool) {
	a, b := sessionPos(days, d), sessionPos(days, to)
	if a < 0 || b < 0 {
		return 0, false
	}
	return b - a, true
}
