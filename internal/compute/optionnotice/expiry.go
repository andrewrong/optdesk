package optionnotice

import "github.com/andrewrong/optdesk/internal/compute"

// expiryNotices raises expiry_today (act) and expiry_near (warn, within limit calendar days)
// for each short option position.
func expiryNotices(cur []held, day, limit int) []compute.Notice {
	var out []compute.Notice
	for _, h := range cur {
		if !h.short() {
			continue
		}
		switch n := h.expDays - day; {
		case n == 0:
			out = append(out, newNotice("expiry_today", sevAct, h, fact("days", "0")))
		case n > 0 && n <= limit:
			out = append(out, newNotice("expiry_near", sevWarn, h, fact("days", itoa(n))))
		}
	}
	return out
}
