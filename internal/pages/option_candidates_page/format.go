package option_candidates_page

import (
	"encoding/json"
	"errors"
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
	yieldPattern  = regexp.MustCompile(`^(-?)([0-9]+)\.([0-9]+)$`)
	numberPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
)

var errBadYield = errors.New("option_candidates_page: bad yield text")

// shiftPoint writes the stored decimal text "0.123456" as "12.3456%": the decimal point moves two
// places in the text, the digits are kept and nothing is rounded.
func shiftPoint(text string) (string, error) {
	m := yieldPattern.FindStringSubmatch(text)
	if m == nil {
		return "", errBadYield
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
	return sign + out + "%", nil
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
