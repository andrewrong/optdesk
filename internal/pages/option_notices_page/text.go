package option_notices_page

import (
	"regexp"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Texts of the page. Option terms stay English, other text is Chinese.
const (
	pageTitle     = "Option notices"
	gapNoCapture  = "No position snapshot of the main account in the last 7 days"
	gapNoReader   = "No reader: cannot show"
	noteNoCapture = "No position snapshot of this account in the last 7 days"
	noteNoGuard   = "No guard record"
	noContract    = "—"
	staleHead     = "Position snapshot is from "
	staleTail     = ", not the current day"
	cushionHead   = "Cushion "
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"
)

var (
	idPattern    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	idUnsafe     = regexp.MustCompile(`[^A-Za-z0-9_-]`)
	codeUnsafe   = regexp.MustCompile(`[^a-z_]`)
	maxTextRunes = 200
)

// noticeTitles are the titles of the known codes; a code not listed is shown as it is.
var noticeTitles = map[string]string{
	"expiry_today":    "Expires today",
	"expiry_near":     "Expiring soon",
	"call_guard":      "Held call blocked by the guard",
	"earnings_inside": "Earnings date is before expiry",
	"assignment_seen": "Suspected assignment",
	"cushion_low":     "Cushion is low",
	"capture_stale":   "Position snapshot is too old",
}

// factLabels are the labels of the known facts; a label not listed is shown as it is.
var factLabels = map[string]string{
	"earnings":     "Earnings date",
	"capture_day":  "Snapshot day",
	"session_days": "Trading days apart",
	"cushion":      "Cushion",
}

// reasonTexts are the sentences of the known guard reason codes.
var reasonTexts = map[string]string{
	"earnings_before_expiry": "Earnings date is before expiry",
	"early_assignment_risk":  "Early assignment risk before the ex-dividend date",
	"strike_floor":           "Strike is below the sell price floor",
}

func noticeTitle(code string) string {
	if s, ok := noticeTitles[code]; ok {
		return s
	}
	return code
}

func factLabel(label string) string {
	if s, ok := factLabels[label]; ok {
		return s
	}
	return label
}

func reasonText(code string) string {
	if s, ok := reasonTexts[code]; ok {
		return s
	}
	return code
}

// amountText is the decimal text of an amount followed by its currency.
func amountText(amount, currency string) string {
	if currency == "" {
		return amount
	}
	return amount + " " + currency
}

// clip keeps text within the schema limit and never empty.
func clip(s string) string {
	if s == "" {
		return noContract
	}
	r := []rune(s)
	if len(r) > maxTextRunes {
		return string(r[:maxTextRunes])
	}
	return s
}

// noticeID is notice-<alias>-<n>-<code>, within the schema pattern and length.
func noticeID(alias string, n int, code string) string {
	a := idUnsafe.ReplaceAllString(alias, "_")
	if len(a) > 20 {
		a = a[:20]
	}
	c := codeUnsafe.ReplaceAllString(code, "_")
	if len(c) > 32 {
		c = c[:32]
	}
	return "notice-" + a + "-" + strconv.Itoa(n) + "-" + c
}

// sourceID is the data_times source of an alias.
func sourceID(alias string) string {
	a := idUnsafe.ReplaceAllString(alias, "_")
	if len(a) > 40 {
		a = a[:40]
	}
	return "account_" + a
}

// daysBack is the calendar day n days before day.
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
