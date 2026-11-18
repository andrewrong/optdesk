package option_result_page

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Texts of the page. Option terms stay English, other text is Chinese.
const (
	pageTitle   = "Option result"
	nothingText = "No closed option positions"
	noReader    = "No reader: cannot show"
	gapText     = "—"
	dayLayout   = "2006-01-02"
	timeLayout  = "2006-01-02T15:04:05-07:00"
)

var (
	idPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	decPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
)

// number is the decimal text s as a JSON number, written as it is (no float is involved).
func number(s string) (json.Number, error) {
	if !decPattern.MatchString(s) {
		return "", fmt.Errorf("option_result_page: %q is not a decimal", s)
	}
	return json.Number(s), nil
}

// percent is the decimal text s moved two places to the right with a percent sign, rounded half away from zero
// to one decimal, a negative with U+2212 (no sign for a zero result).
func percent(s string) (string, error) {
	if !decPattern.MatchString(s) {
		return "", fmt.Errorf("option_result_page: %q is not a decimal", s)
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return "", fmt.Errorf("option_result_page: %q is not a decimal", s)
	}
	r := d.Mul(decimal.NewFromInt(100)).Round(1)
	out := r.Abs().StringFixed(1) + "%"
	if r.IsNegative() {
		out = "−" + out
	}
	return out, nil
}

// moneyText is the decimal text s as money of currency: "$" or "¥" (another currency its code and a space), the
// integer part grouped by thousands, two decimals rounded half away from zero, a negative with U+2212 before the
// symbol and no sign for a zero result. A text that is not a decimal is returned unchanged.
func moneyText(s, currency string) string {
	if !decPattern.MatchString(s) {
		return s
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return s
	}
	r := d.Round(2)
	whole, frac, _ := strings.Cut(r.Abs().StringFixed(2), ".")
	var b strings.Builder
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	sym := currency + " "
	switch currency {
	case "USD":
		sym = "$"
	case "JPY":
		sym = "¥"
	}
	out := sym + b.String() + "." + frac
	if r.IsNegative() {
		out = "−" + out
	}
	return out
}

// orGap is s, or the gap text when s is "".
func orGap(s string) string {
	if s == "" {
		return gapText
	}
	return s
}

// count is the number of positions that ended with status, as text; no ending of it is 0.
func count(ends []ending, status string) string {
	for _, e := range ends {
		if e.status == status {
			return strconv.Itoa(e.count)
		}
	}
	return "0"
}

// ending is the part of a result ending the page reads.
type ending struct {
	status, realized string
	count            int
}

// stamp is t in zone as the text of generated_at.
func stamp(t time.Time, zone *time.Location) string { return t.In(zone).Format(timeLayout) }

func itoa(n int) string { return strconv.Itoa(n) }
