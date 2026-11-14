package option_candidates_page

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"
)

var (
	numberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

// roundDigits writes the stored decimal text, its point moved shift places right, with exactly places
// decimals, rounded half away from zero on the digits; ok is false for text that is not a plain decimal.
func roundDigits(text string, shift, places int) (string, bool) {
	if !numberPattern.MatchString(text) {
		return "", false
	}
	neg := strings.HasPrefix(text, "-")
	whole, frac, _ := strings.Cut(strings.TrimPrefix(text, "-"), ".")
	if len(frac) < shift {
		frac += strings.Repeat("0", shift-len(frac))
	}
	whole, frac = whole+frac[:shift], frac[shift:]
	up := len(frac) > places && frac[places] >= '5'
	if len(frac) < places {
		frac += strings.Repeat("0", places-len(frac))
	}
	digits := []byte(whole + frac[:places])
	if up {
		i := len(digits) - 1
		for ; i >= 0 && digits[i] == '9'; i-- {
			digits[i] = '0'
		}
		if i < 0 {
			digits = append([]byte{'1'}, digits...)
		} else {
			digits[i]++
		}
	}
	if len(digits) <= places {
		digits = append([]byte(strings.Repeat("0", places+1-len(digits))), digits...)
	}
	cut := len(digits) - places
	out := string(digits[:cut])
	if places > 0 {
		out += "." + string(digits[cut:])
	}
	out = strings.TrimLeft(out[:cut], "0") + out[cut:]
	if out == "" || out[0] == '.' {
		out = "0" + out
	}
	if neg && strings.Trim(out, "0.") != "" {
		out = "-" + out
	}
	return out, true
}

// fixedText is the stored decimal text with exactly places decimals, rounded half away from zero on
// the text (no float); text that is not a plain decimal gives ok false.
func fixedText(text string, places int) (string, bool) { return roundDigits(text, 0, places) }

// percentText writes the stored fraction "0.118212" as "11.8%".
func percentText(text string) (string, bool) {
	p, ok := roundDigits(text, 2, 1)
	if !ok {
		return "", false
	}
	return p + "%", true
}

// minusText writes a leading ASCII hyphen as the typographic minus U+2212.
func minusText(s string) string {
	if strings.HasPrefix(s, "-") {
		return "\u2212" + s[1:]
	}
	return s
}

// moneyText writes the stored decimal text as "$1,105.75".
func moneyText(text string) (string, bool) {
	f, ok := fixedText(text, 2)
	if !ok {
		return "", false
	}
	sign := ""
	if strings.HasPrefix(f, "-") {
		sign, f = "-", f[1:]
	}
	whole, frac, _ := strings.Cut(f, ".")
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sign + "$" + b.String() + "." + frac, true
}

// numberLiteral is the stored decimal text as a JSON number, when the text is one.
func numberLiteral(text string) (json.Number, bool) {
	if !numberPattern.MatchString(text) {
		return "", false
	}
	return json.Number(text), true
}

// daysBack is the calendar day n days before day; it is day subtraction on the date text.
func daysBack(day contracts.Day, n int) (contracts.Day, error) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return "", err
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format(dayLayout)), nil
}

// midnight is the start of day in zone as an RFC 3339 text.
func midnight(day contracts.Day, zone *time.Location) (string, error) {
	t, err := time.ParseInLocation(dayLayout, string(day), zone)
	if err != nil {
		return "", err
	}
	return t.Format(timeLayout), nil
}
