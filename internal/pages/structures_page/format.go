package structures_page

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

const (
	dayLayout    = "2006-01-02"
	routePrefix  = "/options/structures/"
	maxTextRunes = 200
)

var (
	decimalPattern = regexp.MustCompile(`^(-?)([0-9]+)(?:\.([0-9]+))?$`)
	numberPattern  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

var errBadDecimal = errors.New("structures_page: bad decimal text")

// percentText writes the stored decimal text "0.123456" as "12.3456%": the decimal point moves two
// places in the text, the digits are kept and nothing is rounded.
func percentText(text string) (string, error) {
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
	return sign + out + "%", nil
}

// numberLiteral is the stored decimal text as a JSON number, when the text is one.
func numberLiteral(text string) (json.Number, bool) {
	if !numberPattern.MatchString(text) {
		return "", false
	}
	return json.Number(text), true
}

// shiftDay is the calendar day n days after day (n may be negative); it is day arithmetic on the date text.
func shiftDay(day contracts.Day, n int) (contracts.Day, error) {
	t, err := time.Parse(dayLayout, string(day))
	if err != nil {
		return "", err
	}
	return contracts.Day(t.AddDate(0, 0, n).Format(dayLayout)), nil
}

// midnight is the start of day in zone as an RFC 3339 text.
func midnight(day contracts.Day, zone *time.Location) (string, error) {
	t, err := time.ParseInLocation(dayLayout, string(day), zone)
	if err != nil {
		return "", err
	}
	return t.Format(time.RFC3339), nil
}

// hrefOf is the detail route of a structure id, path-escaped (Decision 5). The carried schema pattern for href
// is ^/structure/[A-Za-z0-9_-]{1,64}$, which no id of the form <alias>:<position> can match; the schema belongs to
// contracts and changes in its own issue, with the web fixtures that use the old route.
func hrefOf(id string) string { return routePrefix + url.PathEscape(id) }

// clip keeps a text within the schema's 200 characters.
func clip(s string) string {
	r := []rune(s)
	if len(r) <= maxTextRunes {
		return s
	}
	return string(r[:maxTextRunes-1]) + "…"
}
