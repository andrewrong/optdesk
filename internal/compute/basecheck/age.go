package basecheck

import (
	"sort"
	"time"

	"github.com/andrewrong/optdesk/internal/compute/basecheckapi"
	"github.com/andrewrong/optdesk/internal/contracts"
)

// refCalendar is the calendar whose last closed session ends a count of calendar days, and the
// calendar of a row that names none.
const refCalendar = "XNYS"

// age is the age of one subject's newest day: calendar days and sessions of its calendar. Sessions
// is -1 when the subject has no calendar.
type age struct {
	Days, Sessions int
	Subject        string
	Newest         contracts.Day
}

// calendarOf is the calendar id of a subject of a row: Calendars[subject]. A fact with no subject
// of a row with exactly one calendar takes that one. "" means no calendar.
func calendarOf(row basecheckapi.RowInfo, subject string) string {
	if c, ok := row.Calendars[subject]; ok {
		return c
	}
	if subject == "" && len(row.Calendars) == 1 {
		for _, c := range row.Calendars {
			return c
		}
	}
	return ""
}

// firstCalendar is the calendar of a row's first subject: XNYS when the row has none or names it,
// else the smallest id.
func firstCalendar(row basecheckapi.RowInfo) string {
	cals := make([]string, 0, len(row.Calendars))
	for _, c := range row.Calendars {
		cals = append(cals, c)
	}
	if len(cals) == 0 {
		return refCalendar
	}
	sort.Strings(cals)
	return cals[0]
}

// sessionsBetween counts the days of sessions (ascending) after newest and up to last, both
// inclusive of last. A newest day on or after last gives 0, never a negative number.
func sessionsBetween(sessions []contracts.Day, newest, last contracts.Day) int {
	if newest >= last {
		return 0
	}
	n := 0
	for _, s := range sessions {
		if s > newest && s <= last {
			n++
		}
	}
	return n
}

// calendarDays is the number of calendar days from newest to last, 0 when newest is not before it.
func calendarDays(newest, last contracts.Day) (int, bool) {
	a, err := time.Parse("2006-01-02", string(newest))
	if err != nil {
		return 0, false
	}
	b, err := time.Parse("2006-01-02", string(last))
	if err != nil {
		return 0, false
	}
	if !a.Before(b) {
		return 0, true
	}
	return int(b.Sub(a).Hours() / 24), true
}

// subjectAge is the age of the newest day of one subject. ok is false when a calendar's last
// closed session or its session days are missing, or a day is not a date.
func subjectAge(in basecheckapi.Input, row basecheckapi.RowInfo, f basecheckapi.Fact) (age, bool) {
	a := age{Subject: f.Subject, Newest: f.Day, Sessions: -1}
	cal := calendarOf(row, f.Subject)
	ref := cal
	if ref == "" {
		ref = refCalendar
	}
	last, ok := in.Days[ref]
	if !ok {
		return a, false
	}
	days, ok := calendarDays(f.Day, last)
	if !ok {
		return a, false
	}
	a.Days = days
	if cal != "" {
		sessions := in.Sessions[cal]
		if len(sessions) == 0 {
			return a, false
		}
		a.Sessions = sessionsBetween(sessions, f.Day, last)
	}
	return a, true
}

// value is the age in the unit a limit kind reads: sessions of the subject's calendar, else (no
// calendar, or a limit in days) calendar days.
func (a age) value(k Kind) int {
	if k == Sessions && a.Sessions >= 0 {
		return a.Sessions
	}
	return a.Days
}
