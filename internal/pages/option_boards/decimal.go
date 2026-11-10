//nolint:unused // used by the page builders of entries 3 and 4 of docs/design/option-page-builders.md
package option_boards

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"
	minusSign  = "−"
)

var (
	decimalPattern = regexp.MustCompile(`^(-?)([0-9]+)(?:\.([0-9]+))?$`)
	idPattern      = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	weekdayNames   = [...]string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
)

var errBadDecimal = errors.New("option_boards: bad decimal text")

// shiftPercent writes the stored decimal text "0.123456" as "12.3456%": the decimal point moves two places in
// the text, the digits are kept and nothing is rounded. Text that is not a plain decimal ("abc", "1e-3") is
// refused.
func shiftPercent(text string) (string, error) {
	m := decimalPattern.FindStringSubmatch(text)
	if m == nil {
		return "", errBadDecimal
	}
	sign, whole, frac := m[1], m[2], m[3]
	if len(frac) < 2 {
		frac += strings.Repeat("0", 2-len(frac))
	}
	whole = strings.TrimLeft(whole+frac[:2], "0")
	if whole == "" {
		whole = "0"
	}
	out := whole
	if rest := frac[2:]; rest != "" {
		out += "." + rest
	}
	if strings.Trim(out, "0.") == "" {
		sign = ""
	}
	return sign + out + "%", nil
}

// signedText writes a stored decimal text with the typographic minus "−" for the sign.
func signedText(text string) string {
	if rest, ok := strings.CutPrefix(text, "-"); ok {
		return minusSign + rest
	}
	return text
}

// scaledInt is the decimal text times 10^scale as an exact integer; ok is false when the text is not a decimal
// or the product is not an integer ("0.155" at scale 2).
func scaledInt(text string, scale int) (int64, bool) {
	m := decimalPattern.FindStringSubmatch(text)
	if m == nil || scale < 0 || len(m[2]) > 15 {
		return 0, false
	}
	frac := m[3]
	if len(frac) > scale && strings.Trim(frac[scale:], "0") != "" {
		return 0, false
	}
	if len(frac) < scale {
		frac += strings.Repeat("0", scale-len(frac))
	}
	var v int64
	for _, c := range m[2] + frac[:scale] {
		v = v*10 + int64(c-'0')
		if v > 1<<53 {
			return 0, false
		}
	}
	if m[1] == "-" {
		v = -v
	}
	return v, true
}

// compareDecimal orders two stored decimal texts: -1, 0 or 1. A text that is not a decimal is an error.
func compareDecimal(a, b string) (int, error) {
	ma, mb := decimalPattern.FindStringSubmatch(a), decimalPattern.FindStringSubmatch(b)
	if ma == nil || mb == nil {
		return 0, errBadDecimal
	}
	na, nb := ma[1] == "-" && !isZeroDecimal(ma), mb[1] == "-" && !isZeroDecimal(mb)
	switch {
	case na && !nb:
		return -1, nil
	case !na && nb:
		return 1, nil
	}
	c := compareMagnitude(ma, mb)
	if na {
		c = -c
	}
	return c, nil
}

func isZeroDecimal(m []string) bool { return strings.Trim(m[2]+m[3], "0") == "" }

func compareMagnitude(a, b []string) int {
	wa, wb := strings.TrimLeft(a[2], "0"), strings.TrimLeft(b[2], "0")
	if len(wa) != len(wb) {
		if len(wa) < len(wb) {
			return -1
		}
		return 1
	}
	if c := strings.Compare(wa, wb); c != 0 {
		return c
	}
	fa, fb := strings.TrimRight(a[3], "0"), strings.TrimRight(b[3], "0")
	return strings.Compare(fa, fb)
}

// numberLiteral is the stored decimal text as a JSON number, when the text is one.
func numberLiteral(text string) (json.Number, bool) {
	if !decimalPattern.MatchString(text) {
		return "", false
	}
	return json.Number(text), true
}

// parseDay reads a date text.
func parseDay(day contracts.Day) (time.Time, error) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return time.Time{}, fmt.Errorf("option_boards: bad day %q: %w", day, err)
	}
	return t, nil
}

// monthDay is "MM-DD" of a date text.
func monthDay(day contracts.Day) (string, error) {
	t, err := parseDay(day)
	if err != nil {
		return "", err
	}
	return t.Format("01-02"), nil
}

// weekdayText is the Chinese weekday of a date text, "周五" for a Friday; the date text is read as a date, with
// no zone and no calendar.
func weekdayText(day contracts.Day) (string, error) {
	t, err := parseDay(day)
	if err != nil {
		return "", err
	}
	return weekdayNames[t.Weekday()], nil
}

// daysBack is the calendar day n days before day; it is day subtraction on the date text.
func daysBack(day contracts.Day, n int) (contracts.Day, error) {
	t, err := parseDay(day)
	if err != nil {
		return "", err
	}
	return contracts.Day(t.AddDate(0, 0, -n).Format(dayLayout)), nil
}
