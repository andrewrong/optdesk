package option_result_page

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// percent is the decimal text s moved two places to the right with a percent sign; no digit is rounded.
func percent(s string) (string, error) {
	if !decPattern.MatchString(s) {
		return "", fmt.Errorf("option_result_page: %q is not a decimal", s)
	}
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	whole, frac, _ := strings.Cut(s, ".")
	for len(frac) < 2 {
		frac += "0"
	}
	whole = strings.TrimLeft(whole+frac[:2], "0")
	if whole == "" {
		whole = "0"
	}
	out := sign + whole
	if rest := frac[2:]; rest != "" {
		out += "." + rest
	}
	return out + "%", nil
}

// orGap is s, or the gap text when s is empty.
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
