package ruleparam

import (
	"fmt"
	"math/big"
	"strings"
)

// Normalize checks the kind and the range of text for key and returns its
// canonical text: no sign, no exponent, no leading zero, no trailing zero. Blank
// text is valid and returns "", which means the default. Surrounding white
// space is dropped. max_dte is judged here against its own bound only; the
// order of max_dte and min_dte needs both values and is CheckWindow's.
func (registry) Normalize(key, text string) (string, error) {
	s, ok := lookup(key)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownKey, key)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", nil
	}
	canon, ok := canonical(text, s.Kind == kindDecimal)
	if !ok {
		return "", fmt.Errorf("%w: %s must be a %s number", ErrRefused, key, s.Kind)
	}
	v, _ := new(big.Rat).SetString(canon)
	if v.Cmp(rat(s.Min)) < 0 || v.Cmp(rat(s.Max)) > 0 {
		return "", fmt.Errorf("%w: %s must be from %s to %s", ErrRefused, key, s.Min, s.Max)
	}
	return canon, nil
}

// CheckWindow refuses a max_dte below min_dte. stored holds the texts as
// stored or posted; a missing or empty key takes its default.
func CheckWindow(stored map[string]string) error {
	r := registry{}.Resolve(stored)
	lo, ok1 := canonical(strings.TrimSpace(r["min_dte"]), false)
	hi, ok2 := canonical(strings.TrimSpace(r["max_dte"]), false)
	if !ok1 || !ok2 {
		return fmt.Errorf("%w: min_dte and max_dte must be integers", ErrRefused)
	}
	if rat(hi).Cmp(rat(lo)) < 0 {
		return fmt.Errorf("%w: max_dte %s is below min_dte %s", ErrRefused, hi, lo)
	}
	return nil
}

// canonical returns the canonical form of an unsigned decimal literal
// (digits, and with frac one dot between digits), or false.
func canonical(t string, frac bool) (string, bool) {
	whole, fraction, hasDot := strings.Cut(t, ".")
	if hasDot && (!frac || fraction == "") {
		return "", false
	}
	if !digits(whole) || (hasDot && !digits(fraction)) {
		return "", false
	}
	whole = strings.TrimLeft(whole, "0")
	if whole == "" {
		whole = "0"
	}
	fraction = strings.TrimRight(fraction, "0")
	if fraction == "" {
		return whole, true
	}
	return whole + "." + fraction, true
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func rat(s string) *big.Rat {
	r, _ := new(big.Rat).SetString(s)
	return r
}
