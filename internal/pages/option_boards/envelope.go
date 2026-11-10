//nolint:unused // used by the page builders of entries 3 and 4 of docs/design/option-page-builders.md
package option_boards

import (
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

// The run statuses of a page.
const (
	runOK      = "ok"
	runPartial = "partial"
)

// sourceTime is the time of one source of data.
type sourceTime struct {
	Source   string `json:"source"`
	DataTime string `json:"data_time"`
}

// staleLabel says that the data is not of the build day.
type staleLabel struct {
	DataTime   string `json:"data_time"`
	StaleAfter string `json:"stale_after"`
	Text       string `json:"text"`
}

// gap is a value the page could not give, with the reason.
type gap struct {
	Reason string `json:"reason"`
	Text   string `json:"text"`
}

// mkGap is a gap of the reason with the text.
func mkGap(reason, text string) gap { return gap{Reason: reason, Text: text} }

// runStatus is ok, or partial when any gap exists.
func runStatus(gaps []gap) string {
	if len(gaps) > 0 {
		return runPartial
	}
	return runOK
}

// zoneTime writes an instant in the owner zone in the form YYYY-MM-DDTHH:MM:SS+08:00.
func zoneTime(t time.Time, zone *time.Location) string { return t.In(zone).Format(timeLayout) }

// dayStart is 00:00:00 of the day in the zone.
func dayStart(day contracts.Day, zone *time.Location) (string, error) {
	t, err := time.ParseInLocation(dayLayout, string(day), zone)
	if err != nil {
		return "", err
	}
	return t.Format(timeLayout), nil
}

// dayEnd is 23:59:59 of the day in the zone.
func dayEnd(day contracts.Day, zone *time.Location) (string, error) {
	t, err := time.ParseInLocation(dayLayout, string(day), zone)
	if err != nil {
		return "", err
	}
	return t.AddDate(0, 0, 1).Add(-time.Second).Format(timeLayout), nil
}
