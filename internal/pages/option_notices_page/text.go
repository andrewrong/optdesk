package option_notices_page

import (
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/andrewrong/optdesk/internal/compute"
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
	noteHint      = "No guard record: the held-call guard did not store a verdict for this account on the snapshot day, " +
		"so no short call was checked against early assignment or earnings."
)

const (
	dayLayout  = "2006-01-02"
	timeLayout = "2006-01-02T15:04:05-07:00"
)

var (
	decimalText  = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)
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

// amountText is a cash amount by the site money rule: two places, half to even, thousands
// grouped, the symbol before the amount for USD and JPY and the code after it for other
// currencies. A text that is not a plain decimal is returned as stored, the code after it.
func amountText(amount, currency string) string {
	body, ok := moneyBody(amount)
	if !ok {
		body = amount
		if currency != "" {
			body += " " + currency
		}
		return body
	}
	switch currency {
	case "USD":
		return signed(body, "$")
	case "JPY":
		return signed(body, "¥")
	case "":
		return body
	}
	return body + " " + currency
}

func signed(body, sym string) string {
	if rest, neg := strings.CutPrefix(body, "-"); neg {
		return "-" + sym + rest
	}
	return sym + body
}

// moneyBody is the amount with two places and grouped thousands, without symbol.
func moneyBody(amount string) (string, bool) {
	if !decimalText.MatchString(amount) {
		return "", false
	}
	r, ok := new(big.Rat).SetString(amount)
	if !ok {
		return "", false
	}
	v := new(big.Rat).Mul(r, big.NewRat(100, 1))
	q, m := new(big.Int).DivMod(v.Num(), v.Denom(), new(big.Int))
	switch new(big.Int).Lsh(m, 1).Cmp(v.Denom()) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	s := q.String()
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for len(s) < 3 {
		s = "0" + s
	}
	whole, frac := s[:len(s)-2], s[len(s)-2:]
	var b strings.Builder
	if neg && s != "000" {
		b.WriteByte('-')
	}
	for i, c := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return b.String() + "." + frac, true
}

// thresholdText is a threshold ratio as a percent without a needless ".0": "0.15" gives "15%".
func thresholdText(ratio string) string {
	return strings.Replace(percentText(ratio), ".0%", "%", 1)
}

// cushionHintText explains the cushion with the thresholds the rule is run with.
func cushionHintText(p compute.NoticeParams) string {
	return clip("Cushion: the broker's stored Cushion value. Warns at " + thresholdText(p.CushionWarn) +
		" or below, needs action at " + thresholdText(p.CushionAct) + " or below.")
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

// percentText shows a decimal ratio as a percent with one place, half even, in exact
// arithmetic; a text that does not parse as a decimal is returned as stored.
func percentText(ratio string) string {
	r, ok := new(big.Rat).SetString(ratio)
	if !ok || !decimalText.MatchString(ratio) {
		return ratio
	}
	// tenths of a percent = ratio * 1000, rounded half to even.
	v := new(big.Rat).Mul(r, big.NewRat(1000, 1))
	n, d := v.Num(), v.Denom()
	q, m := new(big.Int).DivMod(n, d, new(big.Int))
	switch new(big.Int).Lsh(m, 1).Cmp(d) {
	case 1:
		q.Add(q, big.NewInt(1))
	case 0:
		if q.Bit(0) == 1 {
			q.Add(q, big.NewInt(1))
		}
	}
	s := q.String()
	neg := ""
	if s[0] == '-' {
		neg, s = "-", s[1:]
	}
	for len(s) < 2 {
		s = "0" + s
	}
	return neg + s[:len(s)-1] + "." + s[len(s)-1:] + "%"
}
