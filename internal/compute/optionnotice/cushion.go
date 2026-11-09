package optionnotice

import (
	"strings"

	"github.com/andrewrong/optdesk/internal/compute"
)

// cushionNotice reads the stored Cushion tag: at or below CushionAct is act, at or below
// CushionWarn is warn. It returns the stored text as written, or "" when absent or bad.
func cushionNotice(values []compute.NoticeValue, p compute.NoticeParams) (string, []compute.Notice) {
	for _, v := range values {
		if !strings.EqualFold(v.Tag, "Cushion") {
			continue
		}
		c, ok := parseDec(v.Value)
		if !ok {
			continue
		}
		text := strings.TrimSpace(v.Value)
		sev := ""
		if act, ok := parseDec(p.CushionAct); ok && c.Cmp(act) <= 0 {
			sev = sevAct
		} else if warn, ok := parseDec(p.CushionWarn); ok && c.Cmp(warn) <= 0 {
			sev = sevWarn
		}
		if sev == "" {
			return text, nil
		}
		return text, []compute.Notice{{Code: "cushion_low", Severity: sev, Facts: []compute.NoticeFact{fact("cushion", text)}}}
	}
	return "", nil
}

// staleNotice raises capture_stale (info) when the capture is more than one session day
// older than the build day (weekdays; no holiday calendar).
func staleNotice(in compute.NoticeInput, day int, dayOK bool) []compute.Notice {
	capDay, ok := parseDay(string(in.CaptureDay))
	if !ok || !dayOK {
		return nil
	}
	n := sessionDaysBetween(capDay, day)
	if n <= 1 {
		return nil
	}
	return []compute.Notice{{Code: "capture_stale", Severity: sevInfo, Facts: []compute.NoticeFact{
		fact("capture_day", dayText(capDay)), fact("session_days", itoa(n))}}}
}
