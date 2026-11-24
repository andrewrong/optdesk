package home_page

import "fmt"

// The gap-line and calendar texts, for the external tests of the package.
const GapLineNone = gapLineNone

func GapLineMany(n int) string { return fmt.Sprintf(gapLineMany, n) }
func CalendarGap(m, d, ours, ibkr string) string {
	return fmt.Sprintf(calendarGapFmt, m, d, ours, ibkr)
}
