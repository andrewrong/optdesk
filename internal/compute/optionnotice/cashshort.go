package optionnotice

import (
	"math/big"

	"github.com/andrewrong/optdesk/internal/compute"
)

// cashShortNotices raises one cash_short notice per currency for its first row whose Left parses
// and is below zero. Left is cumulative, so one notice stands for the whole shortfall; a row with
// no cash (empty or unparsable Left) raises nothing. rows come in day order, as cashByDate gives them.
func cashShortNotices(rows []compute.CashDay, day int, dayOK bool, p compute.NoticeParams) []compute.Notice {
	type short struct {
		first compute.CashDay
		left  *big.Rat
		days  int
	}
	var order []string
	byCur := map[string]*short{}
	for _, r := range rows {
		left, ok := parseDec(r.Left)
		if !ok || left.Sign() >= 0 {
			continue
		}
		s := byCur[r.Currency]
		if s == nil {
			s = &short{first: r, left: left}
			byCur[r.Currency] = s
			order = append(order, r.Currency)
		}
		s.days++
	}
	var out []compute.Notice
	for _, cur := range order {
		s := byCur[cur]
		avail, ok := parseDec(s.first.Available)
		if !ok {
			continue
		}
		sev := sevWarn
		if d, ok := parseDay(string(s.first.Day)); ok && dayOK && d-day <= p.ExpiryDays {
			sev = sevAct
		}
		out = append(out, compute.Notice{Code: "cash_short", Severity: sev, Expiry: s.first.Day, Facts: []compute.NoticeFact{
			fact("currency", cur),
			fact("available", s.first.Available),
			fact("due", formatTrim(new(big.Rat).Sub(avail, s.left))),
			fact("left", s.first.Left),
			fact("short_days", itoa(s.days)),
			fact("reason", "cash_short_cumulative"),
		}})
	}
	return out
}
