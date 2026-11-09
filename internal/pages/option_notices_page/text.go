package option_notices_page

import (
	"regexp"
	"strconv"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// Texts of the page. Option terms stay English, other text is Chinese.
const (
	pageTitle     = "期权提示"
	gapNoCapture  = "近 7 天没有主账户的持仓快照"
	gapNoReader   = "没有读取器,无法显示"
	noteNoCapture = "近 7 天没有这个账户的持仓快照"
	noteNoGuard   = "没有守卫记录"
	noContract    = "—"
	staleHead     = "持仓快照来自 "
	staleTail     = ",不是当日"
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
	"expiry_today":    "今日到期",
	"expiry_near":     "即将到期",
	"call_guard":      "已持 call 被守卫拦截",
	"earnings_inside": "财报日在到期日之前",
	"assignment_seen": "疑似被行权",
	"cushion_low":     "Cushion 偏低",
	"capture_stale":   "持仓快照过旧",
}

// factLabels are the labels of the known facts; a label not listed is shown as it is.
var factLabels = map[string]string{
	"earnings":     "财报日",
	"capture_day":  "快照日",
	"session_days": "相隔交易日",
	"cushion":      "Cushion",
}

// reasonTexts are the sentences of the known guard reason codes.
var reasonTexts = map[string]string{
	"earnings_before_expiry": "财报日在到期日之前",
	"early_assignment_risk":  "除息前有提前被行权的风险",
	"strike_floor":           "行权价低于卖出价下限",
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
