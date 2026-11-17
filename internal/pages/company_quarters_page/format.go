// format.go groups the thousands of decimal text and writes the typographic minus. It works on the digits
// as text and never parses a number, so no digit is rounded or lost.
package company_quarters_page

import "strings"

const minusSign = "−"

// group returns s with a comma between every three digits of the integer part and U+2212 for a leading
// "-". The fraction is kept as filed. Text that is not plain decimal text is returned unchanged.
func group(s string) string {
	body, neg := strings.CutPrefix(s, "-")
	whole, frac, hasFrac := strings.Cut(body, ".")
	if !digits(whole) || (hasFrac && !digits(frac)) {
		return s
	}
	var b strings.Builder
	if neg {
		b.WriteString(minusSign)
	}
	lead := len(whole) % 3
	if lead == 0 {
		lead = 3
	}
	for i := 0; i < len(whole); i++ {
		if i > 0 && (i-lead)%3 == 0 && i >= lead {
			b.WriteByte(',')
		}
		b.WriteByte(whole[i])
	}
	if hasFrac {
		b.WriteByte('.')
		b.WriteString(frac)
	}
	return b.String()
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
